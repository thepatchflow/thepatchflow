package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
)

var (
	rdb              *redis.Client
	ctx              = context.Background()
	translationCache = make(map[string]map[string]string)
	redisAvailable   = false
)

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

	app.All("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/dashboard") {
			return c.Next()
		}

		bodyBytes := c.Body()

		// 1. Check Cache
		if rule, ok := getCacheRule(path); ok {
			fmt.Println("[PROXY] ⚡ Cache Hit! Applying translation rule:", rule)
			bodyBytes = applyTranslation(bodyBytes, rule)
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

			agentResp, err := http.Post(agentURL, "application/json", bytes.NewBuffer(agentPayloadBytes))
			if err == nil {
				defer agentResp.Body.Close()
				var aiResult map[string]interface{}
				if err := json.NewDecoder(agentResp.Body).Decode(&aiResult); err == nil {
					if patch, ok := aiResult["json_patch"].(map[string]interface{}); ok {
						rule := make(map[string]string)
						for k, v := range patch {
							rule[k] = v.(string)
						}

						fmt.Println("[PROXY] 🔧 Applying AI fix and caching rule...")
						setCacheRule(path, rule)

						newBodyBytes := applyTranslation(bodyBytes, rule)

						// Replay request
						fmt.Println("[PROXY] 🔁 Replaying request to target API...")
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
						
						if err := client.Do(retryReq, retryResp); err == nil {
							fmt.Println("[PROXY] ✅ Request successfully self-healed!")
							c.Status(retryResp.StatusCode())
							retryResp.Header.VisitAll(func(k, v []byte) {
								c.Set(string(k), string(v))
							})
							return c.Send(retryResp.Body())
						}
					}
				}
			} else {
				fmt.Println("[PROXY] Failed to reach AI Agent:", err)
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
