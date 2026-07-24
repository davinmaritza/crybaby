package main

import (
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cyrbaby/pkg/dashboard"
)

var (
	tgBot *TelegramBot
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config.json", "Path to config file")
	flag.Parse()

	loadConfig(configFile)

	// Initialize DB
	db, err := NewDB(globalConfig.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("SQLite database initialized at: %s", globalConfig.DBPath)

	// Initialize WebSocket Hub
	hub := NewHub(db)
	go hub.Run()
	log.Println("WebSocket Hub started.")

	// Initialize Telegram Bot
	tgBot, err = InitTelegramBot(globalConfig.TelegramToken, globalConfig.TelegramAdminIDs, db, hub)
	if err != nil {
		log.Printf("Failed to initialize Telegram Bot: %v", err)
	}

	// Periodically prune old metrics (older than 7 days)
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			deleted, err := db.PruneMetricSamples(7 * 24 * time.Hour)
			if err != nil {
				log.Printf("Failed to prune metric samples: %v", err)
			} else if deleted > 0 {
				log.Printf("Pruned %d metric samples older than 7 days.", deleted)
			}
		}
	}()

	// Setup multiplexer
	mux := http.NewServeMux()

	// REST APIs (Auth)
	mux.HandleFunc("POST /api/login", handleLogin)
	mux.HandleFunc("POST /api/logout", handleLogout)
	mux.HandleFunc("GET /api/auth/check", handleCheckAuth)

	// REST APIs (Fleet Operations)
	mux.HandleFunc("GET /api/status", authMiddleware(handleStatus(db, hub)))
	mux.HandleFunc("GET /api/servers", authMiddleware(handleServers(db, hub)))
	mux.HandleFunc("GET /api/servers/{id}", authMiddleware(handleServerDetail(db, hub)))
	mux.HandleFunc("POST /api/servers/{id}/rename", authMiddleware(handleServerRename(db)))
	mux.HandleFunc("POST /api/servers/{id}/cluster", authMiddleware(handleServerCluster(db)))
	mux.HandleFunc("POST /api/servers/{id}/approve", authMiddleware(handleServerApprove(db, hub)))
	mux.HandleFunc("POST /api/servers/{id}/reject", authMiddleware(handleServerReject(db, hub)))
	mux.HandleFunc("POST /api/servers/{id}/remove", authMiddleware(handleServerRemove(db, hub)))
	mux.HandleFunc("POST /api/servers/{id}/exec", authMiddleware(handleServerExec(db, hub)))
	mux.HandleFunc("GET /api/servers/{id}/file/get", authMiddleware(handleFileDownload(hub)))
	mux.HandleFunc("POST /api/servers/{id}/file/put", authMiddleware(handleFileUpload(hub)))

	// REST APIs (Cluster Management)
	mux.HandleFunc("GET /api/clusters", authMiddleware(handleClustersGet(db)))
	mux.HandleFunc("POST /api/clusters", authMiddleware(handleClusterCreate(db)))
	mux.HandleFunc("PUT /api/clusters/{id}", authMiddleware(handleClusterUpdate(db)))
	mux.HandleFunc("DELETE /api/clusters/{id}", authMiddleware(handleClusterDelete(db)))

	// REST APIs (Command logs)
	mux.HandleFunc("GET /api/logs", authMiddleware(handleLogs(db)))

	// REST APIs (AI Agent Console)
	mux.HandleFunc("POST /api/ai/chat", authMiddleware(handleAIChat(db, hub)))

	// WebSocket handler for RMM Agents
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Serve embedded dashboard
	subFS, err := fs.Sub(dashboard.Assets, "assets")
	if err != nil {
		log.Fatalf("Failed to resolve embedded assets: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	addr := ":" + globalConfig.Port
	log.Printf("CryBaby Backend server listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server ListenAndServe failed: %v", err)
	}
}

func loadConfig(path string) {
	configMu.Lock()
	defer configMu.Unlock()

	// Default values
	globalConfig = Config{
		Port:             "8080",
		DBPath:           "crybaby.db",
		AdminPassword:    "admin",
		TelegramAdminIDs: []int64{},
	}

	// Read file
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &globalConfig); err != nil {
			log.Printf("Warning: Failed to parse config file: %v. Using defaults.", err)
		}
	} else {
		log.Printf("Config file not found, creating default one at: %s", path)
		dBytes, _ := json.MarshalIndent(globalConfig, "", "  ")
		os.WriteFile(path, dBytes, 0644)
	}

	// Env overrides
	if val := os.Getenv("PORT"); val != "" {
		globalConfig.Port = val
	}
	if val := os.Getenv("DB_PATH"); val != "" {
		globalConfig.DBPath = val
	}
	if val := os.Getenv("ADMIN_PASSWORD"); val != "" {
		globalConfig.AdminPassword = val
	}
	if val := os.Getenv("TELEGRAM_TOKEN"); val != "" {
		globalConfig.TelegramToken = val
	}
	if val := os.Getenv("TELEGRAM_ADMIN_IDS"); val != "" {
		parts := strings.Split(val, ",")
		var ids []int64
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err == nil {
				ids = append(ids, id)
			}
		}
		globalConfig.TelegramAdminIDs = ids
	}
	if val := os.Getenv("GEMINI_API_KEY"); val != "" {
		globalConfig.GeminiAPIKey = val
	}
	if val := os.Getenv("OLLAMA_URL"); val != "" {
		globalConfig.OllamaURL = val
	}
	if val := os.Getenv("OLLAMA_MODEL"); val != "" {
		globalConfig.OllamaModel = val
	}
	if val := os.Getenv("AUTO_APPROVE"); val == "true" || val == "1" {
		globalConfig.AutoApprove = true
	}
}

