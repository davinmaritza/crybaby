package main

import (
	"archive/zip"
	"bytes"
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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davinmaritza/crybaby/pkg/auth"
	"github.com/davinmaritza/crybaby/pkg/cluster"
	"github.com/davinmaritza/crybaby/pkg/iso"
	"github.com/davinmaritza/crybaby/pkg/mesh"
	"github.com/davinmaritza/crybaby/pkg/types"
	"github.com/gorilla/websocket"
)

const ControllerVersion = "0.6.0"

type Server struct {
	registry     *cluster.Registry
	auth         *auth.Authenticator
	hub          *Hub
	meshKey      *mesh.KeyPair
	isoMgr       *iso.Manager
	history      *cluster.HistoryBuffer
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

	history := cluster.NewHistoryBuffer(30)

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
			var ramPct float64
			if summary.TotalRAMBytes > 0 {
				ramPct = (float64(summary.UsedRAMBytes) / float64(summary.TotalRAMBytes)) * 100.0
			}
			history.Add(summary.AvgCPUUsagePct, ramPct)
			hub.BroadcastJSON(map[string]interface{}{
				"type":    "telemetry_tick",
				"summary": summary,
				"nodes":   nodes,
				"history": history.GetPoints(),
			})
		}
	}()

	isoMgr := iso.NewManager("./iso")

	srv := &Server{
		registry:     registry,
		auth:         authenticator,
		hub:          hub,
		meshKey:      meshKey,
		isoMgr:       isoMgr,
		history:      history,
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
	mux.HandleFunc("/api/v1/cluster/history", s.handleClusterHistory)
	mux.HandleFunc("/api/v1/nodes", s.handleNodes)
	mux.HandleFunc("/api/v1/nodes/register", s.handleRegisterNode)
	mux.HandleFunc("/api/v1/nodes/", s.handleNodeSubroutes)
	mux.HandleFunc("/api/v1/provision/usb-profile", s.handleUSBProfile)
	mux.HandleFunc("/api/v1/provision/usb-zip", s.handleUSBZip)
	mux.HandleFunc("/api/v1/cluster/reset", s.handleClusterReset)
	mux.HandleFunc("/api/v1/ws/telemetry", s.handleWebSocketTelemetry)
	mux.HandleFunc("/api/v1/ws/terminal/", s.handleWebSocketTerminalProxy)
	mux.HandleFunc("/install", s.handleWebInstallerPS1)
	mux.HandleFunc("/install.ps1", s.handleWebInstallerPS1)
	mux.HandleFunc("/install.sh", s.handleWebInstallerSH)
	mux.HandleFunc("/api/v1/binaries/crybaby-agent.exe", s.handleServeAgentBinary)
	mux.HandleFunc("/api/v1/binaries/crybaby-agent-linux", s.handleServeLinuxAgentBinary)
	mux.HandleFunc("/api/v1/iso", s.handleListISOs)
	mux.HandleFunc("/api/v1/iso/download", s.handleDownloadISO)
	mux.HandleFunc("/api/v1/iso/downloads", s.handleListISODownloads)

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="48" fill="#6366F1"/><path d="M50 20 L35 55 L50 55 L45 80 L65 45 L50 45 Z" fill="#FFFFFF"/></svg>`))
	})

	// ISO repository static file server for LAN distribution
	_ = os.MkdirAll("./iso", 0755)
	mux.Handle("/iso/", http.StripPrefix("/iso/", http.FileServer(http.Dir("./iso"))))

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

func (s *Server) handleClusterHistory(w http.ResponseWriter, r *http.Request) {
	points := s.history.GetPoints()
	jsonResponse(w, http.StatusOK, points)
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
		if err := s.registry.UpdateHeartbeat(nodeID, payload.Metrics, payload.Version); err != nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "recorded"})

	case "update":
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		node, found := s.registry.GetNode(nodeID)
		if !found {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Node not found"})
			return
		}
		if node.Status == types.NodeStatusOffline {
			jsonResponse(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("Node '%s' saat ini OFFLINE dan tidak dapat di-update.", node.Name)})
			return
		}

		binPath := filepath.Join(".", "bin", "crybaby-agent.exe")
		if _, err := os.Stat(binPath); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{
				"error": "Biner agent 'bin/crybaby-agent.exe' belum terkompilasi di controller. Jalankan 'GOOS=windows GOARCH=amd64 go build -o bin/crybaby-agent.exe ./cmd/agent' terlebih dahulu.",
			})
			return
		}

		lanIP := getOutboundIP()
		if lanIP == "" || lanIP == "127.0.0.1" {
			lanIP = "localhost"
		}
		downloadURL := fmt.Sprintf("http://%s:%s/api/v1/binaries/crybaby-agent.exe", lanIP, s.port)

		updateReq := types.AgentUpdateRequest{
			DownloadURL:   downloadURL,
			TargetVersion: ControllerVersion,
		}
		reqBytes, _ := json.Marshal(updateReq)

		targetHost := node.IPAddress
		if targetHost == "" {
			targetHost = node.MeshIP
		}
		agentURL := fmt.Sprintf("http://%s:%d/api/v1/agent/update", targetHost, node.AgentPort)

		resp, err := http.Post(agentURL, "application/json", bytes.NewBuffer(reqBytes))
		if err != nil {
			jsonResponse(w, http.StatusBadGateway, map[string]string{
				"error": fmt.Sprintf("Gagal menghubungi agent pada %s: %v", agentURL, err),
			})
			return
		}
		defer resp.Body.Close()

		var agentResp types.AgentUpdateResponse
		if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
			jsonResponse(w, resp.StatusCode, map[string]string{
				"status":  "triggered",
				"message": "Pembaruan agent dikirim, agent sedang memperbarui diri...",
			})
			return
		}

		jsonResponse(w, resp.StatusCode, agentResp)

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

func (s *Server) resolveControllerHost(r *http.Request) string {
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
	return host
}

func (s *Server) handleUSBProfile(w http.ResponseWriter, r *http.Request) {
	host := s.resolveControllerHost(r)
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

func findBinaryPath(name string) (string, os.FileInfo, error) {
	candidates := []string{
		filepath.Join(".", "bin", name),
		filepath.Join("..", "..", "bin", name),
		filepath.Join("..", "bin", name),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, info, nil
		}
	}
	return "", nil, os.ErrNotExist
}

func findFileCandidate(candidates ...string) (string, []byte, error) {
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return p, data, nil
		}
	}
	return "", nil, os.ErrNotExist
}

func (s *Server) handleServeAgentBinary(w http.ResponseWriter, r *http.Request) {
	binPath, info, err := findBinaryPath("crybaby-agent.exe")
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{
			"error": "Agent binary not found in bin/crybaby-agent.exe. Please build it first on controller via: GOOS=windows GOARCH=amd64 go build -o bin/crybaby-agent.exe ./cmd/agent",
		})
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="crybaby-agent.exe"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeFile(w, r, binPath)
}

func (s *Server) handleServeLinuxAgentBinary(w http.ResponseWriter, r *http.Request) {
	binPath, info, err := findBinaryPath("crybaby-agent-linux")
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{
			"error": "Linux agent binary not found in bin/crybaby-agent-linux. Please build it first on controller via: GOOS=linux GOARCH=amd64 go build -o bin/crybaby-agent-linux ./cmd/agent",
		})
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="crybaby-agent-linux"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeFile(w, r, binPath)
}

func (s *Server) handleWebInstallerPS1(w http.ResponseWriter, r *http.Request) {
	host := s.resolveControllerHost(r)
	controllerURL := fmt.Sprintf("http://%s", host)

	script := fmt.Sprintf(`# ============================================================
# CRYBABY CLUSTER - 1-CLICK SERVER NODE INSTALLER (POWERSHELL)
# ============================================================
$ErrorActionPreference = "Stop"

# 1. Self-Elevate to Administrator (UAC) if not already elevated
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[ELEVASI] Meminta izin Administrator (UAC)..." -ForegroundColor Yellow
    Start-Process powershell -ArgumentList '-NoProfile -ExecutionPolicy Bypass -Command "& { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; irm %s/install | iex }"' -Verb RunAs
    exit
}

Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "      CRYBABY CLUSTER - 1-CLICK NODE INSTALLER         " -ForegroundColor Cyan
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "Controller URL  : %s" -ForegroundColor Green
Write-Host "Target Directory: $env:ProgramFiles\Crybaby" -ForegroundColor Green
Write-Host ""

$TargetDir = "$env:ProgramFiles\Crybaby"
if (-not (Test-Path $TargetDir)) {
    New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
}

# 2. Download crybaby-agent.exe
Write-Host "[1/6] Mengunduh biner crybaby-agent.exe dari Controller..." -ForegroundColor Yellow
$AgentExePath = "$TargetDir\crybaby-agent.exe"
try {
    Invoke-WebRequest -Uri "%s/api/v1/binaries/crybaby-agent.exe" -OutFile $AgentExePath -UseBasicParsing
    Write-Host "      [OK] crybaby-agent.exe berhasil diunduh." -ForegroundColor Green
} catch {
    Write-Host "      [ERROR] Gagal mengunduh biner agent: $_" -ForegroundColor Red
    exit 1
}

# 3. Write crybaby-config.json
Write-Host "[2/6] Menulis konfigurasi node cluster..." -ForegroundColor Yellow
$ConfigJson = @"
{
  "controller_url": "%s",
  "cluster_token": "%s",
  "cluster_name": "Crybaby-Primary-Cluster",
  "created_at": "$(Get-Date -Format s)Z"
}
"@
Set-Content -Path "$TargetDir\crybaby-config.json" -Value $ConfigJson -Encoding UTF8
Write-Host "      [OK] crybaby-config.json tersimpan." -ForegroundColor Green

# 4. Windows Power Configuration (Anti-Sleep & Server Mode)
Write-Host "[3/6] Mengonfigurasi skema daya Windows (Anti-Sleep & Server Mode)..." -ForegroundColor Yellow
powercfg -change -standby-timeout-ac 0 2>$null
powercfg -change -hibernate-timeout-ac 0 2>$null
powercfg -change -monitor-timeout-ac 15 2>$null
powercfg -setacvalueindex SCHEME_CURRENT SUB_BUTTONS PBUTTONACTION 0 2>$null
powercfg -setactive SCHEME_CURRENT 2>$null
Write-Host "      [OK] Skema daya diatur ke Always-On Server Mode." -ForegroundColor Green

# 5. Windows Firewall & Network Profile
Write-Host "[4/6] Mengonfigurasi Windows Firewall & Profil Jaringan LAN..." -ForegroundColor Yellow
Set-NetConnectionProfile -NetworkCategory Private -ErrorAction SilentlyContinue 2>$null
netsh advfirewall firewall delete rule name="Crybaby Agent RPC" 2>$null
netsh advfirewall firewall add rule name="Crybaby Agent RPC" dir=in action=allow protocol=TCP localport=8081 profile=any description="Crybaby Cluster RPC & WebSocket" 2>$null
netsh advfirewall firewall delete rule name="Crybaby WireGuard Mesh" 2>$null
netsh advfirewall firewall add rule name="Crybaby WireGuard Mesh" dir=in action=allow protocol=UDP localport=51820 profile=any description="Crybaby P2P Mesh Fabric" 2>$null
netsh advfirewall firewall set rule name="File and Printer Sharing (Echo Request - ICMPv4-In)" new enable=yes 2>$null
Write-Host "      [OK] Port RPC 8081, WireGuard 51820, dan ICMP Ping dibuka." -ForegroundColor Green

# 6. Windows Update (Anti-Auto Reboot)
Write-Host "[5/6] Mengunci kebijakan Windows Update (Anti-Auto Reboot)..." -ForegroundColor Yellow
New-Item -Path "HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" -Force -ErrorAction SilentlyContinue | Out-Null
Set-ItemProperty -Path "HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" -Name "NoAutoRebootWithLoggedOnUsers" -Value 1 -Type DWord -Force 2>$null
Set-ItemProperty -Path "HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" -Name "AUOptions" -Value 3 -Type DWord -Force 2>$null
Set-ItemProperty -Path "HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" -Name "AlwaysAutoRebootAtScheduledTime" -Value 0 -Type DWord -Force 2>$null
Write-Host "      [OK] Windows Update auto-reboot dicegah." -ForegroundColor Green

# 7. Check & Enable Hyper-V
Write-Host "[6/6] Memeriksa Hyper-V dan mendaftarkan Windows Service..." -ForegroundColor Yellow
try {
    $edition = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").EditionID
    if ($edition -like "*Home*") {
        $packages = Get-ChildItem -Path "$env:SystemRoot\servicing\Packages\*Hyper-V*.mum" -ErrorAction SilentlyContinue
        if ($packages) {
            foreach ($pkg in $packages) { dism /online /norestart /add-package:"$($pkg.FullName)" 2>$null | Out-Null }
            dism /online /enable-feature /featurename:Microsoft-Hyper-V-All /norestart /all 2>$null | Out-Null
        }
    } else {
        $hv = Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -ErrorAction SilentlyContinue
        if ($hv -and $hv.State -ne "Enabled") {
            Write-Host "      Mengaktifkan Microsoft-Hyper-V..." -ForegroundColor Yellow
            Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All -NoRestart -ErrorAction SilentlyContinue | Out-Null
        }
    }
} catch {}

sc.exe stop CrybabyAgent 2>$null
sc.exe delete CrybabyAgent 2>$null
Start-Sleep -Seconds 1
$binCmd = '"' + $AgentExePath + '" -config "' + $TargetDir + '\crybaby-config.json"'
sc.exe create CrybabyAgent binPath= $binCmd start= auto DisplayName= "Crybaby Cluster Agent" 2>$null
sc.exe description CrybabyAgent "Crybaby Distributed Windows 11 Cluster & Hyper-V Manager Service" 2>$null
sc.exe failure CrybabyAgent reset= 60 actions= restart/5000/restart/10000/restart/30000 2>$null
sc.exe start CrybabyAgent 2>$null
Write-Host "      [OK] Windows Service CrybabyAgent terpasang dan aktif!" -ForegroundColor Green

Write-Host ""
Write-Host "=======================================================" -ForegroundColor Green
Write-Host "[SUCCESS] PC Server resmi bergabung ke Crybaby Cluster!" -ForegroundColor Green
Write-Host "Agent Service : Berjalan otomatis saat Windows booting" -ForegroundColor Green
Write-Host "Status Node   : Periksa di Controller Dashboard (%s)" -ForegroundColor Green
Write-Host "=======================================================" -ForegroundColor Green
Write-Host ""
`, controllerURL, controllerURL, controllerURL, controllerURL, s.clusterToken, controllerURL)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(script))
}

func (s *Server) handleWebInstallerSH(w http.ResponseWriter, r *http.Request) {
	host := s.resolveControllerHost(r)
	controllerURL := fmt.Sprintf("http://%s", host)

	script := fmt.Sprintf(`#!/usr/bin/env bash
# ============================================================
# CRYBABY CLUSTER - 1-CLICK LINUX NODE INSTALLER
# ============================================================
set -euo pipefail

if [ "$EUID" -ne 0 ]; then
    echo "[ERROR] Harap jalankan script ini sebagai root (sudo bash)." >&2
    exit 1
fi

echo "======================================================="
echo "      CRYBABY CLUSTER - 1-CLICK LINUX NODE INSTALLER   "
echo "======================================================="
echo "Controller Target: %s"
echo ""

mkdir -p /usr/local/bin /etc/crybaby /var/log/crybaby

echo "[1/4] Mengunduh biner crybaby-agent..."
curl -fsSL "%s/api/v1/binaries/crybaby-agent-linux" -o /usr/local/bin/crybaby-agent
chmod +x /usr/local/bin/crybaby-agent

echo "[2/4] Menulis konfigurasi cluster..."
cat <<'EOF' > /etc/crybaby/crybaby-config.json
{
  "controller_url": "%s",
  "cluster_token": "%s",
  "cluster_name": "Crybaby-Primary-Cluster"
}
EOF

echo "[3/4] Mendaftarkan systemd service..."
cat <<'EOF' > /etc/systemd/system/crybaby-agent.service
[Unit]
Description=Crybaby Cluster Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/crybaby-agent -config /etc/crybaby/crybaby-config.json
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable crybaby-agent
systemctl restart crybaby-agent

echo "[4/4] Verifikasi status..."
echo "======================================================="
echo "[SUCCESS] Linux Server resmi bergabung ke Crybaby Cluster!"
echo "Status service: systemctl status crybaby-agent"
echo "Dashboard     : %s"
echo "======================================================="
`, controllerURL, controllerURL, controllerURL, s.clusterToken, controllerURL)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(script))
}

func (s *Server) handleListISOs(w http.ResponseWriter, r *http.Request) {
	list, err := s.isoMgr.List()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, list)
}

func (s *Server) handleDownloadISO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	var req struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request JSON"})
		return
	}
	job, err := s.isoMgr.StartDownload(req.URL, req.Filename)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusAccepted, job)
}

func (s *Server) handleListISODownloads(w http.ResponseWriter, r *http.Request) {
	jobs := s.isoMgr.ListJobs()
	jsonResponse(w, http.StatusOK, jobs)
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
	host := s.resolveControllerHost(r)

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
	if _, batData, err := findFileCandidate("scripts/install.bat", "../../scripts/install.bat", "../scripts/install.bat"); err == nil {
		if fBat, err := zw.Create("install.bat"); err == nil {
			_, _ = fBat.Write(batData)
		}
	}

	// 3. scripts/enable-hyperv.ps1
	if _, psData, err := findFileCandidate("scripts/enable-hyperv.ps1", "../../scripts/enable-hyperv.ps1", "../scripts/enable-hyperv.ps1"); err == nil {
		if fPs, err := zw.Create("enable-hyperv.ps1"); err == nil {
			_, _ = fPs.Write(psData)
		}
	}

	// 4. bin/crybaby-agent.exe
	if _, exeData, err := findFileCandidate("bin/crybaby-agent.exe", "../../bin/crybaby-agent.exe", "../bin/crybaby-agent.exe"); err == nil {
		if fExe, err := zw.Create("crybaby-agent.exe"); err == nil {
			_, _ = fExe.Write(exeData)
		}
	}

	// 5. scripts/uninstall.bat
	if _, unData, err := findFileCandidate("scripts/uninstall.bat", "../../scripts/uninstall.bat", "../scripts/uninstall.bat"); err == nil {
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
