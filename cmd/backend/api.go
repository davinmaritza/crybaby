package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"cyrbaby/pkg/protocol"
	"github.com/google/uuid"
)

type Config struct {
	Port              string   `json:"port"`
	DBPath            string   `json:"db_path"`
	AdminPassword     string   `json:"admin_password"`
	TelegramToken     string   `json:"telegram_token"`
	TelegramAdminIDs  []int64  `json:"telegram_admin_ids"`
	GeminiAPIKey      string   `json:"gemini_api_key"`
	OllamaURL         string   `json:"ollama_url"`
	OllamaModel       string   `json:"ollama_model"`
	AutoApprove       bool     `json:"auto_approve"`
	AllowedAdminUsers []string `json:"allowed_admin_users"`
}


var (
	globalConfig Config
	configMu     sync.RWMutex
)

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configMu.RLock()
		pwd := globalConfig.AdminPassword
		configMu.RUnlock()

		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != pwd {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	configMu.RLock()
	pwd := globalConfig.AdminPassword
	configMu.RUnlock()

	if req.Password != pwd {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    pwd,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: false, // Accessible by Javascript for simple check
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: false,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleCheckAuth(w http.ResponseWriter, r *http.Request) {
	configMu.RLock()
	pwd := globalConfig.AdminPassword
	configMu.RUnlock()

	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value != pwd {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": true})
}

func handleStatus(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		servers, err := db.GetServers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var (
			total    int
			online   int
			offline  int
			pending  int
			cores    int
			threads  int
			totalRAM uint64
			totalDisk uint64
		)

		hub.clientsMu.RLock()
		for _, s := range servers {
			if s.Status == "pending_approval" {
				pending++
				continue
			}

			total++
			cores += s.CPUCores
			threads += s.CPUThreads
			totalRAM += s.RAMTotalMB
			totalDisk += s.DiskTotalMB

			// Check online status based on connection in hub AND last seen time (within 20s)
			client, connected := hub.clients[s.ID]
			if connected && client.approved && time.Since(s.LastSeenAt) < 20*time.Second {
				online++
			} else {
				offline++
			}
		}
		hub.clientsMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_servers":   total,
			"online_servers":  online,
			"offline_servers": offline,
			"pending_servers": pending,
			"total_cores":     cores,
			"total_threads":   threads,
			"total_ram_mb":    totalRAM,
			"total_disk_mb":   totalDisk,
		})
	}
}

func handleServers(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		servers, err := db.GetServers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type ServerListResponse struct {
			*Server
			IsOnline      bool                 `json:"is_online"`
			RecentMetrics *protocol.Heartbeat  `json:"recent_metrics,omitempty"`
		}

		list := make([]ServerListResponse, 0, len(servers))

		hub.clientsMu.RLock()
		for _, s := range servers {
			client, connected := hub.clients[s.ID]
			isOnline := connected && client.approved && time.Since(s.LastSeenAt) < 20*time.Second

			var recentMetrics *protocol.Heartbeat
			if isOnline {
				// Query last metric sample
				samples, err := db.GetRecentMetricSamples(s.ID, 1)
				if err == nil && len(samples) > 0 {
					recentMetrics = &protocol.Heartbeat{
						CPULoadPct:    samples[0].CPULoadPct,
						RAMUsedMB:     samples[0].RAMUsedMB,
						DiskUsedMB:    samples[0].DiskUsedMB,
						UptimeSeconds: samples[0].UptimeSeconds,
					}
				}
			}

			list = append(list, ServerListResponse{
				Server:        s,
				IsOnline:      isOnline,
				RecentMetrics: recentMetrics,
			})
		}
		hub.clientsMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

func handleServerDetail(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		s, err := db.GetServer(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if s == nil {
			http.Error(w, "Server not found", http.StatusNotFound)
			return
		}

		samples, _ := db.GetRecentMetricSamples(id, 20) // Get last 20 metrics for charts

		hub.clientsMu.RLock()
		client, connected := hub.clients[s.ID]
		isOnline := connected && client.approved && time.Since(s.LastSeenAt) < 20*time.Second
		hub.clientsMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"server":   s,
			"is_online": isOnline,
			"metrics":   samples,
		})
	}
}

func handleServerRename(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			CustomName string `json:"custom_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var namePtr *string
		if req.CustomName != "" {
			namePtr = &req.CustomName
		}

		if err := db.UpdateServerCustomName(id, namePtr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleServerCluster(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			ClusterID string `json:"cluster_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var clusterPtr *string
		if req.ClusterID != "" {
			clusterPtr = &req.ClusterID
		}

		if err := db.SetServerCluster(id, clusterPtr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleServerApprove(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := hub.ApprovePendingAgent(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleServerReject(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := hub.RejectPendingAgent(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleServerRemove(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := hub.DecommissionAgent(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleServerExec(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp, err := hub.SendCommand(id, req.Command, "web_user")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleFileDownload(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Missing path parameter", http.StatusBadRequest)
			return
		}

		hub.clientsMu.RLock()
		client, exists := hub.clients[id]
		hub.clientsMu.RUnlock()

		if !exists || !client.approved {
			http.Error(w, "Agent not connected or not approved", http.StatusNotFound)
			return
		}

		transferID := fmt.Sprintf("tx_%d", time.Now().UnixNano())
		req := protocol.FileGetRequest{
			TransferID: transferID,
			Path:       path,
		}

		payload, _ := json.Marshal(req)
		msg := protocol.Message{
			Type:    protocol.TypeFileGetRequest,
			Payload: payload,
		}
		msgBytes, _ := json.Marshal(msg)

		ch := make(chan *protocol.FileGetChunk, 100)
		hub.transfersMu.Lock()
		hub.transfers[transferID] = ch
		hub.transfersMu.Unlock()

		defer func() {
			hub.transfersMu.Lock()
			delete(hub.transfers, transferID)
			hub.transfersMu.Unlock()
		}()

		_ = hub.db.LogCommandStart(transferID, id, "web_user", fmt.Sprintf("Download file: %s", path))

		// Send request
		client.send <- msgBytes

		filename := filepath.Base(path)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Type", "application/octet-stream")

		for {
			select {
			case chunk := <-ch:
				if chunk.Error != "" {
					http.Error(w, chunk.Error, http.StatusInternalServerError)
					_ = hub.db.LogCommandComplete(transferID, fmt.Sprintf("Failed: %s", chunk.Error))
					return
				}

				if chunk.Data != "" {
					data, err := base64.StdEncoding.DecodeString(chunk.Data)
					if err != nil {
						http.Error(w, "Failed to decode chunk", http.StatusInternalServerError)
						_ = hub.db.LogCommandComplete(transferID, fmt.Sprintf("Failed to decode base64: %v", err))
						return
					}
					w.Write(data)
				}

				if chunk.IsEOF {
					_ = hub.db.LogCommandComplete(transferID, "Completed successfully")
					return
				}
			case <-time.After(60 * time.Second):
				http.Error(w, "Timeout waiting for agent file response", http.StatusGatewayTimeout)
				_ = hub.db.LogCommandComplete(transferID, "Failed: Timeout")
				return
			}
		}
	}
}

func handleFileUpload(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Missing path parameter", http.StatusBadRequest)
			return
		}

		hub.clientsMu.RLock()
		client, exists := hub.clients[id]
		hub.clientsMu.RUnlock()

		if !exists || !client.approved {
			http.Error(w, "Agent not connected or not approved", http.StatusNotFound)
			return
		}

		err := r.ParseMultipartForm(50 * 1024 * 1024)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Missing file parameter", http.StatusBadRequest)
			return
		}
		defer file.Close()

		transferID := fmt.Sprintf("tx_%d", time.Now().UnixNano())
		req := protocol.FilePutRequest{
			TransferID: transferID,
			Path:       path,
			TotalSize:  header.Size,
		}

		payload, _ := json.Marshal(req)
		msg := protocol.Message{
			Type:    protocol.TypeFilePutRequest,
			Payload: payload,
		}
		msgBytes, _ := json.Marshal(msg)

		_ = hub.db.LogCommandStart(transferID, id, "web_user", fmt.Sprintf("Upload file: %s (%d bytes)", path, header.Size))

		// Initiate transfer
		client.send <- msgBytes

		buf := make([]byte, 64*1024)
		chunkIdx := 0

		for {
			n, err := file.Read(buf)
			if n > 0 {
				chunk := protocol.FilePutChunk{
					TransferID: transferID,
					ChunkIndex: chunkIdx,
					Data:       base64.StdEncoding.EncodeToString(buf[:n]),
					IsEOF:      false,
				}
				chunkIdx++

				p, _ := json.Marshal(chunk)
				m := protocol.Message{
					Type:    protocol.TypeFilePutChunk,
					Payload: p,
				}
				mBytes, _ := json.Marshal(m)
				client.send <- mBytes
			}

			if err == io.EOF {
				chunk := protocol.FilePutChunk{
					TransferID: transferID,
					ChunkIndex: chunkIdx,
					Data:       "",
					IsEOF:      true,
				}
				p, _ := json.Marshal(chunk)
				m := protocol.Message{
					Type:    protocol.TypeFilePutChunk,
					Payload: p,
				}
				mBytes, _ := json.Marshal(m)
				client.send <- mBytes
				break
			} else if err != nil {
				chunk := protocol.FilePutChunk{
					TransferID: transferID,
					Error:      err.Error(),
					IsEOF:      true,
				}
				p, _ := json.Marshal(chunk)
				m := protocol.Message{
					Type:    protocol.TypeFilePutChunk,
					Payload: p,
				}
				mBytes, _ := json.Marshal(m)
				client.send <- mBytes

				http.Error(w, err.Error(), http.StatusInternalServerError)
				_ = hub.db.LogCommandComplete(transferID, fmt.Sprintf("Failed during upload: %v", err))
				return
			}
		}

		_ = hub.db.LogCommandComplete(transferID, "Completed successfully")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleClustersGet(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := db.GetClusters()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

func handleClusterCreate(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		id := uuid.New().String()
		if err := db.CreateCluster(id, req.Name, req.Description); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": id})
	}
}

func handleClusterUpdate(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := db.UpdateCluster(id, req.Name, req.Description); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleClusterDelete(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := db.DeleteCluster(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleLogs(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs, err := db.GetCommandLogs(100) // Get last 100 logs
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
	}
}

func handleAIChat(db *DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Message string `json:"message"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		reply, err := AskAI(req.Message, db, hub)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"reply": reply})
	}
}

