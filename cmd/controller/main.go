package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/davinmaritza/crybaby/pkg/auth"
	"github.com/davinmaritza/crybaby/pkg/cluster"
	"github.com/davinmaritza/crybaby/pkg/mesh"
	"github.com/davinmaritza/crybaby/pkg/types"
)

type Server struct {
	registry     *cluster.Registry
	auth         *auth.Authenticator
	hub          *Hub
	meshKey      *mesh.KeyPair
	port         string
	clusterToken string
}

func main() {
	port := flag.String("port", "8080", "HTTP & WebSocket listening port")
	masterPassword := flag.String("password", "crybaby123", "Dashboard master password")
	clusterToken := flag.String("token", "", "Cluster join token (auto-generated if empty)")
	storagePath := flag.String("storage", "crybaby-cluster.json", "Persistent JSON cluster storage file")
	flag.Parse()

	if envPass := os.Getenv("CRYBABY_PASSWORD"); envPass != "" {
		*masterPassword = envPass
	}
	if envToken := os.Getenv("CRYBABY_TOKEN"); envToken != "" {
		*clusterToken = envToken
	}

	if *clusterToken == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		*clusterToken = hex.EncodeToString(b)
	}

	meshKey, err := mesh.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate WireGuard controller keys: %v", err)
	}

	signingSecret := hex.EncodeToString([]byte(*clusterToken + *masterPassword))
	authenticator := auth.NewAuthenticator(*masterPassword, signingSecret)
	registry := cluster.NewRegistry(*clusterToken, *storagePath)
	hub := NewHub()

	go hub.Run()

	// Background node reaper (every 5 seconds, timeout at 15s)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			registry.ReapOfflineNodes(15 * time.Second)
		}
	}()

	// Periodic telemetry broadcast loop (every 2 seconds)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			summary := registry.GetClusterSummary()
			nodes := registry.ListNodes()
			hub.BroadcastJSON(map[string]interface{}{
				"type":    "telemetry_tick",
				"summary": summary,
				"nodes":   nodes,
			})
		}
	}()

	srv := &Server{
		registry:     registry,
		auth:         authenticator,
		hub:          hub,
		meshKey:      meshKey,
		port:         *port,
		clusterToken: *clusterToken,
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	log.Printf("======================================================")
	log.Printf("🚀 Crybaby Controller is active on http://0.0.0.0:%s", *port)
	log.Printf("🔑 Master Password: %s", *masterPassword)
	log.Printf("🛡️  Cluster Join Token: %s", *clusterToken)
	log.Printf("🌐 WireGuard Controller Mesh IP: 10.42.0.1/16")
	log.Printf("======================================================")

	if err := http.ListenAndServe(":"+*port, mux); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/cluster/status", s.handleClusterStatus)
	mux.HandleFunc("/api/v1/nodes", s.handleNodes)
	mux.HandleFunc("/api/v1/nodes/register", s.handleRegisterNode)
	mux.HandleFunc("/api/v1/nodes/", s.handleNodeSubroutes)
	mux.HandleFunc("/api/v1/provision/usb-profile", s.handleUSBProfile)
	mux.HandleFunc("/api/v1/ws/telemetry", s.handleWebSocketTelemetry)

	// Embedded web dashboard
	mux.HandleFunc("/", s.handleWebDashboard)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"app":     "crybaby-controller",
		"version": "0.1.0-dev",
		"time":    time.Now().UTC(),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if !s.auth.VerifyPassword(req.Password) {
		http.Error(w, "Invalid password credentials", http.StatusUnauthorized)
		return
	}

	token, exp, err := s.auth.IssueToken(24 * time.Hour)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, http.StatusOK, types.AuthResponse{
		Token:     token,
		ExpiresAt: exp,
	})
}

func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	summary := s.registry.GetClusterSummary()
	jsonResponse(w, http.StatusOK, summary)
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.registry.ListNodes()
	jsonResponse(w, http.StatusOK, nodes)
}

func (s *Server) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.RegisterNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	remoteIP := r.RemoteAddr
	if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
		remoteIP = remoteIP[:idx]
	}

	node, err := s.registry.RegisterNode(req, remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	wgConfig := mesh.NodeWireGuardConfig(
		"<NODE_PRIVATE_KEY>",
		node.MeshIP,
		s.meshKey.PublicKey,
		fmt.Sprintf("%s:%s", remoteIP, s.port),
		"10.42.0.1",
	)

	jsonResponse(w, http.StatusOK, types.RegisterNodeResponse{
		NodeID:           node.ID,
		MeshIP:           node.MeshIP,
		ControllerMeshIP: "10.42.0.1",
		WireGuardConfig:  wgConfig,
		HeartbeatSec:     5,
	})
}

func (s *Server) handleNodeSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/")
	parts := strings.Split(path, "/")
	nodeID := parts[0]

	if len(parts) == 1 {
		node, found := s.registry.GetNode(nodeID)
		if !found {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, http.StatusOK, node)
		return
	}

	action := parts[1]
	switch action {
	case "heartbeat":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload types.HeartbeatPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid heartbeat JSON", http.StatusBadRequest)
			return
		}
		if err := s.registry.UpdateHeartbeat(nodeID, payload.Metrics); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "recorded"})

	case "wol":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		node, found := s.registry.GetNode(nodeID)
		if !found {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		if node.Hardware.MACAddress == "" {
			http.Error(w, "Node has no registered MAC address", http.StatusBadRequest)
			return
		}
		if err := cluster.SendWakeOnLAN(node.Hardware.MACAddress); err != nil {
			http.Error(w, fmt.Sprintf("Failed to send WoL packet: %v", err), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"status":  "magic_packet_sent",
			"mac":     node.Hardware.MACAddress,
			"node_id": nodeID,
		})

	case "vms":
		node, found := s.registry.GetNode(nodeID)
		if !found {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		subPath := strings.TrimPrefix(path, nodeID+"/")
		s.proxyToNode(node, subPath, w, r)

	case "switches":
		node, found := s.registry.GetNode(nodeID)
		if !found {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		s.proxyToNode(node, "switches", w, r)

	default:
		http.Error(w, "Unknown node subroute", http.StatusNotFound)
	}
}

func (s *Server) proxyToNode(node *types.Node, subpath string, w http.ResponseWriter, r *http.Request) {
	port := node.AgentPort
	if port == 0 {
		port = 8081
	}

	targetURL := fmt.Sprintf("http://%s:%d/api/v1/%s", node.IPAddress, port, subpath)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to construct proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Node agent unreachable at %s: %v", targetURL, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Server) handleUSBProfile(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = "127.0.0.1:" + s.port
	}
	profile := types.USBProvisionProfile{
		ControllerURL: fmt.Sprintf("http://%s", host),
		ClusterToken:  s.clusterToken,
		ClusterName:   "Crybaby-Primary-Cluster",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	jsonResponse(w, http.StatusOK, profile)
}

func (s *Server) handleWebSocketTelemetry(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	s.hub.register <- conn

	// Send initial cluster state immediately
	_ = conn.WriteJSON(map[string]interface{}{
		"type":    "initial_state",
		"summary": s.registry.GetClusterSummary(),
		"nodes":   s.registry.ListNodes(),
	})
}

func jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
