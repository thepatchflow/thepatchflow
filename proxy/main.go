package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

var translationCache = make(map[string]map[string]string)

func main() {
	targetURL, _ := url.Parse("http://localhost:8081")
	agentURL := "http://localhost:8082/heal"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		
		// 1. Check Cache
		if rule, ok := translationCache[r.URL.Path]; ok {
			fmt.Println("[PROXY] ⚡ Cache Hit! Applying translation rule:", rule)
			bodyBytes = applyTranslation(bodyBytes, rule)
		}

		// 2. Forward to Target
		fmt.Printf("[PROXY] Forwarding request to %s\n", targetURL.String()+r.URL.Path)
		req, _ := http.NewRequest(r.Method, targetURL.String()+r.URL.Path, bytes.NewBuffer(bodyBytes))
		req.Header = r.Header.Clone()

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "Target unreachable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		// 3. Check for 400 Bad Request (Schema breaking change)
		if resp.StatusCode == http.StatusBadRequest {
			fmt.Println("[PROXY] 🚨 Detected breaking schema change (400)!")
			fmt.Println("[PROXY] 🧠 Calling AI Agent to generate fix...")
			
			agentPayload := map[string]interface{}{
				"original_request": bodyBytes,
				"error_response":   respBody,
				"target_endpoint":  r.URL.Path,
			}
			agentPayloadBytes, _ := json.Marshal(agentPayload)
			
			agentResp, err := http.Post(agentURL, "application/json", bytes.NewBuffer(agentPayloadBytes))
			if err == nil {
				var aiResult map[string]interface{}
				json.NewDecoder(agentResp.Body).Decode(&aiResult)
				agentResp.Body.Close()
				
				if patch, ok := aiResult["json_patch"].(map[string]interface{}); ok {
					rule := make(map[string]string)
					for k, v := range patch {
						rule[k] = v.(string)
					}
					
					fmt.Println("[PROXY] 🔧 Applying AI fix and caching rule...")
					translationCache[r.URL.Path] = rule
					
					newBodyBytes := applyTranslation(bodyBytes, rule)
					
					// Replay request
					fmt.Println("[PROXY] 🔁 Replaying request to target API...")
					retryReq, _ := http.NewRequest(r.Method, targetURL.String()+r.URL.Path, bytes.NewBuffer(newBodyBytes))
					retryReq.Header = req.Header.Clone()
					
					retryResp, _ := http.DefaultClient.Do(retryReq)
					defer retryResp.Body.Close()
					
					retryRespBody, _ := io.ReadAll(retryResp.Body)
					
					// Return fixed response
					for k, v := range retryResp.Header {
						w.Header()[k] = v
					}
					w.WriteHeader(retryResp.StatusCode)
					w.Write(retryRespBody)
					fmt.Println("[PROXY] ✅ Request successfully self-healed!")
					return
				}
			} else {
				fmt.Println("[PROXY] Failed to reach AI Agent:", err)
			}
		}

		// Normal flow
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	})

	fmt.Println("Patchflow Proxy listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func applyTranslation(body []byte, rule map[string]string) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	
	modified := false
	for oldKey, newKey := range rule {
		if val, exists := payload[oldKey]; exists {
			delete(payload, oldKey)
			payload[newKey] = val
			modified = true
		}
	}
	
	if modified {
		newBody, _ := json.Marshal(payload)
		return newBody
	}
	return body
}
