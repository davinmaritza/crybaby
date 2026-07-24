package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"cyrbaby/pkg/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows/svc"
)

const AgentVersion = "1.0.0"

type Config struct {
	UUID       string `json:"uuid"`
	Token      string `json:"token,omitempty"`
	BackendURL string `json:"backend_url"`
}

var (
	configPath     string
	config         Config
	conn           *websocket.Conn
	connMu         time.Duration // Backoff counter
	writeMu        sync.Mutex
	putTransfers   = make(map[string]chan *protocol.FilePutChunk)
	putTransfersMu sync.Mutex
)

type myService struct{}

func (m *myService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	go runAgent()

loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			break loop
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	return
}

func main() {
	flag.StringVar(&configPath, "config", "agent_config.json", "Path to agent configuration file")
	flag.Parse()

	if runtime.GOOS == "windows" {
		isService, err := svc.IsWindowsService()
		if err == nil && isService {
			svc.Run("CryBabyAgent", &myService{})
			return
		}
	}

	runAgent()
}

func runAgent() {
	if !filepath.IsAbs(configPath) {
		exePath, err := os.Executable()
		if err == nil {
			configPath = filepath.Join(filepath.Dir(exePath), configPath)
		}
	}

	loadOrCreateConfig()

	log.Printf("Starting CryBaby Agent v%s", AgentVersion)
	log.Printf("Device UUID: %s", config.UUID)
	log.Printf("Connecting to Backend: %s", config.BackendURL)

	reconnectLoop()
}



func loadOrCreateConfig() {
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &config); err == nil && config.UUID != "" {
			if config.BackendURL == "" {
				config.BackendURL = "ws://your-backend-domain.com:25583/ws"
			}
			return
		}
	}

	// Generate new config
	config = Config{
		UUID:       uuid.New().String(),
		Token:      "",
		BackendURL: "ws://your-backend-domain.com:25583/ws",
	}


	saveConfig()
}

func saveConfig() {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config: %v", err)
		return
	}
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		log.Printf("Failed to write config: %v", err)
	}
}

func reconnectLoop() {
	backoff := 2 * time.Second
	maxBackoff := 60 * time.Second

	for {
		err := connectAndServe()
		if err != nil {
			log.Printf("Connection error: %v. Reconnecting in %v...", err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			// Add jitter
			backoff += time.Duration(rand.Intn(1000)) * time.Millisecond
		} else {
			backoff = 2 * time.Second // Reset backoff
		}
	}
}

func connectAndServe() error {
	u, err := url.Parse(config.BackendURL)
	if err != nil {
		return fmt.Errorf("invalid backend URL: %w", err)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	conn = c
	defer conn.Close()

	log.Println("Successfully connected to backend WebSocket!")

	// 1. Send registration
	sysInfo, err := GetSystemInfo(AgentVersion)
	if err != nil {
		log.Printf("Error getting system info: %v", err)
		sysInfo = &SysInfo{
			Hostname:     "Unknown-Host",
			OSVersion:    "Windows",
			AgentVersion: AgentVersion,
		}
	}

	regReq := protocol.RegisterRequest{
		UUID:         config.UUID,
		Hostname:     sysInfo.Hostname,
		OSVersion:    sysInfo.OSVersion,
		CPUModel:     sysInfo.CPUModel,
		CPUCores:     sysInfo.CPUCores,
		CPUThreads:   sysInfo.CPUThreads,
		RAMTotalMB:   sysInfo.RAMTotalMB,
		DiskTotalMB:  sysInfo.DiskTotalMB,
		AgentVersion: sysInfo.AgentVersion,
		Token:        config.Token,
	}

	payload, err := json.Marshal(regReq)
	if err != nil {
		return err
	}

	regMsg := protocol.Message{
		Type:    protocol.TypeRegisterRequest,
		Payload: payload,
	}

	if err := writeJSONSafe(regMsg); err != nil {
		return err
	}

	// 2. Start heartbeats
	done := make(chan struct{})
	defer close(done)
	go startHeartbeatTicker(done)

	// 3. Read loop
	for {
		var msg protocol.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			return err
		}

		go handleServerMessage(msg)
	}
}

func startHeartbeatTicker(done chan struct{}) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics, err := GetLiveMetrics()
			if err != nil {
				log.Printf("Error getting live metrics: %v", err)
				continue
			}

			hb := protocol.Heartbeat{
				CPULoadPct:    metrics.CPULoadPct,
				RAMUsedMB:     metrics.RAMUsedMB,
				DiskUsedMB:    metrics.DiskUsedMB,
				UptimeSeconds: metrics.UptimeSeconds,
			}

			payload, _ := json.Marshal(hb)
			msg := protocol.Message{
				Type:    protocol.TypeHeartbeat,
				Payload: payload,
			}

			// We need a lock or write thread to prevent concurrent writes, gorilla websocket is not thread safe
			// Let's use simple WriteJSON since this ticker is the only periodic sender
			err = writeJSONSafe(msg)
			if err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
				return
			}
		case <-done:
			return
		}
	}
}

func handleServerMessage(msg protocol.Message) {
	switch msg.Type {
	case protocol.TypeRegisterResponse:
		var resp protocol.RegisterResponse
		if err := json.Unmarshal(msg.Payload, &resp); err != nil {
			log.Printf("Failed to unmarshal register response: %v", err)
			return
		}

		if !resp.Success {
			log.Printf("Registration rejected: %s", resp.Message)
			if resp.Status == "rejected" || resp.Status == "removed" {
				// Uninstall signal
				uninstallAgent()
			}
			conn.Close()
			return
		}

		log.Printf("Registration status: %s", resp.Status)
		if resp.Token != "" && resp.Token != config.Token {
			config.Token = resp.Token
			saveConfig()
			log.Println("New authentication token saved.")
		}

	case protocol.TypeCommandRequest:
		var req protocol.CommandRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			log.Printf("Failed to parse command request: %v", err)
			return
		}

		log.Printf("Executing command [%s]: %s", req.CommandID, req.Command)
		output, exitCode, err := ExecuteCommand(req.Command)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}

		resp := protocol.CommandResponse{
			CommandID: req.CommandID,
			Output:    output,
			ExitCode:  exitCode,
			Error:     errStr,
		}

		payload, _ := json.Marshal(resp)
		responseMsg := protocol.Message{
			Type:    protocol.TypeCommandResponse,
			Id:      req.CommandID,
			Payload: payload,
		}

		_ = writeJSONSafe(responseMsg)

	case protocol.TypeUninstallSignal:
		log.Println("Received uninstall signal from backend.")
		uninstallAgent()

	case protocol.TypeUpdateConfig:
		var req protocol.UpdateConfigRequest
		if err := json.Unmarshal(msg.Payload, &req); err == nil && req.NewBackendURL != "" {
			log.Printf("Received new backend URL: %s. Updating config...", req.NewBackendURL)
			config.BackendURL = req.NewBackendURL
			saveConfig()
			if conn != nil {
				conn.Close() // Force reconnect to new URL
			}
		}

	case protocol.TypeFileGetRequest:
		var req protocol.FileGetRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			log.Printf("Failed to parse file get request: %v", err)
			return
		}
		go handleFileGet(req)

	case protocol.TypeFilePutRequest:
		var req protocol.FilePutRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			log.Printf("Failed to parse file put request: %v", err)
			return
		}
		go handleFilePut(req)

	case protocol.TypeFilePutChunk:
		var chunk protocol.FilePutChunk
		if err := json.Unmarshal(msg.Payload, &chunk); err != nil {
			log.Printf("Failed to parse file put chunk: %v", err)
			return
		}
		putTransfersMu.Lock()
		ch, exists := putTransfers[chunk.TransferID]
		putTransfersMu.Unlock()
		if exists {
			ch <- &chunk
		}
	}
}

func uninstallAgent() {
	log.Println("Uninstalling agent...")
	_ = os.Remove(configPath)
	log.Println("Config file removed. Exiting.")
	os.Exit(0)
}

func writeJSONSafe(v interface{}) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}
	return conn.WriteJSON(v)
}
