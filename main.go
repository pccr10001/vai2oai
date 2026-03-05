package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	upstreamBaseURL := os.Getenv("UPSTREAM_BASE_URL")
	upstreamAPIKey := os.Getenv("UPSTREAM_API_KEY")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if upstreamBaseURL == "" {
		log.Fatal("UPSTREAM_BASE_URL is not set")
	}

	r := gin.Default()

	// Catch-all handler for proxying
	r.Any("/*proxyPath", func(c *gin.Context) {
		proxyPath := c.Param("proxyPath")
		targetURL := fmt.Sprintf("%s%s", strings.TrimRight(upstreamBaseURL, "/"), proxyPath)

		// 1. Parse Request Body
		var bodyBytes []byte
		var err error

		if c.Request.Body != nil {
			bodyBytes, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
				return
			}
		}

		// 2. Modify Body if it's a JSON request containing "model"
		if len(bodyBytes) > 0 {
			var jsonBody map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
				if model, ok := jsonBody["model"].(string); ok {
					// Prepend vertex_ai/ if not already present (optional check, but good for safety)
					if !strings.HasPrefix(model, "vertex_ai/") {
						jsonBody["model"] = "vertex_ai/" + model
						log.Printf("Proxying model: %s -> %s", model, jsonBody["model"])

						newBodyBytes, err := json.Marshal(jsonBody)
						if err == nil {
							bodyBytes = newBodyBytes
						}
					}
				}
			}
		}

		// 3. Create Upstream Request
		req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upstream request"})
			return
		}

		// 4. Copy Headers
		for k, v := range c.Request.Header {
			// Skip headers that are hop-by-hop or likely to change
			if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Host") {
				continue
			}
			req.Header[k] = v
		}

		// Ensure upstream API key is set if provided, overriding client key if necessary
		// Or if the client provides it, maybe we just pass it through?
		// The requirement said "I have API Key", so we probably should use the one in ENV.
		if upstreamAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+upstreamAPIKey)
		}

		// 5. Execute Request
		client := &http.Client{
			Timeout: 10 * time.Minute,
		}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to upstream service"})
			return
		}
		defer resp.Body.Close()

		// 6. Copy Response Headers
		for k, v := range resp.Header {
			c.Writer.Header()[k] = v
		}
		c.Status(resp.StatusCode)

		// 7. Stream Response Body
		// Using Stream() to handle potentially large responses or SSE
		_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			log.Printf("Error copying response body: %v", err)
		}
	})

	log.Printf("Starting proxy server on port %s forwarding to %s", port, upstreamBaseURL)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
