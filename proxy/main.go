package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"patchflow/healstore"
)

type TelemetryEvent struct {
	ID       int    `json:"id"`
	Endpoint string `json:"endpoint"`
	Status   string `json:"status"`
	Time     string `json:"time"`
	Diff     string `json:"diff"`
	Verified bool   `json:"verified"`
}

type TelemetryData struct {
	TotalErrors int              `json:"totalErrors"`
	TotalHealed int              `json:"totalHealed"`
	TotalPRs    int              `json:"totalPRs"`
	Events      []TelemetryEvent `json:"events"`
}

var (
	rdb              *redis.Client
	ctx              = context.Background()
	translationCache = make(map[string]map[string]string)
	redisAvailable   = false

	telemetry      TelemetryData
	eventIDCounter int
	telemetryMutex sync.Mutex

	agentURLRecord = "http://localhost:8082/heal/record"
	agentURLHeals  = "http://localhost:8082/api/heals"
	agentURLVendors   = "http://localhost:8082/api/vendors"
	agentURLPreds  = "http://localhost:8082/api/predictions"
)

// verified=true means the translation rule was proven against the real upstream:
// the exact failing request was replayed with the translation applied and succeeded.
func pushTelemetryEvent(endpoint, status, diff string, verified bool) {
	telemetryMutex.Lock()
	defer telemetryMutex.Unlock()
	
	eventIDCounter++
	event := TelemetryEvent{
		ID:       eventIDCounter,
		Endpoint: endpoint,
		Status:   status,
		Time:     time.Now().Format("15:04:05"),
		Diff:     diff,
		Verified: verified,
	}
	// Insert at beginning
	telemetry.Events = append([]TelemetryEvent{event}, telemetry.Events...)
	if len(telemetry.Events) > 50 {
		telemetry.Events = telemetry.Events[:50]
	}
	
	if status == "ERROR" || status == "HEALED" {
		telemetry.TotalErrors++
	}
	if status == "HEALED" {
		telemetry.TotalHealed++
		telemetry.TotalPRs++
	}
}

func getCacheRule(endpoint string) (map[string]string, bool) {
	if redisAvailable {
		val, err := rdb.Get(ctx, endpoint).Result()
		if err == nil {
			var rule map[string]string
			if json.Unmarshal([]byte(val), &rule) == nil {
				return rule, true
			}
		}
		return nil, false
	}
	
	rule, ok := translationCache[endpoint]
	return rule, ok
}

func setCacheRule(endpoint string, rule map[string]string) {
	if redisAvailable {
		ruleBytes, _ := json.Marshal(rule)
		rdb.Set(ctx, endpoint, string(ruleBytes), 0)
	} else {
		translationCache[endpoint] = rule
	}
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

// inferSchema extracts the JSON key→type pairs from a request body. This is the
// ground-truth "old_schema"/"new_schema" pair that makes the healing index useful.
func inferSchema(body []byte) map[string]string {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	schema := make(map[string]string, len(payload))
	for k, v := range payload {
		schema[k] = jsonType(v)
	}
	return schema
}

func jsonType(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// proxyAgentJSON fetches a JSON payload from the agent service and forwards it.
func proxyAgentJSON(c *fiber.Ctx, url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(map[string]string{"error": "agent unreachable"})
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(map[string]string{"error": "failed to read agent response"})
	}
	return c.Status(resp.StatusCode).Type("json").Send(body)
}

// recordHealEvent POSTs the ground-truth heal record to the agent's store.
// Runs off the hot path in a goroutine so the proxy never waits on it.
func recordHealEvent(endpoint string, oldBody, newBody []byte, rule map[string]string, verified bool, replayStatus int) {
	payload := map[string]interface{}{
		"vendor":        healstore.Vendor(endpoint),
		"endpoint":      endpoint,
		"old_schema":    inferSchema(oldBody),
		"new_schema":    inferSchema(newBody),
		"patch":         rule,
		"verified":      verified,
		"replay_status": replayStatus,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(agentURLRecord, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Println("[PROXY] ⚠️ Could not record heal event:", err)
		return
	}
	resp.Body.Close()

	if verified {
		fmt.Printf("[PROXY] 📦 Heal event recorded & ground-truth stored for %s (replay HTTP %d)\n", endpoint, replayStatus)
	}
}

func Start() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	
	if _, err := rdb.Ping(ctx).Result(); err == nil {
		fmt.Println("[PROXY] 🟢 Redis connected successfully!")
		redisAvailable = true
	} else {
		// Just set the flag to false. We don't need to print the error every time the CLI boots.
		// It will silently fall back to in-memory mode.
		redisAvailable = false
	}

	app := fiber.New()
	targetURL := "http://localhost:8081"
	agentURL := "http://localhost:8082/heal"

	// Serve the static frontend dist if it exists
	app.Static("/dashboard", "./dist")

	app.Get("/api/telemetry", func(c *fiber.Ctx) error {
		telemetryMutex.Lock()
		defer telemetryMutex.Unlock()
		return c.JSON(telemetry)
	})

	// The ground-truth breaking-change index lives in the agent; proxy here.
	app.Get("/api/heals", func(c *fiber.Ctx) error {
		return proxyAgentJSON(c, agentURLHeals+"?limit="+c.Query("limit", "50"))
	})
	app.Get("/api/vendors", func(c *fiber.Ctx) error {
		return proxyAgentJSON(c, agentURLVendors)
	})
	app.Get("/api/predictions", func(c *fiber.Ctx) error {
		return proxyAgentJSON(c, agentURLPreds+"?limit="+c.Query("limit", "20"))
	})

	app.All("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/dashboard") || strings.HasPrefix(path, "/api/telemetry") || strings.HasPrefix(path, "/api/heals") || strings.HasPrefix(path, "/api/vendors") || strings.HasPrefix(path, "/api/predictions") {
			return c.Next()
		}

		bodyBytes := c.Body()

		// 1. Check Cache
		if rule, ok := getCacheRule(path); ok {
			fmt.Println("[PROXY] ⚡ Cache Hit! Applying translation rule:", rule)
			bodyBytes = applyTranslation(bodyBytes, rule)
			ruleJson, _ := json.Marshal(rule)
			pushTelemetryEvent(c.Method()+" "+path, "HEALED", string(ruleJson), true)
		}

		// 2. Forward to Target using FastHTTP
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		
		req.Header.SetMethod(c.Method())
		req.SetRequestURI(targetURL + path)
		c.Request().Header.VisitAll(func(k, v []byte) {
			req.Header.SetBytesKV(k, v)
		})
		req.SetBody(bodyBytes)

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		fmt.Printf("[PROXY] Forwarding request to %s\n", targetURL+path)
		
		client := &fasthttp.Client{}
		if err := client.Do(req, resp); err != nil {
			return c.Status(fiber.StatusBadGateway).SendString("Target unreachable")
		}

		// 3. Check for 400 Bad Request
		if resp.StatusCode() == fiber.StatusBadRequest {
			fmt.Println("[PROXY] 🚨 Detected breaking schema change (400)!")
			fmt.Println("[PROXY] 🧠 Calling AI Agent to generate fix...")

			agentPayload := map[string]interface{}{
				"original_request": bodyBytes,
				"error_response":   resp.Body(),
				"target_endpoint":  path,
			}
			agentPayloadBytes, _ := json.Marshal(agentPayload)

			// The AI call happens only on the first failure and is published to
			// the translation cache in-memory, so subsequent requests never wait.
			agentClient := &http.Client{Timeout: 20 * time.Second}
			agentResp, err := agentClient.Post(agentURL, "application/json", bytes.NewBuffer(agentPayloadBytes))
			if err == nil {
				defer agentResp.Body.Close()
				var aiResult map[string]interface{}
				if err := json.NewDecoder(agentResp.Body).Decode(&aiResult); err == nil {
					if patch, ok := aiResult["json_patch"].(map[string]interface{}); ok {
						rule := make(map[string]string)
						for k, v := range patch {
							rule[k] = v.(string)
						}

						fmt.Println("[PROXY] 🔧 Applying AI fix and replaying request to target API...")
						newBodyBytes := applyTranslation(bodyBytes, rule)

						// Replay the EXACT failing request with the translation applied.
						// This is the verification step: does the rule actually heal?
						retryReq := fasthttp.AcquireRequest()
						defer fasthttp.ReleaseRequest(retryReq)
						
						retryReq.Header.SetMethod(c.Method())
						retryReq.SetRequestURI(targetURL + path)
						c.Request().Header.VisitAll(func(k, v []byte) {
							retryReq.Header.SetBytesKV(k, v)
						})
						retryReq.SetBody(newBodyBytes)
						
						retryResp := fasthttp.AcquireResponse()
						defer fasthttp.ReleaseResponse(retryResp)
						
						replayStatus := 0
						verified := false
						if err := client.Do(retryReq, retryResp); err == nil {
							replayStatus = retryResp.StatusCode()
							verified = replayStatus < 400
						}

						ruleJson, _ := json.Marshal(rule)

						// Ground-truth index entry + PR + verification comment (async).
						go recordHealEvent(path, bodyBytes, newBodyBytes, rule, verified, replayStatus)

						if verified {
							fmt.Printf("[PROXY] ✅ Request self-healed and REPLAY-VERIFIED (HTTP %d)!\n", replayStatus)
							setCacheRule(path, rule)
							pushTelemetryEvent(c.Method()+" "+path, "HEALED", string(ruleJson), verified)

							c.Status(retryResp.StatusCode())
							retryResp.Header.VisitAll(func(k, v []byte) {
								c.Set(string(k), string(v))
							})
							return c.Send(retryResp.Body())
						}

						fmt.Printf("[PROXY] ⚠️ Translation rule did NOT verify on replay (HTTP %d). Not caching.\n", replayStatus)
						pushTelemetryEvent(c.Method()+" "+path, "ERROR", "rule failed verification: HTTP "+strconv.Itoa(replayStatus), false)
					}
				}
			} else {
				fmt.Println("[PROXY] Failed to reach AI Agent:", err)
				pushTelemetryEvent(c.Method()+" "+path, "ERROR", err.Error(), false)
			}
		}

		// Normal flow
		c.Status(resp.StatusCode())
		resp.Header.VisitAll(func(k, v []byte) {
			c.Set(string(k), string(v))
		})
		return c.Send(resp.Body())
	})

	fmt.Println("[PROXY] 🚀 Fiber Proxy listening on :8080")
	log.Fatal(app.Listen(":8080"))
}
