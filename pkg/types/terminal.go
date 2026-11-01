package types

// TerminalMessageType identifies the payload intent in a terminal WebSocket stream.
type TerminalMessageType string

const (
	TerminalMsgInput  TerminalMessageType = "input"
	TerminalMsgOutput TerminalMessageType = "output"
	TerminalMsgResize TerminalMessageType = "resize"
	TerminalMsgError  TerminalMessageType = "error"
)

// TerminalMessage represents a chunk of shell I/O or terminal state change.
type TerminalMessage struct {
	Type TerminalMessageType `json:"type"`
	Data string              `json:"data,omitempty"`
	Rows uint16              `json:"rows,omitempty"`
	Cols uint16              `json:"cols,omitempty"`
}
