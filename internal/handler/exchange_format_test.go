package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// BinanceBalanceResponse represents Binance /fapi/v2/balance response
type BinanceBalanceResponse []struct {
	AccountAlias       string `json:"accountAlias"`
	Asset              string `json:"asset"`
	Balance            string `json:"balance"`
	CrossWalletBalance string `json:"crossWalletBalance"`
	CrossUnPnl         string `json:"crossUnPnl"`
	AvailableBalance   string `json:"availableBalance"`
	MaxWithdrawAmount  string `json:"maxWithdrawAmount"`
	MarginAvailable    bool   `json:"marginAvailable"`
	UpdateTime         int64  `json:"updateTime"`
}

// BinancePositionRiskResponse represents Binance /fapi/v2/positionRisk response
type BinancePositionRiskResponse []struct {
	Symbol           string `json:"symbol"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	MarkPrice        string `json:"markPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	LiquidationPrice string `json:"liquidationPrice"`
	Leverage         string `json:"leverage"`
	MarginType       string `json:"marginType"`
	IsolatedMargin   string `json:"isolatedMargin"`
	IsAutoAddMargin  string `json:"isAutoAddMargin"`
	PositionSide     string `json:"positionSide"`
	UpdateTime       int64  `json:"updateTime"`
}

// BinanceOrderResponse represents Binance /fapi/v1/order response
type BinanceOrderResponse struct {
	OrderId       uint   `json:"orderId"`
	Symbol        string `json:"symbol"`
	Status        string `json:"status"`
	ClientOrderId string `json:"clientOrderId"`
	Price         string `json:"price"`
	AvgPrice      string `json:"avgPrice"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	PositionSide  string `json:"positionSide"`
	ReduceOnly    bool   `json:"reduceOnly"`
	ClosePosition bool   `json:"closePosition"`
	UpdateTime    int64  `json:"updateTime"`
}

// BinanceErrorResponse represents Binance error response
type BinanceErrorResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// OKXResponse represents OKX standard response wrapper
type OKXResponse struct {
	Code string        `json:"code"`
	Msg  string        `json:"msg"`
	Data []interface{} `json:"data"`
}

// OKXBalanceData represents OKX balance data structure
type OKXBalanceData struct {
	TotalEq     string `json:"totalEq"`
	IsoEq       string `json:"isoEq"`
	AdjEq       string `json:"adjEq"`
	OrdFroz     string `json:"ordFroz"`
	Imr         string `json:"imr"`
	Mmr         string `json:"mmr"`
	NotionalUsd string `json:"notionalUsd"`
	MgnRatio    string `json:"mgnRatio"`
	UTime       string `json:"uTime"`
}

// BybitResponse represents Bybit standard response wrapper
type BybitResponse struct {
	RetCode int         `json:"retCode"`
	RetMsg  string      `json:"retMsg"`
	Result  interface{} `json:"result"`
	Time    int64       `json:"time"`
}

// BitgetResponse represents Bitget standard response wrapper
type BitgetResponse struct {
	Code        string      `json:"code"`
	Msg         string      `json:"msg"`
	RequestTime int64       `json:"requestTime"`
	Data        interface{} `json:"data"`
}

// HyperliquidMetaResponse represents Hyperliquid meta response
type HyperliquidMetaResponse struct {
	Universe []struct {
		Name       string `json:"name"`
		SzDecimals int    `json:"szDecimals"`
	} `json:"universe"`
}

// TestBinanceBalanceResponseFormat tests Binance balance API response format
func TestBinanceBalanceResponseFormat(t *testing.T) {
	// Simulate a response from our server
	mockResponse := `[{
		"accountAlias": "SgsR",
		"asset": "USDT",
		"balance": "10000.00000000",
		"crossWalletBalance": "10000.00000000",
		"crossUnPnl": "0.00000000",
		"availableBalance": "9000.00000000",
		"maxWithdrawAmount": "9000.00000000",
		"marginAvailable": true,
		"updateTime": 1703683200000
	}]`

	var response BinanceBalanceResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Len(t, response, 1, "Should have one balance entry")
	assert.Equal(t, "USDT", response[0].Asset, "Asset should be USDT")
	assert.Equal(t, "10000.00000000", response[0].Balance, "Balance should be string with 8 decimals")
	assert.True(t, response[0].MarginAvailable, "MarginAvailable should be true")
	assert.Greater(t, response[0].UpdateTime, int64(0), "UpdateTime should be positive")
}

// TestBinancePositionRiskResponseFormat tests Binance position API response format
func TestBinancePositionRiskResponseFormat(t *testing.T) {
	mockResponse := `[{
		"symbol": "BTCUSDT",
		"positionAmt": "0.01000000",
		"entryPrice": "87399.20000000",
		"markPrice": "87500.00000000",
		"unRealizedProfit": "1.00800000",
		"liquidationPrice": "79008.87680000",
		"leverage": "10",
		"marginType": "cross",
		"isolatedMargin": "87.39920000",
		"isAutoAddMargin": "false",
		"positionSide": "LONG",
		"updateTime": 1703683200000
	}]`

	var response BinancePositionRiskResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Len(t, response, 1, "Should have one position")
	assert.Equal(t, "BTCUSDT", response[0].Symbol, "Symbol should be BTCUSDT")
	assert.Equal(t, "LONG", response[0].PositionSide, "PositionSide should be LONG")
	assert.Equal(t, "10", response[0].Leverage, "Leverage should be string")
	assert.Equal(t, "cross", response[0].MarginType, "MarginType should be cross")
}

// TestBinanceOrderResponseFormat tests Binance order API response format
func TestBinanceOrderResponseFormat(t *testing.T) {
	mockResponse := `{
		"orderId": 1,
		"symbol": "BTCUSDT",
		"status": "FILLED",
		"clientOrderId": "2f7f71be-fd0c-4f35-a166-0839e2d9f65a",
		"price": "0.00000000",
		"avgPrice": "87399.20000000",
		"origQty": "0.01000000",
		"executedQty": "0.01000000",
		"type": "MARKET",
		"side": "BUY",
		"positionSide": "LONG",
		"reduceOnly": false,
		"closePosition": false,
		"updateTime": 1703683200000
	}`

	var response BinanceOrderResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, uint(1), response.OrderId, "OrderId should be numeric")
	assert.Equal(t, "BTCUSDT", response.Symbol, "Symbol should be BTCUSDT")
	assert.Equal(t, "FILLED", response.Status, "Status should be FILLED")
	assert.Equal(t, "MARKET", response.Type, "Type should be MARKET")
	assert.Equal(t, "BUY", response.Side, "Side should be BUY")
	assert.Equal(t, "LONG", response.PositionSide, "PositionSide should be LONG")
}

// TestBinanceErrorResponseFormat tests Binance error response format
func TestBinanceErrorResponseFormat(t *testing.T) {
	testCases := []struct {
		name     string
		response string
		code     int
		msg      string
	}{
		{
			name:     "Insufficient Balance",
			response: `{"code":-2019,"msg":"Margin is insufficient."}`,
			code:     -2019,
			msg:      "Margin is insufficient.",
		},
		{
			name:     "Invalid Symbol",
			response: `{"code":-1121,"msg":"Invalid symbol."}`,
			code:     -1121,
			msg:      "Invalid symbol.",
		},
		{
			name:     "Invalid API Key",
			response: `{"code":-2015,"msg":"Invalid API-key, IP, or permissions for action."}`,
			code:     -2015,
			msg:      "Invalid API-key, IP, or permissions for action.",
		},
		{
			name:     "Invalid Signature",
			response: `{"code":-1022,"msg":"Signature for this request is not valid."}`,
			code:     -1022,
			msg:      "Signature for this request is not valid.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var response BinanceErrorResponse
			err := json.Unmarshal([]byte(tc.response), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.code, response.Code)
			assert.Equal(t, tc.msg, response.Msg)
		})
	}
}

// TestOKXBalanceResponseFormat tests OKX balance API response format
func TestOKXBalanceResponseFormat(t *testing.T) {
	mockResponse := `{
		"code": "0",
		"msg": "",
		"data": [{
			"totalEq": "10000.00000000",
			"isoEq": "0",
			"adjEq": "10000.00000000",
			"ordFroz": "0",
			"imr": "100.00000000",
			"mmr": "0",
			"notionalUsd": "1000.00000000",
			"mgnRatio": "999",
			"uTime": "1703683200000",
			"details": [{
				"ccy": "USDT",
				"eq": "10000.00000000",
				"cashBal": "10000.00000000",
				"availBal": "9900.00000000",
				"frozenBal": "100.00000000",
				"upl": "0.00000000"
			}]
		}]
	}`

	var response OKXResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, "0", response.Code, "Code should be '0' for success")
	assert.Equal(t, "", response.Msg, "Msg should be empty for success")
	assert.Len(t, response.Data, 1, "Should have one data entry")
}

// TestOKXErrorResponseFormat tests OKX error response format
func TestOKXErrorResponseFormat(t *testing.T) {
	testCases := []struct {
		name     string
		response string
		code     string
		msg      string
	}{
		{
			name:     "Invalid API Key",
			response: `{"code":"50111","msg":"API key is invalid","data":[]}`,
			code:     "50111",
			msg:      "API key is invalid",
		},
		{
			name:     "Insufficient Balance",
			response: `{"code":"51008","msg":"Order placement failed due to insufficient balance","data":[]}`,
			code:     "51008",
			msg:      "Order placement failed due to insufficient balance",
		},
		{
			name:     "Invalid Passphrase",
			response: `{"code":"50113","msg":"Invalid passphrase.","data":[]}`,
			code:     "50113",
			msg:      "Invalid passphrase.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var response OKXResponse
			err := json.Unmarshal([]byte(tc.response), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.code, response.Code)
			assert.Equal(t, tc.msg, response.Msg)
		})
	}
}

// TestBybitWalletBalanceResponseFormat tests Bybit wallet balance API response format
func TestBybitWalletBalanceResponseFormat(t *testing.T) {
	mockResponse := `{
		"retCode": 0,
		"retMsg": "OK",
		"result": {
			"list": [{
				"accountType": "UNIFIED",
				"totalEquity": "10000.00000000",
				"totalWalletBalance": "10000.00000000",
				"totalAvailableBalance": "9900.00000000",
				"totalPerpUPL": "0.00000000",
				"totalInitialMargin": "100.00000000",
				"coin": [{
					"coin": "USDT",
					"equity": "10000.00000000",
					"walletBalance": "10000.00000000"
				}]
			}]
		},
		"time": 1703683200000
	}`

	var response BybitResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, 0, response.RetCode, "RetCode should be 0 for success")
	assert.Equal(t, "OK", response.RetMsg, "RetMsg should be OK")
	assert.NotNil(t, response.Result, "Result should not be nil")
	assert.Greater(t, response.Time, int64(0), "Time should be positive")
}

// TestBybitErrorResponseFormat tests Bybit error response format
func TestBybitErrorResponseFormat(t *testing.T) {
	testCases := []struct {
		name     string
		response string
		retCode  int
		retMsg   string
	}{
		{
			name:     "Invalid API Key",
			response: `{"retCode":10003,"retMsg":"Invalid apiKey.","result":{},"time":1703683200000}`,
			retCode:  10003,
			retMsg:   "Invalid apiKey.",
		},
		{
			name:     "Invalid Signature",
			response: `{"retCode":10004,"retMsg":"Invalid sign.","result":{},"time":1703683200000}`,
			retCode:  10004,
			retMsg:   "Invalid sign.",
		},
		{
			name:     "Insufficient Balance",
			response: `{"retCode":110007,"retMsg":"Insufficient account balance","result":{},"time":1703683200000}`,
			retCode:  110007,
			retMsg:   "Insufficient account balance",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var response BybitResponse
			err := json.Unmarshal([]byte(tc.response), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.retCode, response.RetCode)
			assert.Equal(t, tc.retMsg, response.RetMsg)
		})
	}
}

// TestBitgetAccountResponseFormat tests Bitget account API response format
func TestBitgetAccountResponseFormat(t *testing.T) {
	mockResponse := `{
		"code": "00000",
		"msg": "success",
		"requestTime": 1703683200000,
		"data": {
			"marginCoin": "USDT",
			"locked": "0",
			"available": "9900.00000000",
			"equity": "10000.00000000",
			"accountBalance": "10000.00000000",
			"unrealizedPL": "0.00000000"
		}
	}`

	var response BitgetResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, "00000", response.Code, "Code should be '00000' for success")
	assert.Equal(t, "success", response.Msg, "Msg should be 'success'")
	assert.Greater(t, response.RequestTime, int64(0), "RequestTime should be positive")
	assert.NotNil(t, response.Data, "Data should not be nil")
}

// TestBitgetErrorResponseFormat tests Bitget error response format
func TestBitgetErrorResponseFormat(t *testing.T) {
	testCases := []struct {
		name     string
		response string
		code     string
		msg      string
	}{
		{
			name:     "Invalid API Key",
			response: `{"code":"40001","msg":"Invalid API key","requestTime":1703683200000,"data":null}`,
			code:     "40001",
			msg:      "Invalid API key",
		},
		{
			name:     "Insufficient Balance",
			response: `{"code":"45110","msg":"Insufficient balance","requestTime":1703683200000,"data":null}`,
			code:     "45110",
			msg:      "Insufficient balance",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var response BitgetResponse
			err := json.Unmarshal([]byte(tc.response), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.code, response.Code)
			assert.Equal(t, tc.msg, response.Msg)
		})
	}
}

// TestHyperliquidMetaResponseFormat tests Hyperliquid meta API response format
func TestHyperliquidMetaResponseFormat(t *testing.T) {
	mockResponse := `{
		"universe": [
			{"name": "BTC", "szDecimals": 5},
			{"name": "ETH", "szDecimals": 4},
			{"name": "SOL", "szDecimals": 2}
		]
	}`

	var response HyperliquidMetaResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Len(t, response.Universe, 3, "Should have 3 assets")
	assert.Equal(t, "BTC", response.Universe[0].Name, "First asset should be BTC")
	assert.Equal(t, 5, response.Universe[0].SzDecimals, "BTC szDecimals should be 5")
}

// TestHyperliquidAllMidsResponseFormat tests Hyperliquid allMids API response format
func TestHyperliquidAllMidsResponseFormat(t *testing.T) {
	mockResponse := `{
		"BTC": "87500.12345678",
		"ETH": "2250.12345678",
		"SOL": "100.12345678"
	}`

	var response map[string]string
	err := json.Unmarshal([]byte(mockResponse), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Contains(t, response, "BTC", "Should contain BTC")
	assert.Contains(t, response, "ETH", "Should contain ETH")
	assert.NotEmpty(t, response["BTC"], "BTC price should not be empty")
}

// TestBinanceRequestFormat tests Binance request format validation
func TestBinanceRequestFormat(t *testing.T) {
	testCases := []struct {
		name        string
		params      map[string]string
		shouldError bool
	}{
		{
			name: "Valid Market Order",
			params: map[string]string{
				"symbol":       "BTCUSDT",
				"side":         "BUY",
				"positionSide": "LONG",
				"type":         "MARKET",
				"quantity":     "0.01",
				"timestamp":    "1703683200000",
				"signature":    "abc123",
			},
			shouldError: false,
		},
		{
			name: "Valid Limit Order",
			params: map[string]string{
				"symbol":       "BTCUSDT",
				"side":         "BUY",
				"positionSide": "LONG",
				"type":         "LIMIT",
				"quantity":     "0.01",
				"price":        "50000",
				"timeInForce":  "GTC",
				"timestamp":    "1703683200000",
				"signature":    "abc123",
			},
			shouldError: false,
		},
		{
			name: "Missing Symbol",
			params: map[string]string{
				"side":      "BUY",
				"type":      "MARKET",
				"quantity":  "0.01",
				"timestamp": "1703683200000",
				"signature": "abc123",
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate required fields
			hasError := false
			requiredFields := []string{"symbol", "side", "type", "quantity", "timestamp", "signature"}
			for _, field := range requiredFields {
				if _, ok := tc.params[field]; !ok {
					hasError = true
					break
				}
			}
			assert.Equal(t, tc.shouldError, hasError, "Validation result mismatch")
		})
	}
}

// TestOKXRequestFormat tests OKX request format validation
func TestOKXRequestFormat(t *testing.T) {
	testCases := []struct {
		name  string
		body  map[string]interface{}
		valid bool
	}{
		{
			name: "Valid Order Request",
			body: map[string]interface{}{
				"instId":  "BTC-USDT-SWAP",
				"tdMode":  "cross",
				"side":    "buy",
				"ordType": "market",
				"sz":      "1",
			},
			valid: true,
		},
		{
			name: "Valid Limit Order",
			body: map[string]interface{}{
				"instId":  "BTC-USDT-SWAP",
				"tdMode":  "cross",
				"side":    "buy",
				"ordType": "limit",
				"px":      "50000",
				"sz":      "1",
			},
			valid: true,
		},
		{
			name: "Missing instId",
			body: map[string]interface{}{
				"tdMode":  "cross",
				"side":    "buy",
				"ordType": "market",
				"sz":      "1",
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate required fields
			requiredFields := []string{"instId", "tdMode", "side", "ordType", "sz"}
			valid := true
			for _, field := range requiredFields {
				if _, ok := tc.body[field]; !ok {
					valid = false
					break
				}
			}
			assert.Equal(t, tc.valid, valid, "Validation result mismatch")
		})
	}
}

// MockHandler for integration testing
type MockHandler struct {
	router *gin.Engine
}

func NewMockHandler() *MockHandler {
	router := gin.New()

	// Add mock routes
	router.GET("/fapi/v2/balance", func(c *gin.Context) {
		c.JSON(200, []gin.H{
			{
				"accountAlias":       "SgsR",
				"asset":              "USDT",
				"balance":            "10000.00000000",
				"crossWalletBalance": "10000.00000000",
				"crossUnPnl":         "0.00000000",
				"availableBalance":   "9900.00000000",
				"maxWithdrawAmount":  "9900.00000000",
				"marginAvailable":    true,
				"updateTime":         1703683200000,
			},
		})
	})

	router.POST("/api/v5/trade/order", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code": "0",
			"msg":  "",
			"data": []gin.H{
				{
					"ordId":   "12345",
					"clOrdId": "test-order-123",
					"tag":     "",
					"sCode":   "0",
					"sMsg":    "",
				},
			},
		})
	})

	router.GET("/v5/account/wallet-balance", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"retCode": 0,
			"retMsg":  "OK",
			"result": gin.H{
				"list": []gin.H{
					{
						"accountType":           "UNIFIED",
						"totalEquity":           "10000.00000000",
						"totalWalletBalance":    "10000.00000000",
						"totalAvailableBalance": "9900.00000000",
						"totalPerpUPL":          "0.00000000",
					},
				},
			},
			"time": 1703683200000,
		})
	})

	router.POST("/info", func(c *gin.Context) {
		var req struct {
			Type string `json:"type"`
		}
		c.BindJSON(&req)

		if req.Type == "meta" {
			c.JSON(200, gin.H{
				"universe": []gin.H{
					{"name": "BTC", "szDecimals": 5},
					{"name": "ETH", "szDecimals": 4},
				},
			})
		} else if req.Type == "allMids" {
			c.JSON(200, gin.H{
				"BTC": "87500.00000000",
				"ETH": "2250.00000000",
			})
		}
	})

	return &MockHandler{router: router}
}

// TestBinanceBalanceIntegration tests Binance balance endpoint integration
func TestBinanceBalanceIntegration(t *testing.T) {
	handler := NewMockHandler()

	req, _ := http.NewRequest("GET", "/fapi/v2/balance", nil)
	req.Header.Set("X-MBX-APIKEY", "test-api-key")

	w := httptest.NewRecorder()
	handler.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response BinanceBalanceResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 1)
	assert.Equal(t, "USDT", response[0].Asset)
	assert.Equal(t, "10000.00000000", response[0].Balance)
}

// TestOKXOrderIntegration tests OKX order endpoint integration
func TestOKXOrderIntegration(t *testing.T) {
	handler := NewMockHandler()

	body := `{"instId":"BTC-USDT-SWAP","tdMode":"cross","side":"buy","ordType":"market","sz":"1"}`
	req, _ := http.NewRequest("POST", "/api/v5/trade/order", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", "test-api-key")

	w := httptest.NewRecorder()
	handler.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response OKXResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "0", response.Code)
	assert.Len(t, response.Data, 1)
}

// TestBybitBalanceIntegration tests Bybit balance endpoint integration
func TestBybitBalanceIntegration(t *testing.T) {
	handler := NewMockHandler()

	req, _ := http.NewRequest("GET", "/v5/account/wallet-balance?accountType=UNIFIED", nil)
	req.Header.Set("X-BAPI-API-KEY", "test-api-key")

	w := httptest.NewRecorder()
	handler.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response BybitResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 0, response.RetCode)
	assert.Equal(t, "OK", response.RetMsg)
}

// TestHyperliquidMetaIntegration tests Hyperliquid meta endpoint integration
func TestHyperliquidMetaIntegration(t *testing.T) {
	handler := NewMockHandler()

	body := `{"type":"meta"}`
	req, _ := http.NewRequest("POST", "/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response HyperliquidMetaResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Universe, 2)
	assert.Equal(t, "BTC", response.Universe[0].Name)
}

// TestHyperliquidAllMidsIntegration tests Hyperliquid allMids endpoint integration
func TestHyperliquidAllMidsIntegration(t *testing.T) {
	handler := NewMockHandler()

	body := `{"type":"allMids"}`
	req, _ := http.NewRequest("POST", "/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "BTC")
	assert.Contains(t, response, "ETH")
}
