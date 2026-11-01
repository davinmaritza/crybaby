package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/davinmaritza/crybaby/pkg/hyperv"
	"github.com/davinmaritza/crybaby/pkg/mesh"
	"github.com/davinmaritza/crybaby/pkg/terminal"
	"github.com/davinmaritza/crybaby/pkg/types"
	"github.com/davinmaritza/crybaby/pkg/updater"
	"github.com/gorilla/websocket"
)

const AgentVersion = "0.6.0"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Config struct {
	ControllerURL string `json:"controller_url"`
	ClusterToken  string `json:"cluster_token"`
	NodeName      string `json:"node_name"`
	AgentPort     int    `json:"agent_port"`
}

func main() {
	controllerFlag := flag.String("controller", "http://127.0.0.1:8080", "Crybaby Controller URL")
	tokenFlag := flag.String("token", "", "Cluster join token")
	nameFlag := flag.String("name", "", "Custom node name (defaults to hostname)")
	portFlag := flag.Int("port", 8081, "Local Agent HTTP RPC port")
	configFlag := flag.String("config", "crybaby-config.json", "Path to config file")
	flag.Parse()

	cfg := Config{
		ControllerURL: *controllerFlag,
		ClusterToken:  *tokenFlag,
		NodeName:      *nameFlag,
		AgentPort:     *portFlag,
	}

	// Try reading config file if present
	if data, err := os.ReadFile(*configFlag); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	hw := DetectHardware(cfg.AgentPort)
	if cfg.NodeName == "" {
		cfg.NodeName = hw.Hostname
		if cfg.NodeName == "" {
			cfg.NodeName = "win11-node"
		}
	}

	log.Printf("======================================================")
	log.Printf("⚡ Crybaby Node Agent starting on port %d...", cfg.AgentPort)
	log.Printf("🖥️  Host: %s (%s)", cfg.NodeName, hw.OS)
	log.Printf("🔌 Controller: %s", cfg.ControllerURL)
	log.Printf("======================================================")

	wgKey, err := mesh.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate local WireGuard keys: %v", err)
	}

	regResp, err := registerWithController(cfg, hw, wgKey.PublicKey)
	if err != nil {
		log.Fatalf("Registration failed: %v", err)
	}

	log.Printf("✅ Joined cluster successfully!")
	log.Printf("🆔 Node ID: %s", regResp.NodeID)
	log.Printf("🌐 Assigned Mesh IP: %s/16", regResp.MeshIP)

	// Initialize Hyper-V Engine
	hvEngine := hyperv.NewEngine(regResp.NodeID)

	// Self-healing: Ensure external switch 'Crybaby-LAN' exists in background
	go func() {
		if err := hvEngine.EnsureVirtualSwitch("Crybaby-LAN"); err != nil {
			log.Printf("⚠️ Notice: EnsureVirtualSwitch 'Crybaby-LAN': %v", err)
		} else {
			log.Printf("🔌 Verified External Virtual Switch 'Crybaby-LAN' is active")
		}
	}()

	// Start local Agent RPC server for Controller commands
	go startAgentRPCServer(cfg.AgentPort, hvEngine)

	// Heartbeat ticker
	ticker := time.NewTicker(time.Duration(regResp.HeartbeatSec) * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			log.Println("Shutting down Crybaby Node Agent...")
			return

		case <-ticker.C:
			metrics := CollectMetrics()
			if vms, err := hvEngine.ListVMs(); err == nil {
				active := 0
				for _, vm := range vms {
					if vm.State == types.VMStateRunning {
						active++
					}
				}
				metrics.ActiveVMCount = active
			}

			if err := sendHeartbeat(cfg.ControllerURL, regResp.NodeID, metrics); err != nil {
				log.Printf("⚠️ Heartbeat failed: %v", err)
			}
		}
	}
}

func startAgentRPCServer(port int, hv hyperv.Engine) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/vms", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			vms, err := hv.ListVMs()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusOK, vms)

		case http.MethodPost:
			var req types.CreateVMRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid VM request JSON", http.StatusBadRequest)
				return
			}
			vm, err := hv.CreateVM(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			jsonResponse(w, http.StatusCreated, vm)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/vms/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/vms/")
		parts := strings.Split(path, "/")
		vmID := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				vm, err := hv.GetVM(vmID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				jsonResponse(w, http.StatusOK, vm)
			case http.MethodDelete:
				if err := hv.DeleteVM(vmID, true); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		action := parts[1]
		switch action {
		case "start":
			if err := hv.StartVM(vmID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "started"})
		case "stop":
			if err := hv.StopVM(vmID, false); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
		case "restart":
			if err := hv.RestartVM(vmID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "restarted"})
		case "checkpoints":
			if len(parts) == 2 {
				// /api/v1/vms/:vm_id/checkpoints
				if r.Method == http.MethodGet {
					cps, err := hv.ListCheckpoints(vmID)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					jsonResponse(w, http.StatusOK, cps)
					return
				} else if r.Method == http.MethodPost {
					var req types.CreateCheckpointRequest
					_ = json.NewDecoder(r.Body).Decode(&req)
					cp, err := hv.CreateCheckpoint(vmID, req.Name)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					jsonResponse(w, http.StatusOK, cp)
					return
				}
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			subAction := parts[2]
			if subAction == "restore" && r.Method == http.MethodPost {
				var req types.CreateCheckpointRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				if err := hv.RestoreCheckpoint(vmID, req.Name); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				jsonResponse(w, http.StatusOK, map[string]string{"status": "restored"})
				return
			}
			if r.Method == http.MethodDelete {
				// /api/v1/vms/:vm_id/checkpoints/:name
				if err := hv.DeleteCheckpoint(vmID, subAction); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			}
			http.Error(w, "Unknown checkpoint action", http.StatusNotFound)
		case "resize":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req types.ResizeVMRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid resize request JSON", http.StatusBadRequest)
				return
			}
			if err := hv.ResizeVM(vmID, req.CPUCount, req.MemoryMB); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "resized"})
		case "expand-disk":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req types.ExpandDiskRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid expand disk request JSON", http.StatusBadRequest)
				return
			}
			if err := hv.ExpandDisk(vmID, req.NewSizeGB); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "expanded"})
		case "clone":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req types.CloneVMRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid clone request JSON", http.StatusBadRequest)
				return
			}
			clonedVM, err := hv.CloneVM(vmID, req.NewName)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonResponse(w, http.StatusCreated, clonedVM)
		default:
			http.Error(w, "Unknown VM action", http.StatusNotFound)
		}
	})

	mux.HandleFunc("/api/v1/switches", func(w http.ResponseWriter, r *http.Request) {
		switches, err := hv.ListVirtualSwitches()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, http.StatusOK, switches)
	})

	mux.HandleFunc("/api/v1/ws/terminal", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Agent terminal WebSocket upgrade failed: %v", err)
			return
		}
		session, err := terminal.NewSession(conn)
		if err != nil {
			log.Printf("Failed to spawn terminal session: %v", err)
			_ = conn.Close()
			return
		}
		defer session.Close()
		session.HandleIO()
	})

	mux.HandleFunc("/api/v1/host/reboot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "reboot_initiated"})
		go func() {
			time.Sleep(1 * time.Second)
			if runtime.GOOS == "windows" {
				_ = exec.Command("shutdown", "/r", "/t", "3", "/c", "Reboot initiated from Crybaby Controller").Run()
			} else {
				_ = exec.Command("reboot").Run()
			}
		}()
	})

	mux.HandleFunc("/api/v1/host/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "shutdown_initiated"})
		go func() {
			time.Sleep(1 * time.Second)
			if runtime.GOOS == "windows" {
				_ = exec.Command("shutdown", "/s", "/t", "3", "/c", "Shutdown initiated from Crybaby Controller").Run()
			} else {
				_ = exec.Command("shutdown", "-h", "now").Run()
			}
		}()
	})

	mux.HandleFunc("/api/v1/agent/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var updateReq types.AgentUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
			http.Error(w, "Invalid update request JSON", http.StatusBadRequest)
			return
		}

		if updateReq.DownloadURL == "" {
			http.Error(w, "DownloadURL cannot be empty", http.StatusBadRequest)
			return
		}

		log.Printf("🔄 Received OTA Update command: target version '%s' from '%s'", updateReq.TargetVersion, updateReq.DownloadURL)

		jsonResponse(w, http.StatusOK, types.AgentUpdateResponse{
			Status:  "updating",
			Message: fmt.Sprintf("Agent update to %s initiated, restarting service...", updateReq.TargetVersion),
		})

		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := updater.ApplyUpdate(updateReq.DownloadURL, updateReq.TargetVersion, ""); err != nil {
				log.Printf("❌ Failed to apply self-update: %v", err)
			}
		}()
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("🔌 Agent Hyper-V RPC listener active on %s", addr)
	_ = http.ListenAndServe(addr, mux)
}

func registerWithController(cfg Config, hw types.NodeHardware, pubKey string) (*types.RegisterNodeResponse, error) {
	reqBody := types.RegisterNodeRequest{
		ClusterToken:    cfg.ClusterToken,
		NodeName:        cfg.NodeName,
		Version:         AgentVersion,
		Hardware:        hw,
		WireGuardPubKey: pubKey,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/nodes/register", cfg.ControllerURL)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("network error connecting to controller: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("controller returned HTTP status %d", resp.StatusCode)
	}

	var regResp types.RegisterNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &regResp, nil
}

func sendHeartbeat(controllerURL, nodeID string, metrics types.NodeMetrics) error {
	payload, err := json.Marshal(types.HeartbeatPayload{
		NodeID:  nodeID,
		Version: AgentVersion,
		Metrics: metrics,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", controllerURL, nodeID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	return nil
}

func jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
