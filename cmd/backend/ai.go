package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var (
	ollamaNodeCounter   uint64
	aiSystemInstruction = `You are the AI Assistant for CryBaby RMM (Remote Monitoring & Management) system by Suzirz.
You can monitor and manage connected servers.
You have access to the following tools:
1. get_fleet_status: Returns a high-level summary of the fleet (total servers, online status).
2. get_servers: Returns detailed info about all connected servers (UUID, hostname, OS).
3. get_global_resources: Gets aggregated CPU cores, RAM, Disk across ALL connected servers.
4. execute_command(server_uuid, command): Executes a shell command on a target server and returns the output.

When asked to perform an action, use the appropriate tool. If you need a UUID, look it up using get_servers first.`
)

// AskAI routes to Ollama (local) or Gemini (cloud) based on config
func AskAI(userPrompt string, db *DB, hub *Hub) (string, error) {
	// Priority 1: Ollama (local AI on user's servers)
	if globalConfig.OllamaURL != "" {
		return AskOllama(userPrompt, db, hub)
	}

	// Priority 2: Gemini API
	if globalConfig.GeminiAPIKey != "" {
		return AskGemini(userPrompt, db, hub)
	}

	return "⚠️ AI tidak dikonfigurasi.\n\nUntuk menggunakan Local AI (Ollama di server Anda), tambahkan ke config.json:\n  \"ollama_url\": \"http://IP_SERVER:11434\"\n  \"ollama_model\": \"llama3\"\n\nAtau tambahkan \"gemini_api_key\" untuk menggunakan Gemini AI.", nil
}

// ─── OLLAMA (Local AI Agent) ─────────────────────────────────────────────────

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
}

type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
}

func AskOllama(userPrompt string, db *DB, hub *Hub) (string, error) {
	model := globalConfig.OllamaModel
	if model == "" {
		model = "llama3"
	}

	fleetCtx := buildFleetContext(db, hub)
	messages := []ollamaMessage{
		{Role: "system", Content: aiSystemInstruction + "\n\n" + fleetCtx},
		{Role: "user", Content: userPrompt},
	}

	// 1. Try static ollama_url if configured
	if globalConfig.OllamaURL != "" {
		ollamaURL := strings.TrimRight(globalConfig.OllamaURL, "/")
		log.Printf("AI (Ollama): routing to static URL %s using model %s", ollamaURL, model)
		reply, err := callOllama(ollamaURL, model, messages)
		if err == nil {
			return reply, nil
		}
		log.Printf("AI (Ollama): static URL failed (%v), trying active online fleet agents...", err)
	}

	// 2. Dynamic discovery: Round-Robin load balancing across online agent nodes on port 11434
	servers, err := db.GetServers()
	if err == nil && len(servers) > 0 {
		// Gather all online node URLs
		var onlineNodes []string
		for _, s := range servers {
			hub.clientsMu.RLock()
			client, isOnline := hub.clients[s.ID]
			hub.clientsMu.RUnlock()

			if isOnline && client != nil {
				remoteAddr := client.conn.RemoteAddr().String()
				ip := remoteAddr
				if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
					ip = remoteAddr[:idx]
				}
				ip = strings.TrimPrefix(ip, "[")
				ip = strings.TrimSuffix(ip, "]")
				onlineNodes = append(onlineNodes, fmt.Sprintf("http://%s:11434", ip))
			}
		}

		if len(onlineNodes) > 0 {
			// Round-Robin selection
			startIndex := int(atomic.AddUint64(&ollamaNodeCounter, 1) % uint64(len(onlineNodes)))
			for i := 0; i < len(onlineNodes); i++ {
				idx := (startIndex + i) % len(onlineNodes)
				targetURL := onlineNodes[idx]

				log.Printf("AI (Ollama): [Round-Robin Load Balancing] routing request to node %s...", targetURL)
				reply, err := callOllama(targetURL, model, messages)
				if err == nil {
					log.Printf("AI (Ollama): SUCCESS response from node %s", targetURL)
					return reply, nil
				}
				log.Printf("AI (Ollama): node %s failed (%v), trying next node...", targetURL, err)
			}
		}
	}

	return "", fmt.Errorf("none of the online agent nodes responded on port 11434. Make sure Ollama is running on at least one online PC.")
}



func callOllama(baseURL, model string, messages []ollamaMessage) (string, error) {
	reqBody := ollamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(baseURL+"/api/chat", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("cannot reach Ollama at %s: %v", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode ollama response: %v", err)
	}

	return ollamaResp.Message.Content, nil
}

// buildFleetContext creates a real-time snapshot of the fleet for the AI context
func buildFleetContext(db *DB, hub *Hub) string {
	servers, err := db.GetServers()
	if err != nil {
		return "[Fleet data unavailable]"
	}

	var sb strings.Builder
	sb.WriteString("=== CURRENT FLEET STATUS ===\n")

	var onlineCount int
	var totalCores int
	var totalRAMGB float64

	for _, s := range servers {
		hub.clientsMu.RLock()
		_, isOnline := hub.clients[s.ID]
		hub.clientsMu.RUnlock()

		status := "OFFLINE"
		if isOnline {
			status = "ONLINE"
			onlineCount++
			totalCores += s.CPUCores
			totalRAMGB += float64(s.RAMTotalMB) / 1024
		}

		name := s.Hostname
		if s.CustomName != nil && *s.CustomName != "" {
			name = *s.CustomName
		}

		sb.WriteString(fmt.Sprintf("- UUID: %s | Name: %s | OS: %s | Status: %s | CPU: %d cores | RAM: %.1f GB\n",
			s.ID, name, s.OSVersion, status, s.CPUCores, float64(s.RAMTotalMB)/1024))
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d total servers, %d online, Total CPU cores: %d, Total RAM: %.1f GB\n",
		len(servers), onlineCount, totalCores, totalRAMGB))
	sb.WriteString("=== END FLEET STATUS ===\n")

	return sb.String()
}

// ─── GEMINI (Cloud AI) ───────────────────────────────────────────────────────

func AskGemini(userPrompt string, db *DB, hub *Hub) (string, error) {
	apiKey := globalConfig.GeminiAPIKey

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create GenAI client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(aiSystemInstruction)},
	}

	model.Tools = []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "get_fleet_status",
					Description: "Gets a summary of the overall fleet status, including online/offline counts.",
				},
				{
					Name:        "get_global_resources",
					Description: "Gets aggregated resource statistics (total CPU cores, total RAM, total Disk space) across all connected servers.",
				},
				{
					Name:        "get_servers",
					Description: "Gets detailed information about all connected servers, including their UUIDs, hostnames, and OS.",
				},
				{
					Name:        "execute_command",
					Description: "Executes a powershell/bash command on a specific server and returns the output.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"server_uuid": {
								Type:        genai.TypeString,
								Description: "The UUID of the target server.",
							},
							"command": {
								Type:        genai.TypeString,
								Description: "The shell command to execute.",
							},
						},
						Required: []string{"server_uuid", "command"},
					},
				},
			},
		},
	}

	chat := model.StartChat()

	log.Printf("AI (Gemini): Received prompt: %s", userPrompt)
	resp, err := chat.SendMessage(ctx, genai.Text(userPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to send message to AI: %v", err)
	}

	for {
		if len(resp.Candidates) == 0 {
			return "AI returned an empty response.", nil
		}

		candidate := resp.Candidates[0]

		var functionCalls []*genai.FunctionCall
		for _, part := range candidate.Content.Parts {
			if call, ok := part.(genai.FunctionCall); ok {
				functionCalls = append(functionCalls, &call)
			}
		}

		if len(functionCalls) == 0 {
			var outBuilder strings.Builder
			for _, part := range candidate.Content.Parts {
				if t, ok := part.(genai.Text); ok {
					outBuilder.WriteString(string(t))
				}
			}
			return outBuilder.String(), nil
		}

		var toolResponses []genai.Part
		for _, call := range functionCalls {
			log.Printf("AI (Gemini): Calling tool %s", call.Name)

			var result interface{}
			switch call.Name {
			case "get_fleet_status":
				servers, _ := db.GetServers()
				online := 0
				for _, s := range servers {
					hub.clientsMu.RLock()
					_, isOnline := hub.clients[s.ID]
					hub.clientsMu.RUnlock()
					if isOnline {
						online++
					}
				}
				result = map[string]interface{}{
					"total_servers":  len(servers),
					"online_servers": online,
				}

			case "get_global_resources":
				servers, _ := db.GetServers()
				var onlineCount, totalCores, totalThreads int
				var totalRAM, totalDisk uint64
				for _, s := range servers {
					hub.clientsMu.RLock()
					_, isOnline := hub.clients[s.ID]
					hub.clientsMu.RUnlock()
					if isOnline {
						onlineCount++
						totalCores += s.CPUCores
						totalThreads += s.CPUThreads
						totalRAM += s.RAMTotalMB
						totalDisk += s.DiskTotalMB
					}
				}
				result = map[string]interface{}{
					"online_servers":    onlineCount,
					"total_cpu_cores":   totalCores,
					"total_cpu_threads": totalThreads,
					"total_ram_gb":      float64(totalRAM) / 1024,
					"total_disk_gb":     float64(totalDisk) / 1024,
				}

			case "get_servers":
				servers, _ := db.GetServers()
				type minimalServer struct {
					UUID     string `json:"uuid"`
					Hostname string `json:"hostname"`
					OS       string `json:"os"`
					IsOnline bool   `json:"is_online"`
				}
				var list []minimalServer
				for _, s := range servers {
					hub.clientsMu.RLock()
					_, isOnline := hub.clients[s.ID]
					hub.clientsMu.RUnlock()
					list = append(list, minimalServer{
						UUID:     s.ID,
						Hostname: s.Hostname,
						OS:       s.OSVersion,
						IsOnline: isOnline,
					})
				}
				result = list

			case "execute_command":
				uuidInter, ok1 := call.Args["server_uuid"]
				cmdInter, ok2 := call.Args["command"]
				if !ok1 || !ok2 {
					result = map[string]string{"error": "Missing server_uuid or command parameter"}
				} else {
					uuid, _ := uuidInter.(string)
					cmd, _ := cmdInter.(string)

					hub.clientsMu.RLock()
					_, isOnline := hub.clients[uuid]
					hub.clientsMu.RUnlock()

					if !isOnline {
						result = map[string]string{"error": "Server is offline"}
					} else {
						resp, err := hub.SendCommand(uuid, cmd, "AI Agent")
						if err != nil {
							result = map[string]string{"error": err.Error()}
						} else {
							output := resp.Output
							if resp.Error != "" {
								output += "\nSystem Error: " + resp.Error
							}
							result = map[string]interface{}{
								"exit_code": resp.ExitCode,
								"output":    output,
							}
						}
					}
				}
			default:
				result = map[string]string{"error": "Unknown function"}
			}

			resBytes, _ := json.Marshal(result)
			var resMap map[string]any
			json.Unmarshal(resBytes, &resMap)
			toolResponses = append(toolResponses, genai.FunctionResponse{
				Name:     call.Name,
				Response: resMap,
			})
		}

		resp, err = chat.SendMessage(ctx, toolResponses...)
		if err != nil {
			return "", fmt.Errorf("failed to send tool responses: %v", err)
		}
	}
}
