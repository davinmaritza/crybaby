package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const banner = `
   ██████╗██████╗ ██╗   ██╗██████╗  █████╗ ██████╗ ██╗   ██╗
  ██╔════╝██╔══██╗╚██╗ ██╔╝██╔══██╗██╔══██╗██╔══██╗╚██╗ ██╔╝
  ██║     ██████╔╝ ╚████╔╝ ██████╔╝███████║██████╔╝ ╚████╔╝ 
  ██║     ██╔══██╗  ╚██╔╝  ██╔══██╗██╔══██║██╔══██╗  ╚██╔╝  
  ╚██████╗██║  ██║   ██║   ██████╔╝██║  ██║██████╔╝   ██║   
   ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═════╝ ╚═╝  ╚═╝╚═════╝    ╚═╝   
`

const helpText = `
  ── Fleet Commands ──
    /status          - Show fleet overview (online/offline, total CPU/RAM)
    /list            - List all active servers with metrics
    /info  <uuid>    - Show detailed info for a specific server
    /pending         - Show servers waiting for approval

  ── Server Actions ──
    /approve <uuid>  - Approve a pending server
    /reject  <uuid>  - Reject a pending server
    /remove  <uuid>  - Remove & decommission a server
    /rename  <uuid> <name> - Set custom name for a server
    /exec    <uuid> <cmd>  - Run a shell command on a server
    /broadcast <cmd> - Run a command on ALL online servers

  ── Cluster Management ──
    /clusters        - List all clusters
    /cluster-create <name> - Create a new cluster

  ── Audit & Logs ──
    /logs            - Show last 20 audit log entries

  ── AI Agent ──
    /ai <prompt>     - Chat with Gemini AI Agent
    (or just type any message without / to chat AI)

  ── Session ──
    /server <url>    - Switch backend server URL
    /config          - Show current session config
    /help            - Show this help message
    /exit            - Exit CryBaby CLI

  Tip: Type any message without a slash to chat with the AI Agent!
`

type Server struct {
	ID           string  `json:"id"`
	Hostname     string  `json:"hostname"`
	CustomName   *string `json:"custom_name"`
	OSVersion    string  `json:"os_version"`
	CPUCores     int     `json:"cpu_cores"`
	CPUThreads   int     `json:"cpu_threads"`
	RAMTotalMB   uint64  `json:"ram_total_mb"`
	DiskTotalMB  uint64  `json:"disk_total_mb"`
	Status       string  `json:"status"`
	IsOnline     bool    `json:"is_online"`
	ClusterName  *string `json:"cluster_name"`
	RecentMetrics *struct {
		CPULoadPct float64 `json:"cpu_load_pct"`
		RAMUsedMB  uint64  `json:"ram_used_mb"`
		UptimeSecs int64   `json:"uptime_seconds"`
	} `json:"recent_metrics"`
}

type Cluster struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ServerCount int    `json:"server_count"`
}

type LogEntry struct {
	ID         string `json:"id"`
	ServerName string `json:"server_name"`
	IssuedBy   string `json:"issued_by"`
	Command    string `json:"command"`
	Result     string `json:"result"`
	IssuedAt   string `json:"issued_at"`
}

type AIResponse struct {
	Response string `json:"response"`
}

var (
	serverURL string
	password  string
	cookies   []*http.Cookie
	client    = &http.Client{Timeout: 30 * time.Second}
)

func main() {
	flagServer := flag.String("server", "http://your-backend-domain.com:25583", "Cyrbaby backend server URL")
	flagPass := flag.String("pass", "admin", "Admin password")


	flag.Parse()

	serverURL = *flagServer
	password = *flagPass

	fmt.Print(banner)
	fmt.Println("  RMM — Remote Monitoring & Management CLI")
	fmt.Println("  by Suzirz  |  Version 1.0.0")
	fmt.Printf("  Backend: %s\n", serverURL)
	fmt.Println(strings.Repeat("─", 72))
	fmt.Println()

	if !doLogin() {
		return
	}

	fmt.Println("  [OK] Authenticated. Type /help for commands or just chat with AI.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("crybaby> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			handleSlashCommand(input, scanner)
		} else {
			// Default: send to AI
			doAI(input)
		}
	}
}

func doLogin() bool {
	fmt.Printf("  Connecting to %s...\n", serverURL)
	body, _ := json.Marshal(map[string]string{"password": password})
	resp, err := client.Post(serverURL+"/api/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("  [ERROR] Cannot reach backend: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("  [ERROR] Authentication failed. Use -pass to set the correct password.")
		return false
	}

	cookies = resp.Cookies()
	return true
}

func handleSlashCommand(input string, scanner *bufio.Scanner) {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/help":
		fmt.Println(helpText)

	case "/exit", "/quit":
		fmt.Println("  Goodbye!")
		os.Exit(0)

	case "/config":
		fmt.Printf("\n  Backend  : %s\n  Password : %s\n\n", serverURL, strings.Repeat("*", len(password)))

	case "/server":
		if len(args) == 0 {
			fmt.Println("  Usage: /server <url>")
			return
		}
		serverURL = args[0]
		fmt.Printf("  Backend URL changed to: %s\n", serverURL)
		doLogin()

	case "/status":
		doStatus()

	case "/list":
		doList()

	case "/pending":
		doPending()

	case "/info":
		if len(args) == 0 {
			fmt.Println("  Usage: /info <uuid>")
			return
		}
		doInfo(args[0])

	case "/approve":
		if len(args) == 0 {
			fmt.Println("  Usage: /approve <uuid>")
			return
		}
		doAction(fmt.Sprintf("/api/servers/%s/approve", args[0]), "POST", nil, "Approved")

	case "/reject":
		if len(args) == 0 {
			fmt.Println("  Usage: /reject <uuid>")
			return
		}
		doAction(fmt.Sprintf("/api/servers/%s/reject", args[0]), "POST", nil, "Rejected")

	case "/remove":
		if len(args) == 0 {
			fmt.Println("  Usage: /remove <uuid>")
			return
		}
		doAction(fmt.Sprintf("/api/servers/%s/remove", args[0]), "POST", nil, "Removed")

	case "/rename":
		if len(args) < 2 {
			fmt.Println("  Usage: /rename <uuid> <name>")
			return
		}
		doAction(fmt.Sprintf("/api/servers/%s/rename", args[0]), "POST",
			map[string]string{"custom_name": strings.Join(args[1:], " ")}, "Renamed")

	case "/exec":
		if len(args) < 2 {
			fmt.Println("  Usage: /exec <uuid> <command>")
			return
		}
		doExec(args[0], strings.Join(args[1:], " "))

	case "/broadcast":
		if len(args) == 0 {
			fmt.Println("  Usage: /broadcast <command>")
			return
		}
		doAction("/api/broadcast", "POST",
			map[string]string{"command": strings.Join(args, " ")}, "Broadcast sent")

	case "/clusters":
		doClusters()

	case "/cluster-create":
		if len(args) == 0 {
			fmt.Println("  Usage: /cluster-create <name>")
			return
		}
		doAction("/api/clusters", "POST",
			map[string]string{"name": strings.Join(args, " ")}, "Cluster created")

	case "/logs":
		doLogs()

	case "/ai":
		if len(args) == 0 {
			fmt.Println("  Usage: /ai <prompt>")
			return
		}
		doAI(strings.Join(args, " "))

	default:
		fmt.Printf("  Unknown command: %s. Type /help for available commands.\n\n", cmd)
	}
}

func apiRequest(method, path string, payload interface{}) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, serverURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	data, _ := io.ReadAll(res.Body)
	return data, res.StatusCode, nil
}

func doStatus() {
	data, _, err := apiRequest("GET", "/api/servers", nil)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	var servers []Server
	json.Unmarshal(data, &servers)

	var online, offline, pending, totalCores int
	var totalRAM, totalUsedRAM uint64
	var cpuAcc float64

	for _, s := range servers {
		switch s.Status {
		case "pending_approval":
			pending++
		default:
			if s.IsOnline {
				online++
				totalCores += s.CPUCores
				totalRAM += s.RAMTotalMB
				if s.RecentMetrics != nil {
					cpuAcc += s.RecentMetrics.CPULoadPct
					totalUsedRAM += s.RecentMetrics.RAMUsedMB
				}
			} else {
				offline++
			}
		}
	}

	avgCPU := 0.0
	if online > 0 {
		avgCPU = cpuAcc / float64(online)
	}
	ramPct := 0.0
	if totalRAM > 0 {
		ramPct = float64(totalUsedRAM) / float64(totalRAM) * 100
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Fleet Status")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  ✔ Online   : %d\n", online)
	fmt.Printf("  ✖ Offline  : %d\n", offline)
	fmt.Printf("  ⏳ Pending  : %d\n", pending)
	fmt.Printf("  Total Cores: %d\n", totalCores)
	fmt.Printf("  Avg CPU    : %.1f%%\n", avgCPU)
	fmt.Printf("  RAM Used   : %.1f GiB / %.1f GiB (%.1f%%)\n",
		float64(totalUsedRAM)/1024, float64(totalRAM)/1024, ramPct)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()
}

func doList() {
	data, _, err := apiRequest("GET", "/api/servers", nil)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	var servers []Server
	json.Unmarshal(data, &servers)

	fmt.Println()
	fmt.Printf("  %-8s %-6s %-20s %-8s %-8s %s\n",
		"ID", "Status", "Name", "CPU%", "RAM%", "Cluster")
	fmt.Println(strings.Repeat("─", 72))
	for _, s := range servers {
		if s.Status == "pending_approval" {
			continue
		}
		status := "OFFLINE"
		if s.IsOnline {
			status = "ONLINE"
		}
		name := s.Hostname
		if s.CustomName != nil && *s.CustomName != "" {
			name = *s.CustomName
		}
		cluster := "Unassigned"
		if s.ClusterName != nil {
			cluster = *s.ClusterName
		}
		cpuStr := "—"
		ramStr := "—"
		if s.IsOnline && s.RecentMetrics != nil {
			cpuStr = fmt.Sprintf("%.0f%%", s.RecentMetrics.CPULoadPct)
			if s.RAMTotalMB > 0 {
				ramStr = fmt.Sprintf("%.0f%%", float64(s.RecentMetrics.RAMUsedMB)/float64(s.RAMTotalMB)*100)
			}
		}
		fmt.Printf("  %-8s %-6s %-20s %-8s %-8s %s\n",
			s.ID[:8], status, name, cpuStr, ramStr, cluster)
	}
	fmt.Println()
}

func doInfo(uuid string) {
	data, _, err := apiRequest("GET", "/api/servers/"+uuid, nil)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	var res struct {
		Server  Server `json:"server"`
		IsOnline bool  `json:"is_online"`
	}
	json.Unmarshal(data, &res)
	s := res.Server

	name := s.Hostname
	if s.CustomName != nil && *s.CustomName != "" {
		name = *s.CustomName + " (" + s.Hostname + ")"
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %s\n", name)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  UUID      : %s\n", s.ID)
	fmt.Printf("  OS        : %s\n", s.OSVersion)
	fmt.Printf("  CPU       : %d Cores / %d Threads\n", s.CPUCores, s.CPUThreads)
	fmt.Printf("  RAM       : %.1f GB\n", float64(s.RAMTotalMB)/1024)
	fmt.Printf("  Disk      : %.1f GB\n", float64(s.DiskTotalMB)/1024)
	fmt.Printf("  Online    : %v\n", res.IsOnline)
	if res.IsOnline && s.RecentMetrics != nil {
		fmt.Printf("  CPU Load  : %.1f%%\n", s.RecentMetrics.CPULoadPct)
		fmt.Printf("  RAM Used  : %.1f GB\n", float64(s.RecentMetrics.RAMUsedMB)/1024)
		up := s.RecentMetrics.UptimeSecs
		fmt.Printf("  Uptime    : %dd %dh %dm\n", up/86400, (up%86400)/3600, (up%3600)/60)
	}
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()
}

func doPending() {
	data, _, err := apiRequest("GET", "/api/servers", nil)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	var servers []Server
	json.Unmarshal(data, &servers)

	fmt.Println()
	found := false
	for _, s := range servers {
		if s.Status == "pending_approval" {
			if !found {
				fmt.Printf("  %-8s %-20s %-12s %s\n", "UUID", "Hostname", "OS", "RAM")
				fmt.Println(strings.Repeat("─", 60))
				found = true
			}
			fmt.Printf("  %-8s %-20s %-12s %.1f GB\n",
				s.ID[:8], s.Hostname, s.OSVersion, float64(s.RAMTotalMB)/1024)
		}
	}
	if !found {
		fmt.Println("  No pending devices.")
	}
	fmt.Println()
}

func doExec(uuid, command string) {
	fmt.Printf("  Executing on %s: %s\n", uuid[:8], command)
	data, status, err := apiRequest("POST", fmt.Sprintf("/api/servers/%s/exec", uuid),
		map[string]string{"command": command})
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("  Server error (%d): %s\n\n", status, string(data))
		return
	}
	var res struct {
		Output   string `json:"output"`
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}
	json.Unmarshal(data, &res)
	fmt.Println()
	if res.Output != "" {
		fmt.Println(res.Output)
	}
	if res.Error != "" {
		fmt.Printf("  [Error] %s\n", res.Error)
	}
	fmt.Printf("  [Exit Code: %d]\n\n", res.ExitCode)
}

func doClusters() {
	data, _, err := apiRequest("GET", "/api/clusters", nil)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	var clusters []Cluster
	json.Unmarshal(data, &clusters)

	fmt.Println()
	if len(clusters) == 0 {
		fmt.Println("  No clusters created yet.")
		fmt.Println()
		return
	}
	fmt.Printf("  %-10s %-20s %-30s %s\n", "ID", "Name", "Description", "Servers")
	fmt.Println(strings.Repeat("─", 70))
	for _, c := range clusters {
		desc := c.Description
		if len(desc) > 28 {
			desc = desc[:28] + ".."
		}
		fmt.Printf("  %-10s %-20s %-30s %d\n", c.ID[:8], c.Name, desc, c.ServerCount)
	}
	fmt.Println()
}

func doLogs() {
	data, _, err := apiRequest("GET", "/api/audit-logs", nil)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	var logs []LogEntry
	json.Unmarshal(data, &logs)

	fmt.Println()
	if len(logs) == 0 {
		fmt.Println("  No audit logs found.")
		fmt.Println()
		return
	}
	fmt.Printf("  %-20s %-15s %-10s %s\n", "Time", "Server", "By", "Command")
	fmt.Println(strings.Repeat("─", 72))
	limit := 20
	if len(logs) < limit {
		limit = len(logs)
	}
	for _, l := range logs[:limit] {
		cmd := l.Command
		if len(cmd) > 35 {
			cmd = cmd[:35] + ".."
		}
		t := l.IssuedAt
		if len(t) > 16 {
			t = t[:16]
		}
		srv := l.ServerName
		if len(srv) > 14 {
			srv = srv[:14]
		}
		fmt.Printf("  %-20s %-15s %-10s %s\n", t, srv, l.IssuedBy, cmd)
	}
	fmt.Println()
}

func doAction(path, method string, payload interface{}, successMsg string) {
	data, status, err := apiRequest(method, path, payload)
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	if status == http.StatusOK || status == http.StatusCreated {
		fmt.Printf("  [OK] %s\n\n", successMsg)
	} else {
		fmt.Printf("  [Error %d]: %s\n\n", status, string(data))
	}
}

func doAI(prompt string) {
	fmt.Println("  Thinking...")
	data, status, err := apiRequest("POST", "/api/ai/chat",
		map[string]string{"message": prompt})
	if err != nil {
		fmt.Printf("  Error: %v\n\n", err)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("  AI error (%d): %s\n\n", status, string(data))
		return
	}
	var res AIResponse
	json.Unmarshal(data, &res)
	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  AI Agent:\n  %s\n", strings.ReplaceAll(res.Response, "\n", "\n  "))
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()
}
