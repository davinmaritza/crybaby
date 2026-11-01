package cluster

import (
	"testing"
	"time"

	"github.com/davinmaritza/crybaby/pkg/types"
)

func TestNodeRegistrationAndHeartbeat(t *testing.T) {
	token := "testClusterToken123"
	reg := NewRegistry(token, "")

	req := types.RegisterNodeRequest{
		ClusterToken: token,
		NodeName:     "node-desktop-win11",
		Hardware: types.NodeHardware{
			Hostname:      "WIN11-LAB-01",
			CPUCores:      8,
			TotalRAMBytes: 32 * 1024 * 1024 * 1024,
			MACAddress:    "00:1A:2B:3C:4D:5E",
			HyperVEnabled: true,
		},
		WireGuardPubKey: "testPubKey==",
	}

	node, err := reg.RegisterNode(req, "192.168.1.50")
	if err != nil {
		t.Fatalf("unexpected registration failure: %v", err)
	}

	if node.MeshIP != "10.42.0.2" {
		t.Errorf("expected first mesh IP 10.42.0.2, got '%s'", node.MeshIP)
	}

	if node.Status != types.NodeStatusOnline {
		t.Errorf("expected node status ONLINE, got '%s'", node.Status)
	}

	// Test heartbeat update
	metrics := types.NodeMetrics{
		CPUUsagePct:   12.5,
		RAMUsedBytes:  4 * 1024 * 1024 * 1024,
		ActiveVMCount: 2,
	}
	if err := reg.UpdateHeartbeat(node.ID, metrics); err != nil {
		t.Fatalf("heartbeat update failed: %v", err)
	}

	summary := reg.GetClusterSummary()
	if summary.TotalNodes != 1 || summary.OnlineNodes != 1 {
		t.Errorf("expected 1 total & online node in summary, got total=%d online=%d", summary.TotalNodes, summary.OnlineNodes)
	}

	if summary.TotalActiveVMs != 2 {
		t.Errorf("expected 2 active VMs, got %d", summary.TotalActiveVMs)
	}

	// Test duplicate registration with same MAC address returns existing node
	duplicateReq := req
	duplicateReq.NodeName = "renamed-node"
	dupNode, err := reg.RegisterNode(duplicateReq, "192.168.1.51")
	if err != nil {
		t.Fatalf("duplicate registration failed: %v", err)
	}
	if dupNode.ID != node.ID {
		t.Errorf("expected duplicate node to reuse ID '%s', got '%s'", node.ID, dupNode.ID)
	}
}

func TestNodeReaper(t *testing.T) {
	token := "testClusterToken"
	reg := NewRegistry(token, "")

	req := types.RegisterNodeRequest{
		ClusterToken: token,
		NodeName:     "stale-node",
		Hardware: types.NodeHardware{
			MACAddress: "AA:BB:CC:DD:EE:FF",
		},
	}
	node, err := reg.RegisterNode(req, "192.168.1.99")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Wait briefly and reap with 1ms timeout to simulate timeout
	time.Sleep(5 * time.Millisecond)
	reg.ReapOfflineNodes(2 * time.Millisecond)

	updatedNode, _ := reg.GetNode(node.ID)
	if updatedNode.Status != types.NodeStatusOffline {
		t.Errorf("expected node to be reaped as OFFLINE, got '%s'", updatedNode.Status)
	}
}
