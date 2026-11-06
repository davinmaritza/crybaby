package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davinmaritza/crybaby/pkg/auth"
	"github.com/davinmaritza/crybaby/pkg/cluster"
	"github.com/davinmaritza/crybaby/pkg/iso"
	"github.com/davinmaritza/crybaby/pkg/types"
)

func TestServeAgentBinary(t *testing.T) {
	reg := cluster.NewRegistry("token123", "")
	authInstance := auth.NewAuthenticator("pass123", "secret123")
	s := &Server{
		registry: reg,
		auth:     authInstance,
		port:     "8080",
	}

	// 1. Test when binary doesn't exist
	req := httptest.NewRequest(http.MethodGet, "/api/v1/binaries/crybaby-agent.exe", nil)
	rr := httptest.NewRecorder()
	s.handleServeAgentBinary(rr, req)

	// In test environment, bin/crybaby-agent.exe may or may not exist
	_, _, err := findBinaryPath("crybaby-agent.exe")
	if err != nil {
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 when binary missing, got %d", rr.Code)
		}
	} else {
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 when binary exists, got %d", rr.Code)
		}
	}
}

func TestUpdateNodeOfflineValidation(t *testing.T) {
	reg := cluster.NewRegistry("token123", "")
	authInstance := auth.NewAuthenticator("pass123", "secret123")
	s := &Server{
		registry: reg,
		auth:     authInstance,
		port:     "8080",
	}

	node, err := reg.RegisterNode(types.RegisterNodeRequest{
		ClusterToken: "token123",
		NodeName:     "offline-node",
		Version:      "0.4.0",
		Hardware: types.NodeHardware{
			MACAddress: "00:11:22:33:44:55",
			AgentPort:  8081,
		},
	}, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register node: %v", err)
	}

	// Set node offline
	node.Status = types.NodeStatusOffline

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes/"+node.ID+"/update", nil)
	rr := httptest.NewRecorder()
	s.handleNodeSubroutes(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway for offline node, got %d", rr.Code)
	}

	var errResp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp["error"] == "" {
		t.Errorf("expected error message in response")
	}
}

func TestISOControllerEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.iso"), []byte("sample-iso-bytes"), 0644)

	reg := cluster.NewRegistry("token123", "")
	authInstance := auth.NewAuthenticator("pass123", "secret123")
	s := &Server{
		registry: reg,
		auth:     authInstance,
		isoMgr:   iso.NewManager(tmpDir),
		port:     "8080",
	}

	// 1. Test handleListISOs
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iso", nil)
	rr := httptest.NewRecorder()
	s.handleListISOs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleListISOs, got %d", rr.Code)
	}

	var list []iso.ISOFile
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode ISO list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "test.iso" {
		t.Errorf("expected 1 ISO file named test.iso, got %+v", list)
	}
}

func TestClusterHistoryEndpoint(t *testing.T) {
	hist := cluster.NewHistoryBuffer(10)
	hist.Add(12.5, 48.0)
	hist.Add(15.0, 52.0)

	s := &Server{
		history: hist,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/history", nil)
	rr := httptest.NewRecorder()
	s.handleClusterHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleClusterHistory, got %d", rr.Code)
	}

	var points []types.TelemetryPoint
	if err := json.NewDecoder(rr.Body).Decode(&points); err != nil {
		t.Fatalf("failed to decode history response: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].CPU != 12.5 || points[1].CPU != 15.0 {
		t.Errorf("unexpected points data: %+v", points)
	}
}

func TestWebInstallerEndpoints(t *testing.T) {
	reg := cluster.NewRegistry("cluster-token-xyz", "")
	authInstance := auth.NewAuthenticator("pass123", "secret123")
	s := &Server{
		registry:     reg,
		auth:         authInstance,
		clusterToken: "cluster-token-xyz",
		port:         "8080",
	}

	// 1. Test GET /install (PowerShell script)
	reqPS1 := httptest.NewRequest(http.MethodGet, "/install?host=192.168.1.50:8080", nil)
	rrPS1 := httptest.NewRecorder()
	s.handleWebInstallerPS1(rrPS1, reqPS1)

	if rrPS1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /install, got %d", rrPS1.Code)
	}
	bodyPS1 := rrPS1.Body.String()
	if !strings.Contains(bodyPS1, "192.168.1.50:8080") {
		t.Errorf("expected controller host in ps1 output")
	}
	if !strings.Contains(bodyPS1, "cluster-token-xyz") {
		t.Errorf("expected cluster token in ps1 output")
	}
	if !strings.Contains(bodyPS1, "CrybabyAgent") || !strings.Contains(bodyPS1, "powercfg") {
		t.Errorf("expected service and power configuration commands in ps1 output")
	}

	// 2. Test GET /install.sh (Linux bash script)
	reqSH := httptest.NewRequest(http.MethodGet, "/install.sh?host=192.168.1.50:8080", nil)
	rrSH := httptest.NewRecorder()
	s.handleWebInstallerSH(rrSH, reqSH)

	if rrSH.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /install.sh, got %d", rrSH.Code)
	}
	bodySH := rrSH.Body.String()
	if !strings.Contains(bodySH, "192.168.1.50:8080") {
		t.Errorf("expected controller host in sh output")
	}
	if !strings.Contains(bodySH, "cluster-token-xyz") {
		t.Errorf("expected cluster token in sh output")
	}
	if !strings.Contains(bodySH, "systemctl restart crybaby-agent") {
		t.Errorf("expected systemctl commands in sh output")
	}

	// 3. Test GET /api/v1/binaries/crybaby-agent-linux
	reqLinuxBin := httptest.NewRequest(http.MethodGet, "/api/v1/binaries/crybaby-agent-linux", nil)
	rrLinuxBin := httptest.NewRecorder()
	s.handleServeLinuxAgentBinary(rrLinuxBin, reqLinuxBin)

	_, _, err := findBinaryPath("crybaby-agent-linux")
	if err != nil {
		if rrLinuxBin.Code != http.StatusNotFound {
			t.Errorf("expected 404 when linux binary missing, got %d", rrLinuxBin.Code)
		}
	} else {
		if rrLinuxBin.Code != http.StatusOK {
			t.Errorf("expected 200 when linux binary exists, got %d", rrLinuxBin.Code)
		}
	}

	// 4. Test GET /api/v1/provision/usb-zip
	reqZip := httptest.NewRequest(http.MethodGet, "/api/v1/provision/usb-zip?host=192.168.1.50:8080", nil)
	rrZip := httptest.NewRecorder()
	s.handleUSBZip(rrZip, reqZip)

	if rrZip.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUSBZip, got %d", rrZip.Code)
	}
	if rrZip.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("expected application/zip header, got %s", rrZip.Header().Get("Content-Type"))
	}

	zipReader, err := zip.NewReader(bytes.NewReader(rrZip.Body.Bytes()), int64(rrZip.Body.Len()))
	if err != nil {
		t.Fatalf("failed to parse usb pack zip: %v", err)
	}
	expectedFiles := map[string]bool{
		"crybaby-config.json": false,
		"install.bat":         false,
		"uninstall.bat":       false,
		"enable-hyperv.ps1":   false,
	}
	for _, f := range zipReader.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}
	for fName, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %s in usb-pack.zip", fName)
		}
	}
}


