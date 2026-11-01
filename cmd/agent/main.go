package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davinmaritza/crybaby/pkg/mesh"
	"github.com/davinmaritza/crybaby/pkg/types"
)

type Config struct {
	ControllerURL string `json:"controller_url"`
	ClusterToken  string `json:"cluster_token"`
	NodeName      string `json:"node_name"`
}

func main() {
	controllerFlag := flag.String("controller", "http://127.0.0.1:8080", "Crybaby Controller URL")
	tokenFlag := flag.String("token", "", "Cluster join token")
	nameFlag := flag.String("name", "", "Custom node name (defaults to hostname)")
	configFlag := flag.String("config", "crybaby-config.json", "Path to config file")
	flag.Parse()

	cfg := Config{
		ControllerURL: *controllerFlag,
		ClusterToken:  *tokenFlag,
		NodeName:      *nameFlag,
	}

	// Try reading config file if present
	if data, err := os.ReadFile(*configFlag); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	hw := DetectHardware()
	if cfg.NodeName == "" {
		cfg.NodeName = hw.Hostname
		if cfg.NodeName == "" {
			cfg.NodeName = "win11-node"
		}
	}

	log.Printf("======================================================")
	log.Printf("⚡ Crybaby Node Agent starting...")
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
			if err := sendHeartbeat(cfg.ControllerURL, regResp.NodeID, metrics); err != nil {
				log.Printf("⚠️ Heartbeat failed: %v", err)
			}
		}
	}
}

func registerWithController(cfg Config, hw types.NodeHardware, pubKey string) (*types.RegisterNodeResponse, error) {
	reqBody := types.RegisterNodeRequest{
		ClusterToken:   cfg.ClusterToken,
		NodeName:       cfg.NodeName,
		Hardware:       hw,
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
