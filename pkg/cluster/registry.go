package cluster

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/davinmaritza/crybaby/pkg/types"
)

// Registry manages the in-memory state and persistence of all cluster nodes.
type Registry struct {
	mu            sync.RWMutex
	nodes         map[string]*types.Node
	clusterToken  string
	nextMeshOctet int
	storagePath   string
}

// NewRegistry initializes an empty cluster registry with persistent storage backing.
func NewRegistry(clusterToken, storagePath string) *Registry {
	r := &Registry{
		nodes:         make(map[string]*types.Node),
		clusterToken:  clusterToken,
		nextMeshOctet: 2, // 10.42.0.1 reserved for controller
		storagePath:   storagePath,
	}
	_ = r.LoadFromFile()
	return r
}

// ValidateClusterToken verifies if the onboarding token matches.
func (r *Registry) ValidateClusterToken(token string) bool {
	return r.clusterToken != "" && r.clusterToken == token
}

// RegisterNode adds or updates a node upon receiving its registration request.
func (r *Registry) RegisterNode(req types.RegisterNodeRequest, remoteIP string) (*types.Node, error) {
	if !r.ValidateClusterToken(req.ClusterToken) {
		return nil, errors.New("invalid or unauthorized cluster token")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if node exists by MAC address to prevent duplicates
	for _, existing := range r.nodes {
		if existing.Hardware.MACAddress == req.Hardware.MACAddress && req.Hardware.MACAddress != "" {
			existing.Status = types.NodeStatusOnline
			existing.LastSeen = time.Now()
			existing.Hardware = req.Hardware
			existing.IPAddress = remoteIP
			existing.WireGuardPubKey = req.WireGuardPubKey
			_ = r.saveToFileLocked()
			return existing, nil
		}
	}

	nodeID := generateRandomID(8)
	meshIP := fmt.Sprintf("10.42.0.%d", r.nextMeshOctet)
	r.nextMeshOctet++

	node := &types.Node{
		ID:             nodeID,
		Name:           req.NodeName,
		IPAddress:      remoteIP,
		MeshIP:         meshIP,
		Status:         types.NodeStatusOnline,
		Hardware:       req.Hardware,
		LastSeen:       time.Now(),
		RegisteredAt:   time.Now(),
		WireGuardPubKey: req.WireGuardPubKey,
	}

	r.nodes[nodeID] = node
	_ = r.saveToFileLocked()
	return node, nil
}

// UpdateHeartbeat records incoming telemetry and refreshes node liveness.
func (r *Registry) UpdateHeartbeat(nodeID string, metrics types.NodeMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.Status = types.NodeStatusOnline
	node.LastSeen = time.Now()
	node.LatestMetrics = metrics
	return nil
}

// GetNode returns a pointer copy of a specific node.
func (r *Registry) GetNode(id string) (*types.Node, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, exists := r.nodes[id]
	if !exists {
		return nil, false
	}
	cp := *node
	return &cp, true
}

// ListNodes returns all registered nodes in the cluster.
func (r *Registry) ListNodes() []*types.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*types.Node, 0, len(r.nodes))
	for _, n := range r.nodes {
		cp := *n
		list = append(list, &cp)
	}
	return list
}

// ReapOfflineNodes checks for timed-out nodes and marks them OFFLINE.
func (r *Registry) ReapOfflineNodes(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for _, node := range r.nodes {
		if node.Status == types.NodeStatusOnline && now.Sub(node.LastSeen) > timeout {
			node.Status = types.NodeStatusOffline
		}
	}
}

// GetClusterSummary computes aggregated metrics across all cluster nodes.
func (r *Registry) GetClusterSummary() types.ClusterSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summary := types.ClusterSummary{
		TotalNodes:      len(r.nodes),
		MeshNetworkCIDR: "10.42.0.0/16",
	}

	var totalCPUUsage float64
	var activeOnlineCPUs int

	for _, n := range r.nodes {
		if n.Status == types.NodeStatusOnline {
			summary.OnlineNodes++
			totalCPUUsage += n.LatestMetrics.CPUUsagePct
			activeOnlineCPUs++
			summary.UsedRAMBytes += n.LatestMetrics.RAMUsedBytes
			summary.UsedDiskBytes += n.LatestMetrics.DiskUsedBytes
			summary.TotalActiveVMs += n.LatestMetrics.ActiveVMCount
		} else {
			summary.OfflineNodes++
		}
		summary.TotalCPUCores += n.Hardware.CPUCores
		summary.TotalRAMBytes += n.Hardware.TotalRAMBytes
		summary.TotalDiskBytes += n.Hardware.TotalDiskBytes
	}

	if activeOnlineCPUs > 0 {
		summary.AvgCPUUsagePct = totalCPUUsage / float64(activeOnlineCPUs)
	}

	return summary
}

// SaveToFile serializes cluster nodes to disk.
func (r *Registry) SaveToFile() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.saveToFileLocked()
}

func (r *Registry) saveToFileLocked() error {
	if r.storagePath == "" {
		return nil
	}
	data, err := json.MarshalIndent(r.nodes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.storagePath, data, 0644)
}

// LoadFromFile restores cluster state from disk if available.
func (r *Registry) LoadFromFile() error {
	if r.storagePath == "" {
		return nil
	}
	data, err := os.ReadFile(r.storagePath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return json.Unmarshal(data, &r.nodes)
}

func generateRandomID(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
