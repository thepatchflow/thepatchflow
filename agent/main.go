package main

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

func main() {
	http.HandleFunc("/heal", handleHealRequest)
	fmt.Println("AI Agent Service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
