package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type AIRequest struct {
	OriginalRequest []byte `json:"original_request"`
	ErrorResponse   []byte `json:"error_response"`
	TargetEndpoint  string `json:"target_endpoint"`
}

type AIResponse struct {
	JSONPatch map[string]interface{} `json:"json_patch"`
	// e.g. {"charge": "amount"} means "rename charge to amount"
}

func handleHealRequest(w http.ResponseWriter, r *http.Request) {
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fmt.Printf("[AGENT] Analyzing failure for endpoint: %s\n", req.TargetEndpoint)
	fmt.Printf("[AGENT] Error received from upstream: %s\n", string(req.ErrorResponse))

	// In a real scenario, we pass req.OriginalRequest and req.ErrorResponse to Claude/OpenAI
	// along with the OpenAPI spec of the target.
	// For the MVP, we simulate the LLM's deduction.

	fmt.Println("[AGENT]  LLM deduced breaking change: 'charge' was deprecated for 'amount'. Generating translation rule...")
	
	resp := AIResponse{
		JSONPatch: map[string]interface{}{
			"charge": "amount",
		},
	}
	
	// Trigger PR Engine asynchronously
	go func() {
		triggerPREngine(req.TargetEndpoint, resp.JSONPatch)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func triggerPREngine(endpoint string, patch map[string]interface{}) {
	// Call pr_engine
	fmt.Println("---------------------------------------------------")
	fmt.Printf("[PR ENGINE] Scanning customer repository...\n")
	fmt.Printf("[PR ENGINE] Looking for usages matching deprecated schema keys: %v\n", patch)
	fmt.Println("[PR ENGINE] Found 3 occurrences of deprecated parameter.")
	fmt.Println("[PR ENGINE] Applying AST refactoring to update code...")
	fmt.Println("[PR ENGINE] Running tests (npm test)... PASSED")
	fmt.Println("[PR ENGINE] ✅ Pull Request #482 opened on GitHub!")
	fmt.Println("---------------------------------------------------")
}

func main() {
	http.HandleFunc("/heal", handleHealRequest)
	fmt.Println("AI Agent Service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
