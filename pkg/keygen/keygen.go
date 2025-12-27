package keygen

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

const (
	alphaNumeric      = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	upperAlphaNumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lowerAlphaNumeric = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// APIKeySet contains the generated API credentials
type APIKeySet struct {
	APIKey     string
	APISecret  string
	Passphrase string // Only for OKX
}

// GenerateAPIKey generates API key and secret based on exchange type
func GenerateAPIKey(exchangeType string) (*APIKeySet, error) {
	switch strings.ToLower(exchangeType) {
	case "binance":
		return generateBinanceKeys()
	case "okx":
		return generateOKXKeys()
	case "bybit":
		return generateBybitKeys()
	case "bitget":
		return generateBitgetKeys()
	case "hyperliquid":
		return generateHyperliquidKeys()
	default:
		return nil, fmt.Errorf("unsupported exchange type: %s", exchangeType)
	}
}

// generateBinanceKeys generates Binance-style API keys
// API Key: 64 characters alphanumeric
// API Secret: 64 characters alphanumeric
func generateBinanceKeys() (*APIKeySet, error) {
	apiKey, err := randomString(64, alphaNumeric)
	if err != nil {
		return nil, err
	}

	apiSecret, err := randomString(64, alphaNumeric)
	if err != nil {
		return nil, err
	}

	return &APIKeySet{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}, nil
}

// generateOKXKeys generates OKX-style API keys
// API Key: UUID format (36 characters with hyphens)
// API Secret: 32 characters Base64
// Passphrase: 16 characters alphanumeric
func generateOKXKeys() (*APIKeySet, error) {
	apiKey := uuid.New().String()

	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	apiSecret := base64.StdEncoding.EncodeToString(secretBytes)

	passphrase, err := randomString(16, alphaNumeric)
	if err != nil {
		return nil, err
	}

	return &APIKeySet{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		Passphrase: passphrase,
	}, nil
}

// generateBybitKeys generates Bybit-style API keys
// API Key: 18 characters uppercase alphanumeric
// API Secret: 36 characters lowercase hex
func generateBybitKeys() (*APIKeySet, error) {
	apiKey, err := randomString(18, upperAlphaNumeric)
	if err != nil {
		return nil, err
	}

	secretBytes := make([]byte, 18)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	apiSecret := hex.EncodeToString(secretBytes)

	return &APIKeySet{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}, nil
}

// generateBitgetKeys generates Bitget-style API keys
// API Key: 32 characters lowercase alphanumeric
// API Secret: 64 characters lowercase alphanumeric
func generateBitgetKeys() (*APIKeySet, error) {
	apiKey, err := randomString(32, lowerAlphaNumeric)
	if err != nil {
		return nil, err
	}

	apiSecret, err := randomString(64, lowerAlphaNumeric)
	if err != nil {
		return nil, err
	}

	return &APIKeySet{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}, nil
}

// generateHyperliquidKeys generates Hyperliquid-style API keys
// API Key: Ethereum address format (42 characters, 0x prefix)
// API Secret: 64 characters hex (private key format)
func generateHyperliquidKeys() (*APIKeySet, error) {
	// Generate random 20 bytes for address
	addrBytes := make([]byte, 20)
	if _, err := rand.Read(addrBytes); err != nil {
		return nil, err
	}
	apiKey := "0x" + hex.EncodeToString(addrBytes)

	// Generate random 32 bytes for private key
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	apiSecret := hex.EncodeToString(secretBytes)

	return &APIKeySet{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}, nil
}

// randomString generates a random string of given length from the given charset
func randomString(length int, charset string) (string, error) {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}
