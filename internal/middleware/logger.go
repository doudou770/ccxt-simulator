package middleware

import (
	"bytes"
	"io"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLoggerMiddleware logs all incoming requests with details
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()

		// Read and restore request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Get request details
		method := c.Request.Method
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery
		clientIP := c.ClientIP()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(startTime)
		statusCode := c.Writer.Status()

		// Determine log level based on status code
		if statusCode >= 400 {
			// Error response - log with more details
			bodyPreview := string(bodyBytes)
			if len(bodyPreview) > 500 {
				bodyPreview = bodyPreview[:500] + "..."
			}
			if rawQuery != "" && len(rawQuery) > 500 {
				rawQuery = rawQuery[:500] + "..."
			}
			log.Printf("[ERROR] %s %s?%s | status=%d | latency=%v | ip=%s | body=%s",
				method, path, rawQuery, statusCode, latency, clientIP, bodyPreview)
		} else {
			// Success response - brief log
			log.Printf("[INFO] %s %s | status=%d | latency=%v | ip=%s",
				method, path, statusCode, latency, clientIP)
		}
	}
}

// DebugLoggerMiddleware logs detailed request info for debugging
func DebugLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and restore request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		method := c.Request.Method
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		// Log request
		log.Printf("[DEBUG] --> %s %s", method, path)
		if rawQuery != "" {
			log.Printf("[DEBUG]     Query: %s", rawQuery)
		}
		if len(bodyBytes) > 0 {
			bodyStr := string(bodyBytes)
			if len(bodyStr) > 1000 {
				bodyStr = bodyStr[:1000] + "..."
			}
			log.Printf("[DEBUG]     Body: %s", bodyStr)
		}

		// Log headers for exchange requests
		if apiKey := c.GetHeader("X-MBX-APIKEY"); apiKey != "" {
			log.Printf("[DEBUG]     X-MBX-APIKEY: %s***", apiKey[:8])
		}
		if apiKey := c.GetHeader("OK-ACCESS-KEY"); apiKey != "" {
			log.Printf("[DEBUG]     OK-ACCESS-KEY: %s***", apiKey[:8])
		}
		if apiKey := c.GetHeader("X-BAPI-API-KEY"); apiKey != "" {
			log.Printf("[DEBUG]     X-BAPI-API-KEY: %s***", apiKey[:8])
		}

		c.Next()

		// Log response status
		log.Printf("[DEBUG] <-- %s %s | status=%d", method, path, c.Writer.Status())
	}
}
