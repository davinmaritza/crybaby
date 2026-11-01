package types

import "time"

// NodeStatus defines the operational lifecycle of a node.
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "ONLINE"
	NodeStatusOffline NodeStatus = "OFFLINE"
	NodeStatusPending NodeStatus = "PENDING"
	NodeStatusWarning NodeStatus = "WARNING"
)

// NodeHardware contains static hardware specifications for a Windows 11 host.
type NodeHardware struct {
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	KernelVersion  string `json:"kernel_version"`
	CPUModel       string `json:"cpu_model"`
	CPUCores       int    `json:"cpu_cores"`
	CPUThreads     int    `json:"cpu_threads"`
	TotalRAMBytes  uint64 `json:"total_ram_bytes"`
	TotalDiskBytes uint64 `json:"total_disk_bytes"`
	MACAddress     string `json:"mac_address"`
	HyperVEnabled  bool   `json:"hyperv_enabled"`
}

// NodeMetrics contains real-time operational telemetry.
type NodeMetrics struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUUsagePct    float64   `json:"cpu_usage_pct"`
	RAMUsedBytes   uint64    `json:"ram_used_bytes"`
	RAMUsagePct    float64   `json:"ram_usage_pct"`
	DiskUsedBytes  uint64    `json:"disk_used_bytes"`
	DiskUsagePct   float64   `json:"disk_usage_pct"`
	NetBytesSent   uint64    `json:"net_bytes_sent"`
	NetBytesRecv   uint64    `json:"net_bytes_recv"`
	ActiveVMCount  int       `json:"active_vm_count"`
	UptimeSeconds  uint64    `json:"uptime_seconds"`
}

// Node represents a managed Windows 11 machine in the Crybaby cluster.
type Node struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	IPAddress      string       `json:"ip_address"`
	MeshIP         string       `json:"mesh_ip"`
	Status         NodeStatus   `json:"status"`
	Hardware       NodeHardware `json:"hardware"`
	LatestMetrics  NodeMetrics  `json:"latest_metrics"`
	LastSeen       time.Time    `json:"last_seen"`
	RegisteredAt   time.Time    `json:"registered_at"`
	WireGuardPubKey string      `json:"wireguard_pubkey"`
}

// RegisterNodeRequest is submitted by the node agent during initial onboarding.
type RegisterNodeRequest struct {
	ClusterToken   string       `json:"cluster_token"`
	NodeName       string       `json:"node_name"`
	Hardware       NodeHardware `json:"hardware"`
	WireGuardPubKey string      `json:"wireguard_pubkey"`
}

// RegisterNodeResponse returns cluster configuration to the newly joined node.
type RegisterNodeResponse struct {
	NodeID          string `json:"node_id"`
	MeshIP          string `json:"mesh_ip"`
	ControllerMeshIP string `json:"controller_mesh_ip"`
	WireGuardConfig string `json:"wireguard_config"`
	HeartbeatSec    int    `json:"heartbeat_sec"`
}

// HeartbeatPayload is sent periodically from the node agent to the controller.
type HeartbeatPayload struct {
	NodeID  string      `json:"node_id"`
	Metrics NodeMetrics `json:"metrics"`
}
