package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"patchflow/healstore"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

type AIRequest struct {
	OriginalRequest []byte `json:"original_request"`
	ErrorResponse   []byte `json:"error_response"`
	TargetEndpoint  string `json:"target_endpoint"`
}

type AIResponse struct {
	JSONPatch map[string]interface{} `json:"json_patch"`
}

// RecordPayload is POSTed by the proxy after it replays the patched request
// against the upstream API. It is the ground-truth entry in the healing index.
type RecordPayload struct {
	Vendor       string            `json:"vendor"`
	Endpoint     string            `json:"endpoint"`
	OldSchema    map[string]string `json:"old_schema"`
	NewSchema    map[string]string `json:"new_schema"`
	Patch        map[string]string `json:"patch"`
	Verified     bool              `json:"verified"`
	ReplayStatus int               `json:"replay_status"`
}

var healStore = healstore.New(healDataPath())

func healDataPath() string {
	if p := os.Getenv("HEAL_DATA_PATH"); p != "" {
		return p
	}
	return healstore.DefaultPath
}

// SeedStore idempotently populates the ground-truth index for demos.
func SeedStore() (added, total int) {
	added = healStore.Seed()
	return added, len(healStore.List(10000))
}

// DataPath exposes the persistence location of the index.
func DataPath() string {
	return healStore.Path()
}

func init() {
	godotenv.Load() // Loads .env if it exists
}

// llmClient returns an OpenAI-compatible client. Preferred path is the Tkngate
// Zero-Trust sidecar (scoped proxy token); falls back to a direct OpenAI key.
func llmClient() (*openai.Client, error) {
	proxyToken := os.Getenv("TKNGATE_PROXY_TOKEN")
	if proxyToken != "" && proxyToken != "sk-your-proxy-token-here" {
		fmt.Println("[AGENT] 🔐 Routing LLM calls through Tkngate Zero-Trust sidecar")
		config := openai.DefaultConfig(proxyToken)
		config.BaseURL = "http://localhost:9090/v1"
		return openai.NewClientWithConfig(config), nil
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		fmt.Println("[AGENT] 🧠 Using direct OpenAI API key")
		return openai.NewClient(apiKey), nil
	}
	return nil, fmt.Errorf("missing TKNGATE_PROXY_TOKEN or OPENAI_API_KEY")
}

func handleHealRequest(w http.ResponseWriter, r *http.Request) {
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fmt.Printf("[AGENT] Analyzing failure for endpoint: %s\n", req.TargetEndpoint)
	fmt.Printf("[AGENT] Error received from upstream: %s\n", string(req.ErrorResponse))

	client, err := llmClient()
	if err != nil {
		fmt.Println("[AGENT] ⚠️ No LLM credentials found:", err)
		fmt.Println("[AGENT] ⚠️ Falling back to mocked response. Set TKNGATE_PROXY_TOKEN or OPENAI_API_KEY for the real path.")
		sendMockResponse(w, req.TargetEndpoint)
		return
	}

	prompt := fmt.Sprintf(`You are an API self-healing infrastructure agent. 
An HTTP request failed because of a schema change.
Target Endpoint: %s
Original Request Payload: %s
Error Response from API: %s

Your job is to deduce how the payload must be transformed to fix the error.
Return ONLY a valid JSON object representing a key-mapping patch. 
For example, if 'charge' is deprecated for 'amount', return: {"charge": "amount"}`,
		req.TargetEndpoint, string(req.OriginalRequest), string(req.ErrorResponse))

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a specialized JSON-translation AI. Output ONLY valid JSON representing the mapping of old keys to new keys.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		},
	)

	if err != nil {
		fmt.Printf("[AGENT] ❌ LLM Error: %v\n", err)
		http.Error(w, "LLM failed", http.StatusInternalServerError)
		return
	}

	llmOutput := resp.Choices[0].Message.Content
	fmt.Printf("[AGENT] 🧠 LLM deduced fix: %s\n", llmOutput)

	var patch map[string]interface{}
	if err := json.Unmarshal([]byte(llmOutput), &patch); err != nil {
		fmt.Printf("[AGENT] ❌ Failed to parse LLM JSON: %v\n", err)
		http.Error(w, "Invalid LLM output format", http.StatusInternalServerError)
		return
	}

	finalResp := AIResponse{JSONPatch: patch}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResp)
}

func sendMockResponse(w http.ResponseWriter, endpoint string) {
	fmt.Println("[AGENT] 🧠 (Mocked) LLM deduced breaking change: 'charge' was deprecated for 'amount'. Generating translation rule...")
	patch := map[string]interface{}{"charge": "amount"}
	resp := AIResponse{JSONPatch: patch}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHealRecord(w http.ResponseWriter, r *http.Request) {
	var p RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if p.Vendor == "" {
		p.Vendor = healstore.Vendor(p.Endpoint)
	}

	event := healStore.Record(healstore.HealEvent{
		Vendor:       p.Vendor,
		Endpoint:     p.Endpoint,
		OldSchema:    p.OldSchema,
		NewSchema:    p.NewSchema,
		Patch:        p.Patch,
		Verified:     p.Verified,
		ReplayStatus: p.ReplayStatus,
	})

	fmt.Printf("[AGENT] 📦 Heal event recorded: %s %s (verified=%v, status=%d)\n", p.Vendor, p.Endpoint, p.Verified, p.ReplayStatus)

	if len(p.Patch) > 0 {
		// The PR + verification comment happens off the proxy's hot path.
		go func(ev healstore.HealEvent) {
			prURL := OpenPR(ev.Endpoint, stringMapToInterface(ev.Patch), ev.Verified, ev.ReplayStatus)
			if prURL != "" {
				healStore.UpdatePRURL(ev.ID, prURL)
				fmt.Printf("[PR ENGINE] 📝 PR URL stored for event %s: %s\n", ev.ID, prURL)
			}
		}(event)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

func handleHeals(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 0 {
			limit = n
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healStore.List(limit))
}

// handleVendors exposes the per-vendor breaking-change index stats.
func handleVendors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healStore.Leaders())
}

// handlePredictions exposes the proactive-heal signals: which vendor field
// migrations have recurred most, so future breakages can be predicted.
func handlePredictions(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 0 {
			limit = n
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healStore.Predictions(limit))
}

func stringMapToInterface(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

type DependabotPayload struct {
	Action string `json:"action"`
	Alert  struct {
		SecurityVulnerability struct {
			Package struct {
				Name string `json:"name"`
			} `json:"package"`
			FirstPatchedVersion struct {
				Identifier string `json:"identifier"`
			} `json:"first_patched_version"`
		} `json:"security_vulnerability"`
	} `json:"alert"`
}

func handleDependabotWebhook(w http.ResponseWriter, r *http.Request) {
	var payload DependabotPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if payload.Action != "created" {
		w.WriteHeader(http.StatusOK)
		return
	}

	pkgName := payload.Alert.SecurityVulnerability.Package.Name
	patchedVer := payload.Alert.SecurityVulnerability.FirstPatchedVersion.Identifier

	fmt.Printf("[AGENT] 🚨 Dependabot Alert Received: Vulnerability in %s. Upgrade required to %s\n", pkgName, patchedVer)
	fmt.Println("[AGENT] 🧠 Simulating AI Code Refactoring...")

	// In a real scenario, the agent would clone the repo, bump the version, run `go build`, 
	// capture compiler errors, and ask the LLM to rewrite the code.
	// We simulate this logic here.
	prompt := fmt.Sprintf(`You are an AI code refactoring agent.
A repository must upgrade '%s' to version '%s' for security reasons.
This caused a compiler error because the library API changed.
Please rewrite the broken code to use the new library API.
Return ONLY valid source code.`, pkgName, patchedVer)

	fmt.Printf("[AGENT] 🧠 LLM Prompt for Code Refactoring:\n%s\n", prompt)

	proxyToken := os.Getenv("TKNGATE_PROXY_TOKEN")
	if proxyToken == "" || proxyToken == "sk-your-proxy-token-here" {
		fmt.Println("[AGENT] ⚠️ No valid TKNGATE_PROXY_TOKEN found. Simulating LLM response.")
		
		simulatedCode := fmt.Sprintf(`package main

import (
	"fmt"
	"%s"
)

func main() {
	// Updated to use the new API from v%s
	client := %s.NewClient(config)
	fmt.Println(client)
}`, pkgName, patchedVer, pkgName)
		
		go func() {
			FixDependabotAlert(pkgName, patchedVer, "main.go", simulatedCode)
		}()
		
		w.WriteHeader(http.StatusOK)
		return
	}

	// Real LLM call would go here using Tkngate proxy (similar to handleHealRequest)
	// For MVP simplicity, we fall back to simulated code if no real errors are provided.
	
	w.WriteHeader(http.StatusOK)
}

func Start() {
	http.HandleFunc("/heal", handleHealRequest)
	http.HandleFunc("/heal/record", handleHealRecord)
	http.HandleFunc("/api/heals", handleHeals)
	http.HandleFunc("/api/vendors", handleVendors)
	http.HandleFunc("/api/predictions", handlePredictions)
	http.HandleFunc("/webhook/dependabot", handleDependabotWebhook)
	fmt.Println("AI Agent Service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
