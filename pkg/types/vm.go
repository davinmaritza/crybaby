package types

import "time"

// VMState represents the hypervisor execution state of a VM.
type VMState string

const (
	VMStateRunning VMState = "RUNNING"
	VMStateStopped VMState = "STOPPED"
	VMStatePaused  VMState = "PAUSED"
	VMStateSaved   VMState = "SAVED"
	VMStateUnknown VMState = "UNKNOWN"
)

// VirtualMachine represents a Hyper-V VM running on a Windows 11 node.
type VirtualMachine struct {
	ID             string    `json:"id"`
	NodeID         string    `json:"node_id"`
	Name           string    `json:"name"`
	State          VMState   `json:"state"`
	CPUCount       int       `json:"cpu_count"`
	MemoryMB       uint64    `json:"memory_mb"`
	DiskPath       string    `json:"disk_path"`
	DiskSizeBytes  uint64    `json:"disk_size_bytes"`
	UptimeSeconds  uint64    `json:"uptime_seconds"`
	IPAddresses    []string  `json:"ip_addresses"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateVMRequest specifies parameters for provisioning a new Hyper-V VM.
type CreateVMRequest struct {
	Name          string `json:"name"`
	CPUCount      int    `json:"cpu_count"`
	MemoryMB      uint64 `json:"memory_mb"`
	DiskSizeGB    uint64 `json:"disk_size_gb"`
	SwitchName    string `json:"switch_name"`
	ISOPath       string `json:"iso_path,omitempty"`
}
