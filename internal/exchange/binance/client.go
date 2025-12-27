package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ccxt-simulator/internal/exchange"
	"github.com/gorilla/websocket"
)

const (
	binanceWSURL         = "wss://fstream.binance.com/ws"
	binanceRestURL       = "https://fapi.binance.com"
	pingInterval         = 30 * time.Second
	reconnectDelay       = 5 * time.Second
	maxReconnectAttempts = 10
)

// Client is a Binance Futures WebSocket client
type Client struct {
	wsURL       string
	restURL     string
	conn        *websocket.Conn
	connMux     sync.RWMutex
	isConnected bool

	subscriber exchange.PriceSubscriber
	subMux     sync.RWMutex

	symbols    map[string]*exchange.SymbolInfo
	symbolsMux sync.RWMutex

	subscribed    map[string]bool
	subscribedMux sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	reconnectAttempts int
}

// NewClient creates a new Binance WebSocket client
func NewClient() *Client {
	return &Client{
		wsURL:      binanceWSURL,
		restURL:    binanceRestURL,
		symbols:    make(map[string]*exchange.SymbolInfo),
		subscribed: make(map[string]bool),
	}
}

// ExchangeName returns the exchange name
func (c *Client) ExchangeName() string {
	return "binance"
}

// IsConnected returns whether the WebSocket is connected
func (c *Client) IsConnected() bool {
	c.connMux.RLock()
	defer c.connMux.RUnlock()
	return c.isConnected
}

// Connect establishes WebSocket connection
func (c *Client) Connect(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Load symbol info first
	if err := c.loadSymbolInfo(); err != nil {
		log.Printf("[Binance] Warning: failed to load symbol info: %v", err)
	}

	if err := c.connect(); err != nil {
		return err
	}

	// Start message handler
	c.wg.Add(1)
	go c.messageLoop()

	// Start ping loop
	c.wg.Add(1)
	go c.pingLoop()

	return nil
}

// connect establishes the WebSocket connection
func (c *Client) connect() error {
	c.connMux.Lock()
	defer c.connMux.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Binance WebSocket: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.reconnectAttempts = 0

	log.Printf("[Binance] WebSocket connected")

	// Resubscribe to previous symbols
	c.subscribedMux.RLock()
	symbols := make([]string, 0, len(c.subscribed))
	for symbol := range c.subscribed {
		symbols = append(symbols, symbol)
	}
	c.subscribedMux.RUnlock()

	if len(symbols) > 0 {
		go c.subscribe(symbols)
	}

	return nil
}

// Subscribe subscribes to price updates for given symbols
func (c *Client) Subscribe(symbols []string) error {
	c.subscribedMux.Lock()
	for _, symbol := range symbols {
		c.subscribed[strings.ToUpper(symbol)] = true
	}
	c.subscribedMux.Unlock()

	return c.subscribe(symbols)
}

// subscribe sends subscription request
func (c *Client) subscribe(symbols []string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Build stream names
	streams := make([]string, len(symbols))
	for i, symbol := range symbols {
		streams[i] = strings.ToLower(symbol) + "@markPrice@1s"
	}

	msg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	c.connMux.RLock()
	err := c.conn.WriteJSON(msg)
	c.connMux.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Printf("[Binance] Subscribed to %d symbols", len(symbols))
	return nil
}

// Unsubscribe unsubscribes from price updates
func (c *Client) Unsubscribe(symbols []string) error {
	c.subscribedMux.Lock()
	for _, symbol := range symbols {
		delete(c.subscribed, strings.ToUpper(symbol))
	}
	c.subscribedMux.Unlock()

	if !c.IsConnected() {
		return nil
	}

	streams := make([]string, len(symbols))
	for i, symbol := range symbols {
		streams[i] = strings.ToLower(symbol) + "@markPrice@1s"
	}

	msg := map[string]interface{}{
		"method": "UNSUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	c.connMux.RLock()
	err := c.conn.WriteJSON(msg)
	c.connMux.RUnlock()

	return err
}

// SetSubscriber sets the price update subscriber
func (c *Client) SetSubscriber(subscriber exchange.PriceSubscriber) {
	c.subMux.Lock()
	defer c.subMux.Unlock()
	c.subscriber = subscriber
}

// messageLoop handles incoming WebSocket messages
func (c *Client) messageLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.connMux.RLock()
		conn := c.conn
		c.connMux.RUnlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Binance] WebSocket error: %v", err)
			}
			c.handleDisconnect()
			continue
		}

		c.handleMessage(message)
	}
}

// handleMessage processes a WebSocket message
func (c *Client) handleMessage(message []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		return
	}

	// Check if it's a mark price update
	eventType, ok := data["e"].(string)
	if !ok || eventType != "markPriceUpdate" {
		return
	}

	symbol, _ := data["s"].(string)
	priceStr, _ := data["p"].(string)
	timeMs, _ := data["E"].(float64)

	price, _ := strconv.ParseFloat(priceStr, 64)

	update := exchange.PriceUpdate{
		Exchange:  "binance",
		Symbol:    symbol,
		Price:     price,
		Timestamp: int64(timeMs),
	}

	c.subMux.RLock()
	subscriber := c.subscriber
	c.subMux.RUnlock()

	if subscriber != nil {
		subscriber.OnPriceUpdate(update)
	}
}

// handleDisconnect handles WebSocket disconnection
func (c *Client) handleDisconnect() {
	c.connMux.Lock()
	c.isConnected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMux.Unlock()

	// Attempt reconnect
	for c.reconnectAttempts < maxReconnectAttempts {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}

		c.reconnectAttempts++
		log.Printf("[Binance] Attempting reconnect %d/%d", c.reconnectAttempts, maxReconnectAttempts)

		if err := c.connect(); err != nil {
			log.Printf("[Binance] Reconnect failed: %v", err)
			continue
		}

		return
	}

	log.Printf("[Binance] Max reconnect attempts reached")
}

// pingLoop sends periodic ping messages
func (c *Client) pingLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.connMux.RLock()
			conn := c.conn
			isConnected := c.isConnected
			c.connMux.RUnlock()

			if !isConnected || conn == nil {
				continue
			}

			if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				log.Printf("[Binance] Ping failed: %v", err)
			}
		}
	}
}

// loadSymbolInfo loads trading pair information from REST API
func (c *Client) loadSymbolInfo() error {
	resp, err := http.Get(c.restURL + "/fapi/v1/exchangeInfo")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Symbols []struct {
			Symbol            string `json:"symbol"`
			BaseAsset         string `json:"baseAsset"`
			QuoteAsset        string `json:"quoteAsset"`
			PricePrecision    int    `json:"pricePrecision"`
			QuantityPrecision int    `json:"quantityPrecision"`
			Filters           []struct {
				FilterType string `json:"filterType"`
				MinQty     string `json:"minQty,omitempty"`
				MaxQty     string `json:"maxQty,omitempty"`
				StepSize   string `json:"stepSize,omitempty"`
				TickSize   string `json:"tickSize,omitempty"`
				Notional   string `json:"notional,omitempty"`
			} `json:"filters"`
		} `json:"symbols"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.symbolsMux.Lock()
	defer c.symbolsMux.Unlock()

	for _, s := range result.Symbols {
		info := &exchange.SymbolInfo{
			Symbol:            s.Symbol,
			BaseAsset:         s.BaseAsset,
			QuoteAsset:        s.QuoteAsset,
			PricePrecision:    s.PricePrecision,
			QuantityPrecision: s.QuantityPrecision,
		}

		for _, f := range s.Filters {
			switch f.FilterType {
			case "LOT_SIZE":
				info.MinQty, _ = strconv.ParseFloat(f.MinQty, 64)
				info.MaxQty, _ = strconv.ParseFloat(f.MaxQty, 64)
				info.StepSize, _ = strconv.ParseFloat(f.StepSize, 64)
			case "PRICE_FILTER":
				info.TickSize, _ = strconv.ParseFloat(f.TickSize, 64)
			case "MIN_NOTIONAL":
				info.MinNotional, _ = strconv.ParseFloat(f.Notional, 64)
			}
		}

		c.symbols[s.Symbol] = info
	}

	log.Printf("[Binance] Loaded %d symbols", len(c.symbols))
	return nil
}

// GetSymbolInfo returns trading pair information
func (c *Client) GetSymbolInfo(symbol string) (*exchange.SymbolInfo, error) {
	c.symbolsMux.RLock()
	defer c.symbolsMux.RUnlock()

	info, ok := c.symbols[strings.ToUpper(symbol)]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return info, nil
}

// GetAllSymbols returns all available trading symbols
func (c *Client) GetAllSymbols() ([]string, error) {
	c.symbolsMux.RLock()
	defer c.symbolsMux.RUnlock()

	symbols := make([]string, 0, len(c.symbols))
	for symbol := range c.symbols {
		symbols = append(symbols, symbol)
	}
	return symbols, nil
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}

	c.connMux.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.isConnected = false
	c.connMux.Unlock()

	c.wg.Wait()

	log.Printf("[Binance] WebSocket closed")
	return nil
}

// GetCurrentPrice returns the current price (from REST API as fallback)
func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	resp, err := http.Get(fmt.Sprintf("%s/fapi/v1/ticker/price?symbol=%s", c.restURL, strings.ToUpper(symbol)))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Price string `json:"price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return strconv.ParseFloat(result.Price, 64)
}

// ValidateSymbol checks if a symbol is valid
func (c *Client) ValidateSymbol(symbol string) bool {
	c.symbolsMux.RLock()
	defer c.symbolsMux.RUnlock()
	_, ok := c.symbols[strings.ToUpper(symbol)]
	return ok
}

// GetMaintenanceMarginRate returns the maintenance margin rate
func (c *Client) GetMaintenanceMarginRate(positionValue float64) float64 {
	// Binance tiered maintenance margin rates
	switch {
	case positionValue <= 50000:
		return 0.004
	case positionValue <= 250000:
		return 0.005
	case positionValue <= 1000000:
		return 0.01
	default:
		return 0.025
	}
}

// GetFeeRate returns taker and maker fee rates
func (c *Client) GetFeeRate() (takerFee, makerFee float64) {
	return 0.0004, 0.0002
}
