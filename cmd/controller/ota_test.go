package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/davinmaritza/crybaby/pkg/auth"
	"github.com/davinmaritza/crybaby/pkg/cluster"
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
	binPath := filepath.Join(".", "bin", "crybaby-agent.exe")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
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
