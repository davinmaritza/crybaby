package main

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/davinmaritza/crybaby/pkg/auth"
	"github.com/davinmaritza/crybaby/pkg/cluster"
	"github.com/davinmaritza/crybaby/pkg/mesh"
	"github.com/davinmaritza/crybaby/pkg/types"
	"github.com/gorilla/websocket"
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
	mux.HandleFunc("/api/v1/provision/usb-zip", s.handleUSBZip)
	mux.HandleFunc("/api/v1/cluster/reset", s.handleClusterReset)
	mux.HandleFunc("/api/v1/ws/telemetry", s.handleWebSocketTelemetry)
	mux.HandleFunc("/api/v1/ws/terminal/", s.handleWebSocketTerminalProxy)

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="48" fill="#6366F1"/><path d="M50 20 L35 55 L50 55 L45 80 L65 45 L50 45 Z" fill="#FFFFFF"/></svg>`))
	})

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
		if r.Method == http.MethodDelete {
			if err := s.registry.RemoveNode(nodeID); err != nil {
				jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "Node removed successfully", "id": nodeID})
			return
		}
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		node, found := s.registry.GetNode(nodeID)
		if !found {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		jsonResponse(w, http.StatusOK, node)
		return
	}

	action := parts[1]
	switch action {
	case "heartbeat":
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		var payload types.HeartbeatPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid heartbeat JSON"})
			return
		}
		if err := s.registry.UpdateHeartbeat(nodeID, payload.Metrics); err != nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "recorded"})

	case "wol":
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		node, found := s.registry.GetNode(nodeID)
		if !found {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		if node.Hardware.MACAddress == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Node has no registered MAC address"})
			return
		}
		if err := cluster.SendWakeOnLAN(node.Hardware.MACAddress); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to send WoL packet: %v", err)})
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
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		subPath := strings.TrimPrefix(path, nodeID+"/")
		s.proxyToNode(node, subPath, w, r)

	case "switches":
		node, found := s.registry.GetNode(nodeID)
		if !found {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		s.proxyToNode(node, "switches", w, r)

	case "reboot":
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		node, found := s.registry.GetNode(nodeID)
		if !found {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		s.proxyToNode(node, "host/reboot", w, r)

	case "shutdown":
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		node, found := s.registry.GetNode(nodeID)
		if !found {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		s.proxyToNode(node, "host/shutdown", w, r)

	default:
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Unknown node subroute"})
	}
}

func (s *Server) proxyToNode(node *types.Node, subpath string, w http.ResponseWriter, r *http.Request) {
	if node.Status == types.NodeStatusOffline {
		jsonResponse(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("Node '%s' saat ini OFFLINE. Pastikan PC menyala dan crybaby-agent aktif.", node.Name),
		})
		return
	}

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
		jsonResponse(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Gagal membuat proxy request: %v", err),
		})
		return
	}

	// Copy headers
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		jsonResponse(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("Node agent tidak dapat dijangkau di %s (%v)", targetURL, err),
		})
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

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		if !localAddr.IP.IsLoopback() && localAddr.IP.To4() != nil {
			return localAddr.IP.String()
		}
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}

func (s *Server) handleUSBProfile(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = r.Host
	}
	if host == "" || strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		lanIP := getOutboundIP()
		if lanIP != "" {
			host = lanIP + ":" + s.port
		} else if host == "" {
			host = "127.0.0.1:" + s.port
		}
	}
	profile := types.USBProvisionProfile{
		ControllerURL: fmt.Sprintf("http://%s", host),
		ClusterToken:  s.clusterToken,
		ClusterName:   "Crybaby-Primary-Cluster",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"profile":        profile,
		"detected_host":  host,
		"controller_url": profile.ControllerURL,
	})
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

func (s *Server) handleUSBZip(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = r.Host
	}
	if host == "" || strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		lanIP := getOutboundIP()
		if lanIP != "" {
			host = lanIP + ":" + s.port
		} else if host == "" {
			host = "127.0.0.1:" + s.port
		}
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"crybaby-usb-pack.zip\"")

	zw := zip.NewWriter(w)
	defer zw.Close()

	// 1. crybaby-config.json
	cfg := types.USBProvisionProfile{
		ControllerURL: fmt.Sprintf("http://%s", host),
		ClusterToken:  s.clusterToken,
		ClusterName:   "Crybaby-Primary-Cluster",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	fConfig, err := zw.Create("crybaby-config.json")
	if err == nil {
		_, _ = fConfig.Write(cfgData)
	}

	// 2. scripts/install.bat
	if batData, err := os.ReadFile("scripts/install.bat"); err == nil {
		if fBat, err := zw.Create("install.bat"); err == nil {
			_, _ = fBat.Write(batData)
		}
	}

	// 3. scripts/enable-hyperv.ps1
	if psData, err := os.ReadFile("scripts/enable-hyperv.ps1"); err == nil {
		if fPs, err := zw.Create("enable-hyperv.ps1"); err == nil {
			_, _ = fPs.Write(psData)
		}
	}

	// 4. bin/crybaby-agent.exe
	if exeData, err := os.ReadFile("bin/crybaby-agent.exe"); err == nil {
		if fExe, err := zw.Create("crybaby-agent.exe"); err == nil {
			_, _ = fExe.Write(exeData)
		}
	}

	// 5. scripts/uninstall.bat
	if unData, err := os.ReadFile("scripts/uninstall.bat"); err == nil {
		if fUn, err := zw.Create("uninstall.bat"); err == nil {
			_, _ = fUn.Write(unData)
		}
	}
}

func (s *Server) handleClusterReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	if err := s.registry.ResetCluster(); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Reset cluster failed: %v", err)})
		return
	}

	// Broadcast empty cluster state to connected dashboards
	s.hub.BroadcastJSON(map[string]interface{}{
		"type":    "telemetry_tick",
		"summary": s.registry.GetClusterSummary(),
		"nodes":   s.registry.ListNodes(),
	})

	jsonResponse(w, http.StatusOK, map[string]string{"status": "Cluster reset successfully. All nodes cleared."})
}

func (s *Server) handleWebSocketTerminalProxy(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/ws/terminal/")
	node, found := s.registry.GetNode(nodeID)
	if !found {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	browserConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Browser terminal WS upgrade failed: %v", err)
		return
	}
	defer browserConn.Close()

	port := node.AgentPort
	if port == 0 {
		port = 8081
	}

	targetWS := fmt.Sprintf("ws://%s:%d/api/v1/ws/terminal", node.IPAddress, port)
	agentConn, _, err := websocket.DefaultDialer.Dial(targetWS, nil)
	if err != nil {
		log.Printf("Failed to dial agent terminal WS at %s: %v", targetWS, err)
		_ = browserConn.WriteJSON(types.TerminalMessage{
			Type: types.TerminalMsgError,
			Data: fmt.Sprintf("Node agent unreachable at %s: %v", targetWS, err),
		})
		return
	}
	defer agentConn.Close()

	errChan := make(chan error, 2)

	// Forward browser -> agent
	go func() {
		for {
			msgType, data, err := browserConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := agentConn.WriteMessage(msgType, data); err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Forward agent -> browser
	go func() {
		for {
			msgType, data, err := agentConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := browserConn.WriteMessage(msgType, data); err != nil {
				errChan <- err
				return
			}
		}
	}()

	<-errChan
}
