package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

func init() {
	godotenv.Load() // Loads .env if it exists
}

func handleHealRequest(w http.ResponseWriter, r *http.Request) {
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fmt.Printf("[AGENT] Analyzing failure for endpoint: %s\n", req.TargetEndpoint)
	fmt.Printf("[AGENT] Error received from upstream: %s\n", string(req.ErrorResponse))

	proxyToken := os.Getenv("TKNGATE_PROXY_TOKEN")
	if proxyToken == "" || proxyToken == "sk-your-proxy-token-here" {
		fmt.Println("[AGENT] ⚠️ No valid TKNGATE_PROXY_TOKEN found. Falling back to mocked response.")
		sendMockResponse(w, req.TargetEndpoint)
		return
	}

	// ZERO-TRUST SECURITY: Route through Tkngate sidecar instead of hitting OpenAI directly.
	config := openai.DefaultConfig(proxyToken)
	config.BaseURL = "http://localhost:9090/v1"
	client := openai.NewClientWithConfig(config)
	
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

	go func() {
		OpenPR(req.TargetEndpoint, finalResp.JSONPatch)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResp)
}

func sendMockResponse(w http.ResponseWriter, endpoint string) {
	fmt.Println("[AGENT] 🧠 (Mocked) LLM deduced breaking change: 'charge' was deprecated for 'amount'. Generating translation rule...")
	patch := map[string]interface{}{"charge": "amount"}
	resp := AIResponse{JSONPatch: patch}
	
	go func() {
		OpenPR(endpoint, resp.JSONPatch)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
	http.HandleFunc("/webhook/dependabot", handleDependabotWebhook)
	fmt.Println("AI Agent Service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
