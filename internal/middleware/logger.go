package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	appLogger *log.Logger
)

// InitLogger initializes the file-based logging system
// Logs are saved in the logs folder as a single app.log file
func InitLogger(logDir string) error {
	// Get absolute path for log directory
	absLogDir, err := filepath.Abs(logDir)
	if err != nil {
		absLogDir = logDir
	}

	// Create logs directory if not exists
	if err := os.MkdirAll(absLogDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", absLogDir, err)
	}

	// Get current date for log file name
	currentDate := time.Now().Format("2006-01-02")

	// Setup single app logger with rotation
	appLogFile := &lumberjack.Logger{
		Filename:   filepath.Join(absLogDir, fmt.Sprintf("app-%s.log", currentDate)),
		MaxSize:    10, // 10 MB
		MaxBackups: 30, // Keep 30 old files
		MaxAge:     30, // 30 days
		Compress:   true,
		LocalTime:  true,
	}

	// Create logger that writes to both file and stdout
	appLogger = log.New(io.MultiWriter(os.Stdout, appLogFile), "", log.LstdFlags)

	// Also set the default logger to use file output
	log.SetOutput(io.MultiWriter(os.Stdout, appLogFile))
	log.SetFlags(log.LstdFlags)

	// Log initialization
	appLogger.Printf("[INFO] Logger initialized, log directory: %s", absLogDir)
	appLogger.Printf("[INFO] Log file: app-%s.log", currentDate)

	return nil
}

// LogInfo logs info level messages
func LogInfo(format string, v ...interface{}) {
	if appLogger != nil {
		appLogger.Printf("[INFO] "+format, v...)
	} else {
		log.Printf("[INFO] "+format, v...)
	}
}

// LogError logs error level messages
func LogError(format string, v ...interface{}) {
	if appLogger != nil {
		appLogger.Printf("[ERROR] "+format, v...)
	} else {
		log.Printf("[ERROR] "+format, v...)
	}
}

// LogDebug logs debug level messages
func LogDebug(format string, v ...interface{}) {
	if appLogger != nil {
		appLogger.Printf("[DEBUG] "+format, v...)
	} else {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// RequestLoggerMiddleware logs all incoming requests
// For GET requests: logs only the full URL with query parameters
// For other requests: logs basic info (full logging handled by TradingLoggerMiddleware)
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Build full URL
		fullURL := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			fullURL = fullURL + "?" + c.Request.URL.RawQuery
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(startTime)
		statusCode := c.Writer.Status()

		// Log format: METHOD URL | status | latency
		if statusCode >= 400 {
			LogError("%s %s | status=%d | latency=%v",
				c.Request.Method, fullURL, statusCode, latency)
		} else {
			LogInfo("%s %s | status=%d | latency=%v",
				c.Request.Method, fullURL, statusCode, latency)
		}
	}
}

// TradingLoggerMiddleware logs complete request details for trading operations
// Records: full URL with query, headers, and body
// Use this for: order creation, leverage setting, margin type, etc.
func TradingLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Read and restore request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Build full URL
		fullURL := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			fullURL = fullURL + "?" + c.Request.URL.RawQuery
		}

		// Build headers string (only relevant trading headers)
		var headersStr string
		relevantHeaders := []string{
			"X-MBX-APIKEY",         // Binance
			"OK-ACCESS-KEY",        // OKX
			"OK-ACCESS-SIGN",       // OKX
			"OK-ACCESS-TIMESTAMP",  // OKX
			"OK-ACCESS-PASSPHRASE", // OKX
			"X-BAPI-API-KEY",       // Bybit
			"X-BAPI-SIGN",          // Bybit
			"X-BAPI-TIMESTAMP",     // Bybit
			"ACCESS-KEY",           // Bitget
			"ACCESS-SIGN",          // Bitget
			"ACCESS-TIMESTAMP",     // Bitget
			"ACCESS-PASSPHRASE",    // Bitget
			"Content-Type",
		}

		var headerParts []string
		for _, h := range relevantHeaders {
			if val := c.GetHeader(h); val != "" {
				// Mask API keys (show first 8 chars only)
				if len(val) > 12 && (h == "X-MBX-APIKEY" || h == "OK-ACCESS-KEY" || h == "X-BAPI-API-KEY" || h == "ACCESS-KEY") {
					val = val[:8] + "***"
				}
				headerParts = append(headerParts, fmt.Sprintf("%s: %s", h, val))
			}
		}
		if len(headerParts) > 0 {
			headersStr = fmt.Sprintf("{%s}", joinStrings(headerParts, ", "))
		} else {
			headersStr = "{}"
		}

		// Body string
		bodyStr := string(bodyBytes)
		if bodyStr == "" {
			bodyStr = "(empty)"
		} else if len(bodyStr) > 1000 {
			bodyStr = bodyStr[:1000] + "..."
		}

		// Log trading request details
		LogInfo("====== TRADING REQUEST ======")
		LogInfo("TIME: %s", startTime.Format("2006-01-02 15:04:05.000"))
		LogInfo("URL: %s %s", c.Request.Method, fullURL)
		LogInfo("HEADERS: %s", headersStr)
		LogInfo("BODY: %s", bodyStr)

		// Process request
		c.Next()

		// Log response
		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		LogInfo("RESPONSE: status=%d | latency=%v", statusCode, latency)
		LogInfo("=============================")
	}
}

// joinStrings joins string slice with separator
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
