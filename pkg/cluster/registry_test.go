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
		Version:      "0.4.5",
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

	if node.Version != "0.4.5" {
		t.Errorf("expected version 0.4.5, got '%s'", node.Version)
	}

	if node.MeshIP != "10.42.0.2" {
		t.Errorf("expected first mesh IP 10.42.0.2, got '%s'", node.MeshIP)
	}

	if node.Status != types.NodeStatusOnline {
		t.Errorf("expected node status ONLINE, got '%s'", node.Status)
	}

	// Test heartbeat update with new version
	metrics := types.NodeMetrics{
		CPUUsagePct:   12.5,
		RAMUsedBytes:  4 * 1024 * 1024 * 1024,
		ActiveVMCount: 2,
	}
	if err := reg.UpdateHeartbeat(node.ID, metrics, "0.5.0"); err != nil {
		t.Fatalf("heartbeat update failed: %v", err)
	}

	updatedNode, _ := reg.GetNode(node.ID)
	if updatedNode.Version != "0.5.0" {
		t.Errorf("expected updated version 0.5.0, got '%s'", updatedNode.Version)
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

	// Test RemoveNode
	if err := reg.RemoveNode(node.ID); err != nil {
		t.Fatalf("failed to remove node: %v", err)
	}
	if _, found := reg.GetNode(node.ID); found {
		t.Errorf("node %s should not exist after RemoveNode", node.ID)
	}
	if err := reg.RemoveNode("non-existent-id"); err == nil {
		t.Errorf("expected error removing non-existent node")
	}

	// Test ResetCluster
	_, _ = reg.RegisterNode(req, "192.168.1.100")
	if len(reg.ListNodes()) == 0 {
		t.Fatalf("expected node to be re-registered")
	}
	if err := reg.ResetCluster(); err != nil {
		t.Fatalf("ResetCluster failed: %v", err)
	}
	if len(reg.ListNodes()) != 0 {
		t.Errorf("expected 0 nodes after ResetCluster, got %d", len(reg.ListNodes()))
	}
}
