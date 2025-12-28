package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/service"
	"github.com/ccxt-simulator/pkg/crypto"
	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyAccount is the key for account in gin context
	ContextKeyAccount = "exchange_account"
	// ContextKeyAPISecret is the key for API secret in gin context
	ContextKeyAPISecret = "api_secret"
)

// ExchangeAuthConfig holds configuration for exchange authentication
type ExchangeAuthConfig struct {
	AESKey string
}

// BinanceAuthMiddleware creates authentication middleware for Binance API
func BinanceAuthMiddleware(accountService *service.AccountService, aesKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get API key from header
		apiKey := c.GetHeader("X-MBX-APIKEY")
		if apiKey == "" {
			c.JSON(401, gin.H{
				"code": -2015,
				"msg":  "Invalid API-key, IP, or permissions for action.",
			})
			c.Abort()
			return
		}

		// Find account by API key
		account, err := accountService.GetAccountByAPIKey(apiKey)
		if err != nil {
			c.JSON(401, gin.H{
				"code": -2015,
				"msg":  "Invalid API-key, IP, or permissions for action.",
			})
			c.Abort()
			return
		}

		// Verify this is a Binance account
		if account.ExchangeType != models.ExchangeBinance {
			c.JSON(401, gin.H{
				"code": -2015,
				"msg":  "API key is not for Binance.",
			})
			c.Abort()
			return
		}

		// Decrypt API secret
		apiSecret, err := crypto.DecryptAES(account.APISecretEncrypted, aesKey)
		if err != nil {
			c.JSON(500, gin.H{
				"code": -1,
				"msg":  "Internal error.",
			})
			c.Abort()
			return
		}

		// Verify signature for POST/PUT/DELETE requests
		if c.Request.Method != "GET" || c.Query("signature") != "" {
			if !verifyBinanceSignature(c, apiSecret) {
				log.Printf("[BINANCE] Signature verification failed: method=%s path=%s query=%s",
					c.Request.Method, c.Request.URL.Path, c.Request.URL.RawQuery)
				c.JSON(401, gin.H{
					"code": -1022,
					"msg":  "Signature for this request is not valid.",
				})
				c.Abort()
				return
			}
		}

		// Verify timestamp (within 5 minutes)
		if timestamp := c.Query("timestamp"); timestamp != "" {
			ts, err := strconv.ParseInt(timestamp, 10, 64)
			if err != nil || abs(time.Now().UnixMilli()-ts) > 300000 {
				c.JSON(400, gin.H{
					"code": -1021,
					"msg":  "Timestamp for this request was 1000ms ahead of the server's time.",
				})
				c.Abort()
				return
			}
		}

		// Store account in context
		c.Set(ContextKeyAccount, account)
		c.Set(ContextKeyAPISecret, apiSecret)
		c.Next()
	}
}

// verifyBinanceSignature verifies the HMAC-SHA256 signature for Binance
func verifyBinanceSignature(c *gin.Context, apiSecret string) bool {
	providedSig := c.Query("signature")
	if providedSig == "" {
		// Try form data
		providedSig = c.PostForm("signature")
	}
	if providedSig == "" {
		log.Printf("[BINANCE] No signature provided")
		return false
	}

	var queryString string

	if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "DELETE" {
		// For POST/PUT/DELETE, check if body is form or JSON
		contentType := c.GetHeader("Content-Type")

		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// Form data
			c.Request.ParseForm()
			params := make(url.Values)
			for key, values := range c.Request.PostForm {
				if key != "signature" {
					params[key] = values
				}
			}
			// Also include query parameters
			for key, values := range c.Request.URL.Query() {
				if key != "signature" {
					params[key] = values
				}
			}
			queryString = params.Encode()
		} else {
			// For JSON body or no body, use query string
			rawQuery := c.Request.URL.RawQuery
			parts := strings.Split(rawQuery, "&")
			var filtered []string
			for _, part := range parts {
				if part != "" && !strings.HasPrefix(part, "signature=") {
					filtered = append(filtered, part)
				}
			}
			queryString = strings.Join(filtered, "&")
		}
	} else {
		// For GET requests, use the raw query string (preserving order)
		rawQuery := c.Request.URL.RawQuery
		// Remove signature parameter from raw query
		parts := strings.Split(rawQuery, "&")
		var filtered []string
		for _, part := range parts {
			if part != "" && !strings.HasPrefix(part, "signature=") {
				filtered = append(filtered, part)
			}
		}
		queryString = strings.Join(filtered, "&")
	}

	// Calculate signature
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(queryString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Debug logging
	if providedSig != expectedSig {
		log.Printf("[BINANCE] Signature mismatch: method=%s path=%s", c.Request.Method, c.Request.URL.Path)
		log.Printf("[BINANCE] Query string to sign: %s", queryString)
		log.Printf("[BINANCE] Expected signature: %s", expectedSig)
		log.Printf("[BINANCE] Provided signature: %s", providedSig)
		return false
	}

	return true
}

// OKXAuthMiddleware creates authentication middleware for OKX API
func OKXAuthMiddleware(accountService *service.AccountService, aesKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get headers
		apiKey := c.GetHeader("OK-ACCESS-KEY")
		timestamp := c.GetHeader("OK-ACCESS-TIMESTAMP")
		sign := c.GetHeader("OK-ACCESS-SIGN")
		passphrase := c.GetHeader("OK-ACCESS-PASSPHRASE")

		if apiKey == "" || timestamp == "" || sign == "" || passphrase == "" {
			c.JSON(401, gin.H{
				"code": "50111",
				"msg":  "Invalid credentials.",
			})
			c.Abort()
			return
		}

		// Find account by API key
		account, err := accountService.GetAccountByAPIKey(apiKey)
		if err != nil {
			c.JSON(401, gin.H{
				"code": "50111",
				"msg":  "Invalid API Key.",
			})
			c.Abort()
			return
		}

		// Verify this is an OKX account
		if account.ExchangeType != models.ExchangeOKX {
			c.JSON(401, gin.H{
				"code": "50111",
				"msg":  "API key is not for OKX.",
			})
			c.Abort()
			return
		}

		// Decrypt API secret and passphrase
		apiSecret, err := crypto.DecryptAES(account.APISecretEncrypted, aesKey)
		if err != nil {
			c.JSON(500, gin.H{
				"code": "50000",
				"msg":  "Internal error.",
			})
			c.Abort()
			return
		}

		storedPassphrase, err := crypto.DecryptAES(account.PassphraseEncrypted, aesKey)
		if err != nil {
			c.JSON(500, gin.H{
				"code": "50000",
				"msg":  "Internal error.",
			})
			c.Abort()
			return
		}

		// Verify passphrase
		if passphrase != storedPassphrase {
			c.JSON(401, gin.H{
				"code": "50113",
				"msg":  "Invalid passphrase.",
			})
			c.Abort()
			return
		}

		// Verify signature
		if !verifyOKXSignature(c, timestamp, apiSecret) {
			c.JSON(401, gin.H{
				"code": "50113",
				"msg":  "Invalid signature.",
			})
			c.Abort()
			return
		}

		// Store account in context
		c.Set(ContextKeyAccount, account)
		c.Set(ContextKeyAPISecret, apiSecret)
		c.Next()
	}
}

// verifyOKXSignature verifies the HMAC-SHA256 + Base64 signature for OKX
func verifyOKXSignature(c *gin.Context, timestamp, apiSecret string) bool {
	sign := c.GetHeader("OK-ACCESS-SIGN")
	if sign == "" {
		return false
	}

	// Build prehash string: timestamp + method + requestPath + body
	method := c.Request.Method
	requestPath := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		requestPath += "?" + c.Request.URL.RawQuery
	}

	var body string
	if method == "POST" || method == "PUT" {
		// Read body
		bodyBytes, _ := c.GetRawData()
		body = string(bodyBytes)
	}

	prehash := timestamp + method + requestPath + body

	// Calculate signature
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(prehash))
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sign), []byte(expectedSig))
}

// BybitAuthMiddleware creates authentication middleware for Bybit API
func BybitAuthMiddleware(accountService *service.AccountService, aesKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get headers
		apiKey := c.GetHeader("X-BAPI-API-KEY")
		timestamp := c.GetHeader("X-BAPI-TIMESTAMP")
		sign := c.GetHeader("X-BAPI-SIGN")
		recvWindow := c.GetHeader("X-BAPI-RECV-WINDOW")

		if apiKey == "" || timestamp == "" || sign == "" {
			c.JSON(10003, gin.H{
				"retCode": 10003,
				"retMsg":  "Invalid apiKey.",
			})
			c.Abort()
			return
		}

		// Find account by API key
		account, err := accountService.GetAccountByAPIKey(apiKey)
		if err != nil {
			c.JSON(401, gin.H{
				"retCode": 10003,
				"retMsg":  "Invalid apiKey.",
			})
			c.Abort()
			return
		}

		// Verify this is a Bybit account
		if account.ExchangeType != models.ExchangeBybit {
			c.JSON(401, gin.H{
				"retCode": 10003,
				"retMsg":  "API key is not for Bybit.",
			})
			c.Abort()
			return
		}

		// Decrypt API secret
		apiSecret, err := crypto.DecryptAES(account.APISecretEncrypted, aesKey)
		if err != nil {
			c.JSON(500, gin.H{
				"retCode": 10000,
				"retMsg":  "Internal error.",
			})
			c.Abort()
			return
		}

		// Verify signature
		if !verifyBybitSignature(c, apiKey, timestamp, recvWindow, apiSecret) {
			c.JSON(401, gin.H{
				"retCode": 10004,
				"retMsg":  "Invalid sign.",
			})
			c.Abort()
			return
		}

		// Store account in context
		c.Set(ContextKeyAccount, account)
		c.Set(ContextKeyAPISecret, apiSecret)
		c.Next()
	}
}

// verifyBybitSignature verifies the HMAC-SHA256 signature for Bybit
func verifyBybitSignature(c *gin.Context, apiKey, timestamp, recvWindow, apiSecret string) bool {
	sign := c.GetHeader("X-BAPI-SIGN")
	if sign == "" {
		return false
	}

	// Build param string
	var paramStr string
	if c.Request.Method == "GET" {
		paramStr = c.Request.URL.RawQuery
	} else {
		bodyBytes, _ := c.GetRawData()
		paramStr = string(bodyBytes)
	}

	// Prehash: timestamp + apiKey + recvWindow + paramStr
	prehash := timestamp + apiKey + recvWindow + paramStr

	// Calculate signature
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(prehash))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sign), []byte(expectedSig))
}

// Helper functions

func sortedEncode(params url.Values) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var pairs []string
	for _, key := range keys {
		for _, value := range params[key] {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(pairs, "&")
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// GetAccount retrieves the account from gin context
func GetAccount(c *gin.Context) *models.Account {
	account, exists := c.Get(ContextKeyAccount)
	if !exists {
		return nil
	}
	return account.(*models.Account)
}

// GetAPISecret retrieves the API secret from gin context
func GetAPISecret(c *gin.Context) string {
	secret, exists := c.Get(ContextKeyAPISecret)
	if !exists {
		return ""
	}
	return secret.(string)
}

// BitgetAuthMiddleware creates authentication middleware for Bitget API
func BitgetAuthMiddleware(accountService *service.AccountService, aesKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get headers
		apiKey := c.GetHeader("ACCESS-KEY")
		timestamp := c.GetHeader("ACCESS-TIMESTAMP")
		sign := c.GetHeader("ACCESS-SIGN")
		passphrase := c.GetHeader("ACCESS-PASSPHRASE")

		if apiKey == "" || timestamp == "" || sign == "" {
			c.JSON(401, gin.H{
				"code": "40001",
				"msg":  "Invalid API credentials.",
			})
			c.Abort()
			return
		}

		// Find account by API key
		account, err := accountService.GetAccountByAPIKey(apiKey)
		if err != nil {
			c.JSON(401, gin.H{
				"code": "40001",
				"msg":  "Invalid API Key.",
			})
			c.Abort()
			return
		}

		// Verify this is a Bitget account
		if account.ExchangeType != models.ExchangeBitget {
			c.JSON(401, gin.H{
				"code": "40001",
				"msg":  "API key is not for Bitget.",
			})
			c.Abort()
			return
		}

		// Decrypt API secret
		apiSecret, err := crypto.DecryptAES(account.APISecretEncrypted, aesKey)
		if err != nil {
			c.JSON(500, gin.H{
				"code": "50000",
				"msg":  "Internal error.",
			})
			c.Abort()
			return
		}

		// Verify passphrase if provided
		if passphrase != "" && account.PassphraseEncrypted != "" {
			storedPassphrase, err := crypto.DecryptAES(account.PassphraseEncrypted, aesKey)
			if err != nil || passphrase != storedPassphrase {
				c.JSON(401, gin.H{
					"code": "40001",
					"msg":  "Invalid passphrase.",
				})
				c.Abort()
				return
			}
		}

		// Verify signature
		if !verifyBitgetSignature(c, timestamp, apiSecret) {
			c.JSON(401, gin.H{
				"code": "40009",
				"msg":  "Invalid signature.",
			})
			c.Abort()
			return
		}

		// Store account in context
		c.Set(ContextKeyAccount, account)
		c.Set(ContextKeyAPISecret, apiSecret)
		c.Next()
	}
}

// verifyBitgetSignature verifies the HMAC-SHA256 + Base64 signature for Bitget
func verifyBitgetSignature(c *gin.Context, timestamp, apiSecret string) bool {
	sign := c.GetHeader("ACCESS-SIGN")
	if sign == "" {
		return false
	}

	// Build prehash string: timestamp + method + requestPath + body
	method := c.Request.Method
	requestPath := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		requestPath += "?" + c.Request.URL.RawQuery
	}

	var body string
	if method == "POST" || method == "PUT" {
		bodyBytes, _ := c.GetRawData()
		body = string(bodyBytes)
	}

	prehash := timestamp + method + requestPath + body

	// Calculate signature
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(prehash))
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sign), []byte(expectedSig))
}

// HyperliquidAuthMiddleware creates authentication middleware for Hyperliquid API
func HyperliquidAuthMiddleware(accountService *service.AccountService, aesKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Hyperliquid uses wallet signature authentication
		// For simulation, we use a simplified API key approach

		// Get API key from header or query
		apiKey := c.GetHeader("HL-API-KEY")
		if apiKey == "" {
			apiKey = c.Query("apiKey")
		}

		if apiKey == "" {
			c.JSON(401, gin.H{"error": "Missing API key"})
			c.Abort()
			return
		}

		// Find account by API key
		account, err := accountService.GetAccountByAPIKey(apiKey)
		if err != nil {
			c.JSON(401, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		// Verify this is a Hyperliquid account
		if account.ExchangeType != models.ExchangeHyperliquid {
			c.JSON(401, gin.H{"error": "API key is not for Hyperliquid"})
			c.Abort()
			return
		}

		// Decrypt API secret (used as wallet private key in real implementation)
		apiSecret, err := crypto.DecryptAES(account.APISecretEncrypted, aesKey)
		if err != nil {
			c.JSON(500, gin.H{"error": "Internal error"})
			c.Abort()
			return
		}

		// Store account in context
		c.Set(ContextKeyAccount, account)
		c.Set(ContextKeyAPISecret, apiSecret)
		c.Next()
	}
}
