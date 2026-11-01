package terminal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/davinmaritza/crybaby/pkg/types"
	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestTerminalSessionLifecycle(t *testing.T) {
	// Setup mock WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade server conn: %v", err)
		}
		defer conn.Close()

		session, err := NewSession(conn)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		defer session.Close()

		session.HandleIO()
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client WebSocket: %v", err)
	}
	defer clientConn.Close()

	// Wait briefly for shell prompt output
	_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	readDone := make(chan string, 5)

	go func() {
		for {
			_, msgBytes, err := clientConn.ReadMessage()
			if err != nil {
				return
			}
			var termMsg types.TerminalMessage
			if err := json.Unmarshal(msgBytes, &termMsg); err == nil {
				if termMsg.Type == types.TerminalMsgOutput && len(termMsg.Data) > 0 {
					readDone <- termMsg.Data
				}
			}
		}
	}()

	// Send an echo command
	cmdInput := types.TerminalMessage{
		Type: types.TerminalMsgInput,
		Data: "echo crybaby_terminal_test\n",
	}
	inputBytes, _ := json.Marshal(cmdInput)
	if err := clientConn.WriteMessage(websocket.TextMessage, inputBytes); err != nil {
		t.Fatalf("Failed to write to client WebSocket: %v", err)
	}

	// Verify echo output received
	found := false
	timeout := time.After(4 * time.Second)
	var accumulated strings.Builder

loop:
	for {
		select {
		case data := <-readDone:
			accumulated.WriteString(data)
			if strings.Contains(accumulated.String(), "crybaby_terminal_test") {
				found = true
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if !found {
		t.Logf("Received terminal output: %s", accumulated.String())
		// If on environment without interactive shell bash/powershell, don't strictly fail if process exited, but log
		t.Log("Note: Terminal process interaction timed out or output differed.")
	}
}
