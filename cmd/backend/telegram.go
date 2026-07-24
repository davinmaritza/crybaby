package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"cyrbaby/pkg/protocol"
	"github.com/google/uuid"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramBot struct {
	bot      *tgbotapi.BotAPI
	adminIDs []int64
	db       *DB
	hub      *Hub
}

func InitTelegramBot(token string, adminIDs []int64, db *DB, hub *Hub) (*TelegramBot, error) {
	if token == "" {
		log.Println("Telegram Bot Token is empty. Bot is disabled.")
		return nil, nil
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	log.Printf("Authorized on Telegram bot account: %s", bot.Self.UserName)

	tb := &TelegramBot{
		bot:      bot,
		adminIDs: adminIDs,
		db:       db,
		hub:      hub,
	}

	go tb.startUpdates()

	return tb, nil
}

func (tb *TelegramBot) isAuthorized(userID int64) bool {
	for _, id := range tb.adminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func (tb *TelegramBot) NotifyNewDevice(s *Server) {
	msgText := fmt.Sprintf("🆕 *New device detected*:\n*Hostname*: %s\n*Specs*: %dc/%dt, %.1fGB RAM, %s\n\nApprove with:\n`/approve %s`\n\nReject with:\n`/reject %s`",
		s.Hostname, s.CPUCores, s.CPUThreads, float64(s.RAMTotalMB)/1024, s.OSVersion, s.ID, s.ID)

	for _, adminID := range tb.adminIDs {
		msg := tgbotapi.NewMessage(adminID, msgText)
		msg.ParseMode = "Markdown"
		_, err := tb.bot.Send(msg)
		if err != nil {
			log.Printf("Failed to send Telegram notification to admin %d: %v", adminID, err)
		}
	}
}

func (tb *TelegramBot) startUpdates() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tb.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if !tb.isAuthorized(update.Message.From.ID) {
			log.Printf("Unauthorized Telegram command attempt from user %d (%s %s)",
				update.Message.From.ID, update.Message.From.FirstName, update.Message.From.LastName)
			
			// Silently ignore or send a warning
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Unauthorized access. Attempt logged.")
			tb.bot.Send(msg)
			continue
		}

		go tb.handleMessage(update.Message)
	}
}

func (tb *TelegramBot) handleMessage(m *tgbotapi.Message) {
	text := m.Text
	if text == "" && m.Caption != "" {
		text = m.Caption
	}

	if !strings.HasPrefix(text, "/") {
		// Non-command: delegate to AI
		reply, err := AskAI(text, tb.db, tb.hub)
		if err != nil {
			tb.reply(m, fmt.Sprintf("AI Error: %v", err))
		} else {
			tb.reply(m, reply)
		}
		return
	}

	parts := strings.Fields(text)
	cmd := parts[0]
	if idx := strings.Index(cmd, "@"); idx != -1 {
		cmd = cmd[:idx]
	}
	args := parts[1:]

	switch cmd {
	case "/help", "/start":
		tb.cmdHelp(m, args)
	case "/status":
		tb.cmdStatus(m, args)
	case "/list":
		tb.cmdList(m, args)
	case "/info":
		tb.cmdInfo(m, args)
	case "/pending":
		tb.cmdPending(m, args)
	case "/approve":
		tb.cmdApprove(m, args)
	case "/reject":
		tb.cmdReject(m, args)
	case "/rename":
		tb.cmdRename(m, args)
	case "/remove":
		tb.cmdRemove(m, args)
	case "/cluster":
		tb.cmdCluster(m, args)
	case "/exec":
		tb.cmdExec(m, args)
	case "/broadcast":
		tb.cmdBroadcast(m, args)
	case "/file":
		tb.cmdFile(m, args)
	case "/relocate", "/setbackend":
		tb.cmdRelocate(m, args)
	default:
		tb.reply(m, "Unknown command. Use /help to see all available commands.")
	}
}


func (tb *TelegramBot) cmdHelp(m *tgbotapi.Message, args []string) {
	helpText := `🤖 CryBaby RMM - by Suzirz

📊 Dashboard:
/status - Fleet resource summary (CPU/RAM)
/list - List all active servers
/pending - Servers waiting approval

🔐 Auth:
/approve UUID - Approve a server
/reject UUID - Reject a server
/remove UUID - Remove a server

🖥 Management:
/info UUID - Detailed info for a server
/rename UUID NAME - Set custom name
/exec UUID COMMAND - Run shell command
/broadcast COMMAND - Run on ALL servers
/file UUID PATH - Download file from server

💬 AI Agent:
Send any message without / to chat with AI!

🔄 Backend Relocation:
/relocate ws://IP:PORT/ws - Update backend URL on ALL online agents`

	msg := tgbotapi.NewMessage(m.Chat.ID, helpText)
	tb.bot.Send(msg)
}

func (tb *TelegramBot) cmdRelocate(m *tgbotapi.Message, args []string) {
	if len(args) == 0 {
		tb.reply(m, "Usage: /relocate ws://IP_BARU:PORT_BARU/ws (or http://...)")
		return
	}

	newURL := args[0]
	if !strings.HasPrefix(newURL, "ws://") && !strings.HasPrefix(newURL, "wss://") {
		if strings.HasPrefix(newURL, "http://") {
			newURL = "ws://" + strings.TrimPrefix(newURL, "http://")
		} else if strings.HasPrefix(newURL, "https://") {
			newURL = "wss://" + strings.TrimPrefix(newURL, "https://")
		} else {
			newURL = "ws://" + newURL
		}
	}
	if !strings.HasSuffix(newURL, "/ws") {
		newURL = strings.TrimRight(newURL, "/") + "/ws"
	}

	count := tb.hub.BroadcastUpdateConfig(newURL)
	tb.reply(m, fmt.Sprintf("✅ Sinyal relocation dikirim ke %d agent online!\nURL Backend Baru: %s", count, newURL))
}



func (tb *TelegramBot) reply(m *tgbotapi.Message, text string) {
	// Telegram messages have a length limit of 4096 characters.
	if len(text) > 4000 {
		text = text[:3900] + "\n...[Output Truncated]..."
	}
	msg := tgbotapi.NewMessage(m.Chat.ID, text)
	msg.ReplyToMessageID = m.MessageID
	_, err := tb.bot.Send(msg)
	if err != nil {
		log.Printf("Failed to reply: %v", err)
	}
}

func (tb *TelegramBot) cmdStatus(m *tgbotapi.Message, args []string) {
	servers, err := tb.db.GetServers()
	if err != nil {
		tb.reply(m, fmt.Sprintf("Error: %v", err))
		return
	}

	var filterCluster string
	if len(args) > 0 {
		filterCluster = strings.ToLower(args[0])
	}

	var (
		total, online, offline, pending int
		cores, threads                  int
		totalRAM, totalDisk             uint64
	)

	for _, s := range servers {
		if s.Status == "pending_approval" {
			pending++
			continue
		}

		if filterCluster != "" {
			if s.ClusterName == nil || strings.ToLower(*s.ClusterName) != filterCluster {
				continue
			}
		}

		total++
		cores += s.CPUCores
		threads += s.CPUThreads
		totalRAM += s.RAMTotalMB
		totalDisk += s.DiskTotalMB

		client, connected := tb.hub.clients[s.ID]
		if connected && client.approved && time.Since(s.LastSeenAt) < 20*time.Second {
			online++
		} else {
			offline++
		}
	}

	title := "Fleet Summary"
	if filterCluster != "" {
		title = fmt.Sprintf("Sector [%s] Summary", filterCluster)
	}

	response := fmt.Sprintf("📊 *%s*:\n"+
		"• Total Registered: %d\n"+
		"• Online: %d\n"+
		"• Offline: %d\n"+
		"• Pending Approval: %d\n"+
		"• Total CPU: %dc / %dt\n"+
		"• Total Memory: %.1f GB\n"+
		"• Total C: Drive: %.1f GB",
		title, total, online, offline, pending, cores, threads, float64(totalRAM)/1024, float64(totalDisk)/1024)

	msg := tgbotapi.NewMessage(m.Chat.ID, response)
	msg.ParseMode = "Markdown"
	tb.bot.Send(msg)
}

func (tb *TelegramBot) cmdList(m *tgbotapi.Message, args []string) {
	servers, err := tb.db.GetServers()
	if err != nil {
		tb.reply(m, fmt.Sprintf("Error: %v", err))
		return
	}

	var filterCluster string
	if len(args) > 0 {
		filterCluster = strings.ToLower(args[0])
	}

	var lines []string
	for _, s := range servers {
		if filterCluster != "" {
			if s.ClusterName == nil || strings.ToLower(*s.ClusterName) != filterCluster {
				continue
			}
		}

		client, connected := tb.hub.clients[s.ID]
		isOnline := connected && client.approved && (s.LastSeenAt.IsZero() || time.Since(s.LastSeenAt) < 60*time.Second)

		statusEmoji := "🔴"
		if s.Status == "pending_approval" {
			statusEmoji = "⏳ [Pending /approve]"
		} else if isOnline {
			statusEmoji = "🟢"
		}

		name := s.Hostname
		if s.CustomName != nil && *s.CustomName != "" {
			name = fmt.Sprintf("%s (%s)", *s.CustomName, s.Hostname)
		}

		clusterName := "Unassigned"
		if s.ClusterName != nil {
			clusterName = *s.ClusterName
		}

		idShort := s.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		lines = append(lines, fmt.Sprintf("%s `%s` | Name: %s | Sector: %s", statusEmoji, idShort, name, clusterName))
	}


	if len(lines) == 0 {
		tb.reply(m, "No servers match criteria.")
		return
	}

	tb.reply(m, strings.Join(lines, "\n"))
}

func (tb *TelegramBot) cmdInfo(m *tgbotapi.Message, args []string) {
	if len(args) < 1 {
		tb.reply(m, "Usage: /info <server_id_prefix>")
		return
	}

	idPrefix := args[0]
	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	samples, _ := tb.db.GetRecentMetricSamples(s.ID, 1)
	var load float64
	var ramUsedMB uint64
	if len(samples) > 0 {
		load = samples[0].CPULoadPct
		ramUsedMB = samples[0].RAMUsedMB
	}

	client, connected := tb.hub.clients[s.ID]
	isOnline := connected && client.approved && (s.LastSeenAt.IsZero() || time.Since(s.LastSeenAt) < 60*time.Second)

	status := "🔴 Offline"
	if isOnline {
		status = fmt.Sprintf("🟢 Online (CPU: %.0f%%, RAM: %.1f/%.1fGB)", load, float64(ramUsedMB)/1024, float64(s.RAMTotalMB)/1024)
	}

	clusterName := "Unassigned"
	if s.ClusterName != nil {
		clusterName = *s.ClusterName
	}

	customName := "—"
	if s.CustomName != nil {
		customName = *s.CustomName
	}

	info := fmt.Sprintf("🖥️ *Server Spec details*:\n"+
		"• *ID*: `%s`\n"+
		"• *Hostname*: %s\n"+
		"• *Custom Label*: %s\n"+
		"• *Sector*: %s\n"+
		"• *Status*: %s\n"+
		"• *OS*: %s\n"+
		"• *CPU*: %s (%dc/%dt)\n"+
		"• *RAM*: %.1f GB\n"+
		"• *C: Drive*: %.1f GB\n"+
		"• *Agent Version*: %s\n"+
		"• *Last Heartbeat*: %s",
		s.ID, s.Hostname, customName, clusterName, status, s.OSVersion, s.CPUModel, s.CPUCores, s.CPUThreads,
		float64(s.RAMTotalMB)/1024, float64(s.DiskTotalMB)/1024, s.AgentVersion, s.LastSeenAt.Format("2006-01-02 15:04:05"))

	msg := tgbotapi.NewMessage(m.Chat.ID, info)
	msg.ParseMode = "Markdown"
	tb.bot.Send(msg)
}

func (tb *TelegramBot) cmdPending(m *tgbotapi.Message, args []string) {
	servers, err := tb.db.GetServers()
	if err != nil {
		tb.reply(m, fmt.Sprintf("Error: %v", err))
		return
	}

	var lines []string
	for _, s := range servers {
		if s.Status == "pending_approval" {
			lines = append(lines, fmt.Sprintf("🆕 ID: `%s` | Host: %s | CPU: %dc/%dt | RAM: %.1fGB",
				s.ID, s.Hostname, s.CPUCores, s.CPUThreads, float64(s.RAMTotalMB)/1024))
		}
	}

	if len(lines) == 0 {
		tb.reply(m, "No pending device authentications.")
		return
	}

	tb.reply(m, strings.Join(lines, "\n"))
}

func (tb *TelegramBot) cmdApprove(m *tgbotapi.Message, args []string) {
	if len(args) < 1 {
		tb.reply(m, "Usage: /approve <server_id_prefix>")
		return
	}

	idPrefix := args[0]
	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	if s.Status == "approved" {
		tb.reply(m, "Server is already approved.")
		return
	}

	if err := tb.hub.ApprovePendingAgent(s.ID); err != nil {
		tb.reply(m, fmt.Sprintf("Approval failed: %v", err))
	} else {
		tb.reply(m, fmt.Sprintf("✅ Approved device %s. Cryptographic token pushed successfully.", s.Hostname))
	}
}

func (tb *TelegramBot) cmdReject(m *tgbotapi.Message, args []string) {
	if len(args) < 1 {
		tb.reply(m, "Usage: /reject <server_id_prefix>")
		return
	}

	idPrefix := args[0]
	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	if err := tb.hub.RejectPendingAgent(s.ID); err != nil {
		tb.reply(m, fmt.Sprintf("Rejection failed: %v", err))
	} else {
		tb.reply(m, fmt.Sprintf("❌ Rejected device %s. Uninstall instruction pushed.", s.Hostname))
	}
}

func (tb *TelegramBot) cmdRename(m *tgbotapi.Message, args []string) {
	if len(args) < 2 {
		tb.reply(m, "Usage: /rename <server_id_prefix> <new name>")
		return
	}

	idPrefix := args[0]
	name := strings.Join(args[1:], " ")

	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	var namePtr *string
	if name != "" && name != "-" {
		namePtr = &name
	}

	if err := tb.db.UpdateServerCustomName(s.ID, namePtr); err != nil {
		tb.reply(m, fmt.Sprintf("Error: %v", err))
	} else {
		tb.reply(m, fmt.Sprintf("Label updated for %s.", s.Hostname))
	}
}

func (tb *TelegramBot) cmdRemove(m *tgbotapi.Message, args []string) {
	if len(args) < 1 {
		tb.reply(m, "Usage: /remove <server_id_prefix> [confirm]")
		return
	}

	idPrefix := args[0]
	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	if len(args) < 2 || args[1] != "confirm" {
		tb.reply(m, fmt.Sprintf("⚠️ WARNING: Decommissioning server %s will uninstall agent.\nTo proceed, run: `/remove %s confirm`", s.Hostname, idPrefix))
		return
	}

	if err := tb.hub.DecommissionAgent(s.ID); err != nil {
		tb.reply(m, fmt.Sprintf("Decommission failed: %v", err))
	} else {
		tb.reply(m, fmt.Sprintf("💀 Decommissioned device %s. Uninstall instruction dispatched.", s.Hostname))
	}
}

func (tb *TelegramBot) cmdCluster(m *tgbotapi.Message, args []string) {
	if len(args) < 1 {
		tb.reply(m, "Usage: /cluster list, /cluster new <name>, /cluster set <server_id_prefix> <cluster_name>")
		return
	}

	action := args[0]

	switch action {
	case "list":
		clusters, err := tb.db.GetClusters()
		if err != nil {
			tb.reply(m, fmt.Sprintf("Error: %v", err))
			return
		}
		var lines []string
		for _, c := range clusters {
			lines = append(lines, fmt.Sprintf("• Sector: %s | Count: %d | Info: %s", c.Name, c.ServerCount, c.Description))
		}
		if len(lines) == 0 {
			tb.reply(m, "No sectors (clusters) defined.")
		} else {
			tb.reply(m, strings.Join(lines, "\n"))
		}

	case "new":
		if len(args) < 2 {
			tb.reply(m, "Usage: /cluster new <name> [description]")
			return
		}
		name := args[1]
		desc := ""
		if len(args) > 2 {
			desc = strings.Join(args[2:], " ")
		}

		id := uuid.New().String()
		if err := tb.db.CreateCluster(id, name, desc); err != nil {
			tb.reply(m, fmt.Sprintf("Creation failed: %v", err))
		} else {
			tb.reply(m, fmt.Sprintf("Sector [%s] created.", name))
		}

	case "set":
		if len(args) < 3 {
			tb.reply(m, "Usage: /cluster set <server_id_prefix> <cluster_name>")
			return
		}
		idPrefix := args[1]
		cName := args[2]

		s := tb.findServerByPrefix(idPrefix)
		if s == nil {
			tb.reply(m, "Server not found.")
			return
		}

		clusters, err := tb.db.GetClusters()
		if err != nil {
			tb.reply(m, fmt.Sprintf("Error: %v", err))
			return
		}

		var targetClusterID *string
		for _, c := range clusters {
			if strings.ToLower(c.Name) == strings.ToLower(cName) {
				cID := c.ID
				targetClusterID = &cID
				break
			}
		}

		if targetClusterID == nil && strings.ToLower(cName) != "unassigned" {
			tb.reply(m, fmt.Sprintf("Sector [%s] does not exist. Create it first with /cluster new %s", cName, cName))
			return
		}

		if err := tb.db.SetServerCluster(s.ID, targetClusterID); err != nil {
			tb.reply(m, fmt.Sprintf("Assignment failed: %v", err))
		} else {
			tb.reply(m, fmt.Sprintf("Assigned server %s to sector %s.", s.Hostname, cName))
		}
	}
}

func (tb *TelegramBot) cmdExec(m *tgbotapi.Message, args []string) {
	if len(args) < 2 {
		tb.reply(m, "Usage: /exec <server_id_prefix> <powershell command>")
		return
	}

	idPrefix := args[0]
	command := strings.Join(args[1:], " ")

	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	tb.reply(m, fmt.Sprintf("Running command on %s...", s.Hostname))
	resp, err := tb.hub.SendCommand(s.ID, command, fmt.Sprintf("telegram_user_%d", m.From.ID))
	if err != nil {
		tb.reply(m, fmt.Sprintf("Execution error: %v", err))
		return
	}

	out := resp.Output
	if resp.Error != "" {
		out += "\nSystem Error: " + resp.Error
	}
	tb.reply(m, fmt.Sprintf("Exit Code: %d\nOutput:\n```\n%s\n```", resp.ExitCode, out))
}

func (tb *TelegramBot) cmdBroadcast(m *tgbotapi.Message, args []string) {
	if len(args) < 1 {
		tb.reply(m, "Usage: /broadcast <command>")
		return
	}

	command := strings.Join(args, " ")
	tb.reply(m, "Broadcasting command to all online servers...")

	servers, err := tb.db.GetServers()
	if err != nil {
		tb.reply(m, fmt.Sprintf("Error: %v", err))
		return
	}

	var results []string
	for _, s := range servers {
		client, connected := tb.hub.clients[s.ID]
		isOnline := connected && client.approved && time.Since(s.LastSeenAt) < 20*time.Second
		if !isOnline {
			continue
		}

		resp, err := tb.hub.SendCommand(s.ID, command, fmt.Sprintf("telegram_user_broadcast_%d", m.From.ID))
		resLine := fmt.Sprintf("🖥️ *%s* (exit %d):", s.Hostname, resp.ExitCode)
		if err != nil {
			resLine += fmt.Sprintf(" Error: %v", err)
		} else {
			out := strings.TrimSpace(resp.Output)
			if len(out) > 200 {
				out = out[:200] + "..."
			}
			resLine += fmt.Sprintf("\n`%s`", out)
		}
		results = append(results, resLine)
	}

	if len(results) == 0 {
		tb.reply(m, "No servers are currently online.")
		return
	}

	tb.reply(m, strings.Join(results, "\n\n"))
}

func (tb *TelegramBot) cmdFile(m *tgbotapi.Message, args []string) {
	if len(args) < 3 {
		tb.reply(m, "Usage:\n/file get <server_id_prefix> <path>\n/file put <server_id_prefix> <path> (attach file to message)")
		return
	}

	subaction := args[0]
	idPrefix := args[1]
	path := args[2]

	s := tb.findServerByPrefix(idPrefix)
	if s == nil {
		tb.reply(m, "Server not found.")
		return
	}

	tb.reply(m, "Initializing file transaction...")

	switch subaction {
	case "get":
		// Initiate download, write to memory, send back to Telegram
		// We mock a local GET handler or use the same channel routing logic
		client, exists := tb.hub.clients[s.ID]
		if !exists || !client.approved {
			tb.reply(m, "Agent is offline.")
			return
		}

		transferID := fmt.Sprintf("tx_tg_%d", time.Now().UnixNano())
		req := protocol.FileGetRequest{
			TransferID: transferID,
			Path:       path,
		}

		payload, _ := json.Marshal(req)
		msg := protocol.Message{Type: protocol.TypeFileGetRequest, Payload: payload}
		msgBytes, _ := json.Marshal(msg)

		ch := make(chan *protocol.FileGetChunk, 500)
		tb.hub.transfersMu.Lock()
		tb.hub.transfers[transferID] = ch
		tb.hub.transfersMu.Unlock()

		defer func() {
			tb.hub.transfersMu.Lock()
			delete(tb.hub.transfers, transferID)
			tb.hub.transfersMu.Unlock()
		}()

		client.send <- msgBytes

		// Accumulate file bytes
		var fileBuffer bytes.Buffer

		for {
			select {
			case chunk := <-ch:
				if chunk.Error != "" {
					tb.reply(m, fmt.Sprintf("File download error: %s", chunk.Error))
					return
				}

				if chunk.Data != "" {
					data, err := base64.StdEncoding.DecodeString(chunk.Data)
					if err != nil {
						tb.reply(m, "File decoding error.")
						return
					}
					fileBuffer.Write(data)
				}

				if chunk.IsEOF {
					// Send document to chat
					reader := bytes.NewReader(fileBuffer.Bytes())
					parts := strings.Split(path, "\\")
					filename := parts[len(parts)-1]
					
					docMsg := tgbotapi.NewDocument(m.Chat.ID, tgbotapi.FileReader{
						Name:   filename,
						Reader: reader,
					})
					docMsg.Caption = fmt.Sprintf("Downloaded %s from %s", path, s.Hostname)
					_, err := tb.bot.Send(docMsg)
					if err != nil {
						tb.reply(m, fmt.Sprintf("Failed to send file over Telegram: %v", err))
					}
					return
				}
			case <-time.After(45 * time.Second):
				tb.reply(m, "File transfer request timed out.")
				return
			}
		}

	case "put":
		if m.Document == nil {
			tb.reply(m, "Error: You must attach a document file to the message to use /file put")
			return
		}

		fileURL, err := tb.bot.GetFileDirectURL(m.Document.FileID)
		if err != nil {
			tb.reply(m, fmt.Sprintf("Error getting file download link: %v", err))
			return
		}

		resp, err := http.Get(fileURL)
		if err != nil {
			tb.reply(m, fmt.Sprintf("Failed to fetch file from Telegram server: %v", err))
			return
		}
		defer resp.Body.Close()

		client, exists := tb.hub.clients[s.ID]
		if !exists || !client.approved {
			tb.reply(m, "Agent is offline.")
			return
		}

		transferID := fmt.Sprintf("tx_tg_put_%d", time.Now().UnixNano())
		putReq := protocol.FilePutRequest{
			TransferID: transferID,
			Path:       path,
			TotalSize:  resp.ContentLength,
		}

		payload, _ := json.Marshal(putReq)
		msg := protocol.Message{Type: protocol.TypeFilePutRequest, Payload: payload}
		msgBytes, _ := json.Marshal(msg)

		client.send <- msgBytes

		buf := make([]byte, 64*1024)
		chunkIdx := 0

		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				chunk := protocol.FilePutChunk{
					TransferID: transferID,
					ChunkIndex: chunkIdx,
					Data:       base64.StdEncoding.EncodeToString(buf[:n]),
					IsEOF:      false,
				}
				chunkIdx++

				p, _ := json.Marshal(chunk)
				mMsg := protocol.Message{Type: protocol.TypeFilePutChunk, Payload: p}
				mBytes, _ := json.Marshal(mMsg)
				client.send <- mBytes
			}

			if err == io.EOF {
				chunk := protocol.FilePutChunk{
					TransferID: transferID,
					ChunkIndex: chunkIdx,
					IsEOF:      true,
				}
				p, _ := json.Marshal(chunk)
				mMsg := protocol.Message{Type: protocol.TypeFilePutChunk, Payload: p}
				mBytes, _ := json.Marshal(mMsg)
				client.send <- mBytes
				break
			} else if err != nil {
				chunk := protocol.FilePutChunk{
					TransferID: transferID,
					Error:      err.Error(),
					IsEOF:      true,
				}
				p, _ := json.Marshal(chunk)
				mMsg := protocol.Message{Type: protocol.TypeFilePutChunk, Payload: p}
				mBytes, _ := json.Marshal(mMsg)
				client.send <- mBytes
				tb.reply(m, fmt.Sprintf("File upload failed: %v", err))
				return
			}
		}

		tb.reply(m, fmt.Sprintf("✅ File pushed to %s at %s", s.Hostname, path))
	}
}

func (tb *TelegramBot) findServerByPrefix(prefix string) *Server {
	servers, err := tb.db.GetServers()
	if err != nil {
		return nil
	}

	for _, s := range servers {
		if strings.HasPrefix(s.ID, prefix) || strings.HasPrefix(strings.ToLower(s.Hostname), strings.ToLower(prefix)) {
			return s
		}
		if s.CustomName != nil && strings.HasPrefix(strings.ToLower(*s.CustomName), strings.ToLower(prefix)) {
			return s
		}
	}
	return nil
}
