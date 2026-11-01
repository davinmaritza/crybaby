package types

// ClusterSummary represents the aggregated health and resource state of the cluster.
type ClusterSummary struct {
	TotalNodes      int     `json:"total_nodes"`
	OnlineNodes     int     `json:"online_nodes"`
	OfflineNodes    int     `json:"offline_nodes"`
	TotalCPUCores   int     `json:"total_cpu_cores"`
	AvgCPUUsagePct  float64 `json:"avg_cpu_usage_pct"`
	TotalRAMBytes   uint64  `json:"total_ram_bytes"`
	UsedRAMBytes    uint64  `json:"used_ram_bytes"`
	TotalDiskBytes  uint64  `json:"total_disk_bytes"`
	UsedDiskBytes   uint64  `json:"used_disk_bytes"`
	TotalActiveVMs  int     `json:"total_active_vms"`
	MeshNetworkCIDR string  `json:"mesh_network_cidr"`
}

// AuthRequest is used for controller dashboard login.
type AuthRequest struct {
	Password string `json:"password"`
}

// AuthResponse returns JWT session credentials.
type AuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// USBProvisionProfile holds configuration embedded on the USB installer.
type USBProvisionProfile struct {
	ControllerURL string `json:"controller_url"`
	ClusterToken  string `json:"cluster_token"`
	ClusterName   string `json:"cluster_name"`
	CreatedAt     string `json:"created_at"`
}

// TelemetryPoint represents a single cluster performance snapshot in time.
type TelemetryPoint struct {
	Timestamp int64   `json:"timestamp"`
	CPU       float64 `json:"cpu"`
	RAMPct    float64 `json:"ram_pct"`
}

