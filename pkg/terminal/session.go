package terminal

import (
	"encoding/json"
	"io"
	"log"
	"os/exec"
	"runtime"
	"sync"

	"github.com/davinmaritza/crybaby/pkg/types"
	"github.com/gorilla/websocket"
)

// Session manages a live shell process bound bidirectionally to a WebSocket connection.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	conn   *websocket.Conn
	mu     sync.Mutex
	done   chan struct{}
}

// NewSession creates and starts a shell process (PowerShell on Windows, bash on Linux).
func NewSession(conn *websocket.Conn) (*Session, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NoLogo")
	} else {
		cmd = exec.Command("bash", "-i")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, err
	}

	s := &Session{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		conn:   conn,
		done:   make(chan struct{}),
	}

	return s, nil
}

// HandleIO starts the bidirectional read and write loops.
func (s *Session) HandleIO() {
	var wg sync.WaitGroup
	wg.Add(3)

	// Stream stdout to WebSocket
	go func() {
		defer wg.Done()
		s.streamPipeToWS(s.stdout)
	}()

	// Stream stderr to WebSocket
	go func() {
		defer wg.Done()
		s.streamPipeToWS(s.stderr)
	}()

	// Read WebSocket input and forward to shell stdin
	go func() {
		defer wg.Done()
		for {
			_, msgBytes, err := s.conn.ReadMessage()
			if err != nil {
				break
			}

			var termMsg types.TerminalMessage
			if err := json.Unmarshal(msgBytes, &termMsg); err == nil {
				if termMsg.Type == types.TerminalMsgInput && len(termMsg.Data) > 0 {
					s.mu.Lock()
					_, _ = s.stdin.Write([]byte(termMsg.Data))
					s.mu.Unlock()
				}
			}
		}
		s.Close()
	}()

	wg.Wait()
	_ = s.cmd.Wait()
}

func (s *Session) streamPipeToWS(r io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			outMsg := types.TerminalMessage{
				Type: types.TerminalMsgOutput,
				Data: string(buf[:n]),
			}
			msgBytes, _ := json.Marshal(outMsg)
			s.mu.Lock()
			err := s.conn.WriteMessage(websocket.TextMessage, msgBytes)
			s.mu.Unlock()
			if err != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
}

// Close terminates the running process and cleans up open pipes.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}

	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.conn.Close()
	log.Println("Terminal session closed successfully.")
}
