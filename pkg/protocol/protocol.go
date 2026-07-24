package protocol

import "encoding/json"

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	TypeRegisterRequest  MessageType = "register_request"
	TypeRegisterResponse MessageType = "register_response"
	TypeHeartbeat        MessageType = "heartbeat"
	TypeCommandRequest   MessageType = "command_request"
	TypeCommandResponse  MessageType = "command_response"
	TypeFileGetRequest   MessageType = "file_get_request"
	TypeFileGetChunk     MessageType = "file_get_chunk"
	TypeFilePutRequest   MessageType = "file_put_request"
	TypeFilePutChunk     MessageType = "file_put_chunk"
	TypeUninstallSignal  MessageType = "uninstall_signal"
	TypeUpdateConfig     MessageType = "update_config"
)

type UpdateConfigRequest struct {
	NewBackendURL string `json:"new_backend_url"`
}

// Message is the top-level container for all WebSocket communications.
type Message struct {
	Type    MessageType     `json:"type"`
	Id      string          `json:"id,omitempty"` // Message ID for matching request/response
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RegisterRequest is sent by the agent upon connecting.
type RegisterRequest struct {
	UUID         string `json:"uuid"`
	Hostname     string `json:"hostname"`
	OSVersion    string `json:"os_version"`
	CPUModel     string `json:"cpu_model"`
	CPUCores     int    `json:"cpu_cores"`
	CPUThreads   int    `json:"cpu_threads"`
	RAMTotalMB   uint64 `json:"ram_total_mb"`
	DiskTotalMB  uint64 `json:"disk_total_mb"`
	AgentVersion string `json:"agent_version"`
	Token        string `json:"token,omitempty"` // Reconnect authentication token
}

// RegisterResponse is sent by the backend to acknowledge registration/auth.
type RegisterResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`            // "approved", "pending_approval", "rejected"
	Token   string `json:"token,omitempty"`   // Newly issued token on first approval
	Message string `json:"message,omitempty"` // Error or informational message
}

// Heartbeat is sent by the agent periodically to report current status.
type Heartbeat struct {
	CPULoadPct    float64 `json:"cpu_load_pct"`
	RAMUsedMB     uint64  `json:"ram_used_mb"`
	DiskUsedMB    uint64  `json:"disk_used_mb"`
	UptimeSeconds uint64  `json:"uptime_seconds"`
}

// CommandRequest is sent by the backend to execute a shell command on the agent.
type CommandRequest struct {
	CommandID string `json:"command_id"`
	Command   string `json:"command"`
}

// CommandResponse is sent by the agent with the command execution result.
type CommandResponse struct {
	CommandID string `json:"command_id"`
	Output    string `json:"output"`
	ExitCode  int    `json:"exit_code"`
	Error     string `json:"error,omitempty"`
}

// FileGetRequest is sent by backend to fetch a file from the agent.
type FileGetRequest struct {
	TransferID string `json:"transfer_id"`
	Path       string `json:"path"`
}

// FileGetChunk is sent by the agent containing a chunk of the requested file.
type FileGetChunk struct {
	TransferID string `json:"transfer_id"`
	ChunkIndex int    `json:"chunk_index"`
	Data       string `json:"data"` // Base64 encoded binary data
	IsEOF      bool   `json:"is_eof"`
	Error      string `json:"error,omitempty"`
}

// FilePutRequest is sent by backend to initiate pushing a file to the agent.
type FilePutRequest struct {
	TransferID string `json:"transfer_id"`
	Path       string `json:"path"`
	TotalSize  int64  `json:"total_size"`
}

// FilePutChunk is sent by backend containing a chunk of the file being pushed.
type FilePutChunk struct {
	TransferID string `json:"transfer_id"`
	ChunkIndex int    `json:"chunk_index"`
	Data       string `json:"data"` // Base64 encoded binary data
	IsEOF      bool   `json:"is_eof"`
	Error      string `json:"error,omitempty"`
}
