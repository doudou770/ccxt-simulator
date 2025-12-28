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
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
)

// InitLogger initializes the file-based logging system
// Logs are saved in the logs folder, rotated daily and when size exceeds 10MB
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

	// Setup info logger with rotation
	infoLogFile := &lumberjack.Logger{
		Filename:   filepath.Join(absLogDir, fmt.Sprintf("app-%s.log", currentDate)),
		MaxSize:    10, // 10 MB
		MaxBackups: 30, // Keep 30 old files
		MaxAge:     30, // 30 days
		Compress:   true,
		LocalTime:  true,
	}

	// Setup error logger with rotation
	errorLogFile := &lumberjack.Logger{
		Filename:   filepath.Join(absLogDir, fmt.Sprintf("error-%s.log", currentDate)),
		MaxSize:    10, // 10 MB
		MaxBackups: 30,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}

	// Setup debug logger with rotation
	debugLogFile := &lumberjack.Logger{
		Filename:   filepath.Join(absLogDir, fmt.Sprintf("debug-%s.log", currentDate)),
		MaxSize:    10, // 10 MB
		MaxBackups: 7,  // Keep 7 old files for debug
		MaxAge:     7,  // 7 days
		Compress:   true,
		LocalTime:  true,
	}

	// Create loggers that write to both file and stdout
	infoLogger = log.New(io.MultiWriter(os.Stdout, infoLogFile), "", log.LstdFlags)
	errorLogger = log.New(io.MultiWriter(os.Stderr, errorLogFile, infoLogFile), "", log.LstdFlags)
	debugLogger = log.New(io.MultiWriter(os.Stdout, debugLogFile), "", log.LstdFlags)

	// Also set the default logger to use file output
	log.SetOutput(io.MultiWriter(os.Stdout, infoLogFile))
	log.SetFlags(log.LstdFlags)

	// Force write to create log files immediately
	infoLogger.Printf("[INFO] Logger initialized, log directory: %s", absLogDir)
	infoLogger.Printf("[INFO] Log files: app-%s.log, error-%s.log, debug-%s.log", currentDate, currentDate, currentDate)

	return nil
}

// LogInfo logs info level messages
func LogInfo(format string, v ...interface{}) {
	if infoLogger != nil {
		infoLogger.Printf("[INFO] "+format, v...)
	} else {
		log.Printf("[INFO] "+format, v...)
	}
}

// LogError logs error level messages
func LogError(format string, v ...interface{}) {
	if errorLogger != nil {
		errorLogger.Printf("[ERROR] "+format, v...)
	} else {
		log.Printf("[ERROR] "+format, v...)
	}
}

// LogDebug logs debug level messages
func LogDebug(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogger.Printf("[DEBUG] "+format, v...)
	} else {
		log.Printf("[DEBUG] "+format, v...)
	}
}

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
			queryPreview := rawQuery
			if len(queryPreview) > 500 {
				queryPreview = queryPreview[:500] + "..."
			}
			LogError("%s %s?%s | status=%d | latency=%v | ip=%s | body=%s",
				method, path, queryPreview, statusCode, latency, clientIP, bodyPreview)
		} else {
			// Success response - brief log
			LogInfo("%s %s | status=%d | latency=%v | ip=%s",
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
		LogDebug("--> %s %s", method, path)
		if rawQuery != "" {
			LogDebug("    Query: %s", rawQuery)
		}
		if len(bodyBytes) > 0 {
			bodyStr := string(bodyBytes)
			if len(bodyStr) > 1000 {
				bodyStr = bodyStr[:1000] + "..."
			}
			LogDebug("    Body: %s", bodyStr)
		}

		// Log headers for exchange requests
		if apiKey := c.GetHeader("X-MBX-APIKEY"); apiKey != "" && len(apiKey) >= 8 {
			LogDebug("    X-MBX-APIKEY: %s***", apiKey[:8])
		}
		if apiKey := c.GetHeader("OK-ACCESS-KEY"); apiKey != "" && len(apiKey) >= 8 {
			LogDebug("    OK-ACCESS-KEY: %s***", apiKey[:8])
		}
		if apiKey := c.GetHeader("X-BAPI-API-KEY"); apiKey != "" && len(apiKey) >= 8 {
			LogDebug("    X-BAPI-API-KEY: %s***", apiKey[:8])
		}

		c.Next()

		// Log response status
		LogDebug("<-- %s %s | status=%d", method, path, c.Writer.Status())
	}
}
