package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"

	"twNetMap/backend/datastore"
	"twNetMap/backend/scanner"
)

// LLMResponse matches the strict format requested.
type LLMResponse struct {
	Nodes []InferNode `json:"nodes"`
	Links []InferLink `json:"links"`
}

type InferNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Type   string `json:"type"` // router | switch | wifi | mobile | pc | server | printer | unknown
	Reason string `json:"reason"`
}

type InferLink struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // "lan"
}

// RunInference connects to the configured LLM and runs the topology inference.
func RunInference(ctx context.Context, cfg *datastore.Config, scanResults []*scanner.ScanResult) (*LLMResponse, error) {
	var model llms.Model
	var err error

	switch cfg.ActiveProvider {
	case "ollama":
		url := cfg.OllamaURL
		if url == "" {
			url = "http://localhost:11434"
		}
		mName := cfg.OllamaModel
		if mName == "" {
			mName = "llama3"
		}
		model, err = ollama.New(
			ollama.WithServerURL(url),
			ollama.WithModel(mName),
		)
	case "openai":
		if cfg.APIKeyOpenAI == "" {
			return nil, fmt.Errorf("OpenAI API key is empty")
		}
		model, err = openai.New(
			openai.WithToken(cfg.APIKeyOpenAI),
		)
	case "gemini":
		if cfg.APIKeyGemini == "" {
			return nil, fmt.Errorf("Gemini API key is empty")
		}
		model, err = googleai.New(
			ctx,
			googleai.WithAPIKey(cfg.APIKeyGemini),
		)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.ActiveProvider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Format prompt
	scanJSON, err := json.MarshalIndent(scanResults, "", "  ")
	if err != nil {
		return nil, err
	}

	systemPrompt := `You are an expert network administrator and AI topology engine.
You are given a JSON array of network scan results containing IP addresses, MAC addresses, OUI vendor names, open ports, SNMP/LLDP properties, and TCP banners/HTML responses.
Analyze the data and determine:
1. The device type of each node. Valid types are: "router", "switch", "wifi", "mobile", "pc", "server", "printer", "unknown".
   - Guidelines:
     - Open SNMP ports (161) or switches vendors (e.g., Cisco, Allied Telesis, Juniper) often indicate "switch" or "router".
     - Open HTTP/HTTPS and router UI keywords or dual interfaces suggest "router" or "switch".
     - Wifi access points, wireless controllers, or wireless bridges (e.g., vendors like "Buffalo", "TP-Link", "Netgear", "Ubiquiti", or banners containing wireless/SSID/AP keywords) suggest "wifi".
     - Mobile devices like smartphones and tablets (e.g., Apple, Samsung, Google vendors, or banners with mobile OS keywords) suggest "mobile".
     - Port 9100 or 515 suggests "printer".
     - Ports 22, 3306, 5432 suggest "server" (or Linux machines).
     - PC/Desktop OS vendors or common endpoints suggest "pc".
2. The banners and HTML info:
   - Make use of the "banners" field (port-to-text map) which contains server/service banners (e.g. SSH/FTP welcome strings) and tag-stripped HTML page content.
   - Use these titles, headers, and text snippets to identify device model, vendor, or operating system. For example, if a title says "AP-100 Setup" or "Wireless AP", it is a "wifi" device.
3. The connectivity (links) between nodes.
   - Use LLDP neighbor information (matching chassis IDs, system names, or management IPs to other nodes) to establish links.
   - Use structural reasoning: switches connect to endpoints (PCs, servers, printers, wifi APs) and other switches; routers are default gateways that connect to outer links or main switches.
   - Return links in a non-directed manner (e.g. from A to B). Do not duplicate links (e.g., if link A->B exists, do not add B->A).

You MUST output strictly valid JSON conforming to the following schema without any conversational text or markdown codeblocks (do NOT wrap with ` + "`" + `json...` + "`" + `).

Output Schema:
{
  "nodes": [
    {
      "id": "node_unique_id (use the IP or MAC address as ID)",
      "label": "node_label (use SysName, HostName, or IP address)",
      "type": "router | switch | wifi | mobile | pc | server | printer | unknown",
      "reason": "Brief explanation of how you classified this node"
    }
  ],
  "links": [
    {
      "from": "node_id_1",
      "to": "node_id_2",
      "type": "lan"
    }
  ]
}`

	userPrompt := fmt.Sprintf("Here is the raw scan data:\n\n%s\n\nGenerate the network topology JSON:", string(scanJSON))

	log.Printf("ai userPrompt=%s", userPrompt)
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	resp, err := model.GenerateContent(ctx, content, llms.WithTemperature(0.1))
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	text := resp.Choices[0].Content
	// Strip markdown blocks if the LLM ignored instructions
	text = cleanJSONResponse(text)

	log.Printf("ai resp=%s", text)

	var llmResp LLMResponse
	if err := json.Unmarshal([]byte(text), &llmResp); err != nil {
		log.Printf("Raw LLM output: %s", text)
		return nil, fmt.Errorf("failed to parse LLM JSON response: %w", err)
	}

	return &llmResp, nil
}

func cleanJSONResponse(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var body []string
		for _, line := range lines {
			if !strings.HasPrefix(line, "```") {
				body = append(body, line)
			}
		}
		text = strings.Join(body, "\n")
	}
	return strings.TrimSpace(text)
}
