package mockapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Mock API that expects {"amount": 10} instead of {"charge": 10}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The "Breaking Change": We used to accept "charge", now we only accept "amount"
	if _, ok := req["charge"]; ok {
		// Return 400 Bad Request to simulate schema failure
		http.Error(w, `{"error": "invalid_request_error", "message": "Received unknown parameter: charge. Use amount instead."}`, http.StatusBadRequest)
		return
	}

	if amount, ok := req["amount"]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "success", "amount_charged": %v}`, amount)
		return
	}

	http.Error(w, `{"error": "invalid_request_error", "message": "Missing required parameter: amount"}`, http.StatusBadRequest)
}

func Start() {
	http.HandleFunc("/v1/payments", paymentHandler)
	fmt.Println("Mock API listening on :8081 (Expects payload: {\"amount\": <int>})")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
