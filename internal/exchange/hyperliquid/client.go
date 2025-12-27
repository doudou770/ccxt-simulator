package hyperliquid

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
	hyperliquidWSURL     = "wss://api.hyperliquid.xyz/ws"
	hyperliquidRestURL   = "https://api.hyperliquid.xyz"
	pingInterval         = 30 * time.Second
	reconnectDelay       = 5 * time.Second
	maxReconnectAttempts = 10
)

// Client is a Hyperliquid WebSocket client
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

// NewClient creates a new Hyperliquid WebSocket client
func NewClient() *Client {
	return &Client{
		wsURL:      hyperliquidWSURL,
		restURL:    hyperliquidRestURL,
		symbols:    make(map[string]*exchange.SymbolInfo),
		subscribed: make(map[string]bool),
	}
}

// ExchangeName returns the exchange name
func (c *Client) ExchangeName() string {
	return "hyperliquid"
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

	if err := c.loadSymbolInfo(); err != nil {
		log.Printf("[Hyperliquid] Warning: failed to load symbol info: %v", err)
	}

	if err := c.connect(); err != nil {
		return err
	}

	c.wg.Add(1)
	go c.messageLoop()

	c.wg.Add(1)
	go c.pingLoop()

	return nil
}

func (c *Client) connect() error {
	c.connMux.Lock()
	defer c.connMux.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Hyperliquid WebSocket: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.reconnectAttempts = 0

	log.Printf("[Hyperliquid] WebSocket connected")

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

// Subscribe subscribes to price updates
func (c *Client) Subscribe(symbols []string) error {
	c.subscribedMux.Lock()
	for _, symbol := range symbols {
		c.subscribed[c.convertSymbol(symbol)] = true
	}
	c.subscribedMux.Unlock()

	return c.subscribe(symbols)
}

func (c *Client) subscribe(symbols []string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Hyperliquid uses a different subscription format
	// Subscribe to all mids (mark prices)
	msg := map[string]interface{}{
		"method": "subscribe",
		"subscription": map[string]interface{}{
			"type": "allMids",
		},
	}

	c.connMux.RLock()
	err := c.conn.WriteJSON(msg)
	c.connMux.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Printf("[Hyperliquid] Subscribed to %d symbols", len(symbols))
	return nil
}

// Unsubscribe unsubscribes from price updates
func (c *Client) Unsubscribe(symbols []string) error {
	c.subscribedMux.Lock()
	for _, symbol := range symbols {
		delete(c.subscribed, c.convertSymbol(symbol))
	}
	c.subscribedMux.Unlock()

	return nil
}

// SetSubscriber sets the price update subscriber
func (c *Client) SetSubscriber(subscriber exchange.PriceSubscriber) {
	c.subMux.Lock()
	defer c.subMux.Unlock()
	c.subscriber = subscriber
}

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
				log.Printf("[Hyperliquid] WebSocket error: %v", err)
			}
			c.handleDisconnect()
			continue
		}

		c.handleMessage(message)
	}
}

func (c *Client) handleMessage(message []byte) {
	var data struct {
		Channel string `json:"channel"`
		Data    struct {
			Mids map[string]string `json:"mids"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &data); err != nil {
		return
	}

	if data.Channel != "allMids" || data.Data.Mids == nil {
		return
	}

	ts := time.Now().UnixMilli()

	c.subMux.RLock()
	subscriber := c.subscriber
	c.subMux.RUnlock()

	for symbol, priceStr := range data.Data.Mids {
		price, _ := strconv.ParseFloat(priceStr, 64)

		// Check if we're subscribed to this symbol
		c.subscribedMux.RLock()
		_, subscribed := c.subscribed[symbol]
		c.subscribedMux.RUnlock()

		if !subscribed && len(c.subscribed) > 0 {
			continue
		}

		update := exchange.PriceUpdate{
			Exchange:  "hyperliquid",
			Symbol:    c.convertToStandardSymbol(symbol),
			Price:     price,
			Timestamp: ts,
		}

		if subscriber != nil {
			subscriber.OnPriceUpdate(update)
		}
	}
}

func (c *Client) handleDisconnect() {
	c.connMux.Lock()
	c.isConnected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMux.Unlock()

	for c.reconnectAttempts < maxReconnectAttempts {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}

		c.reconnectAttempts++
		log.Printf("[Hyperliquid] Attempting reconnect %d/%d", c.reconnectAttempts, maxReconnectAttempts)

		if err := c.connect(); err != nil {
			log.Printf("[Hyperliquid] Reconnect failed: %v", err)
			continue
		}

		return
	}

	log.Printf("[Hyperliquid] Max reconnect attempts reached")
}

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

			msg := map[string]string{"method": "ping"}
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("[Hyperliquid] Ping failed: %v", err)
			}
		}
	}
}

func (c *Client) loadSymbolInfo() error {
	// Hyperliquid uses POST for metadata
	resp, err := http.Post(c.restURL+"/info", "application/json",
		strings.NewReader(`{"type": "meta"}`))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Universe []struct {
			Name       string `json:"name"`
			SzDecimals int    `json:"szDecimals"`
		} `json:"universe"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.symbolsMux.Lock()
	defer c.symbolsMux.Unlock()

	for _, s := range result.Universe {
		info := &exchange.SymbolInfo{
			Symbol:            s.Name,
			BaseAsset:         s.Name,
			QuoteAsset:        "USD",
			QuantityPrecision: s.SzDecimals,
			PricePrecision:    6,
		}

		c.symbols[s.Name] = info
	}

	log.Printf("[Hyperliquid] Loaded %d symbols", len(c.symbols))
	return nil
}

// convertSymbol converts standard symbol (BTCUSDT) to Hyperliquid format (BTC)
func (c *Client) convertSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	// Remove USDT suffix
	if strings.HasSuffix(symbol, "USDT") {
		return symbol[:len(symbol)-4]
	}
	if strings.HasSuffix(symbol, "USD") {
		return symbol[:len(symbol)-3]
	}
	return symbol
}

// convertToStandardSymbol converts Hyperliquid format to standard symbol
func (c *Client) convertToStandardSymbol(symbol string) string {
	// BTC -> BTCUSDT
	if !strings.HasSuffix(symbol, "USDT") && !strings.HasSuffix(symbol, "USD") {
		return symbol + "USDT"
	}
	return symbol
}

// GetSymbolInfo returns trading pair information
func (c *Client) GetSymbolInfo(symbol string) (*exchange.SymbolInfo, error) {
	c.symbolsMux.RLock()
	defer c.symbolsMux.RUnlock()

	hlSymbol := c.convertSymbol(symbol)
	info, ok := c.symbols[hlSymbol]
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
		symbols = append(symbols, c.convertToStandardSymbol(symbol))
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

	log.Printf("[Hyperliquid] WebSocket closed")
	return nil
}

// GetCurrentPrice returns the current price
func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	hlSymbol := c.convertSymbol(symbol)

	resp, err := http.Post(c.restURL+"/info", "application/json",
		strings.NewReader(`{"type": "allMids"}`))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	priceStr, ok := result[hlSymbol]
	if !ok {
		return 0, fmt.Errorf("symbol not found: %s", symbol)
	}

	return strconv.ParseFloat(priceStr, 64)
}

// ValidateSymbol checks if a symbol is valid
func (c *Client) ValidateSymbol(symbol string) bool {
	c.symbolsMux.RLock()
	defer c.symbolsMux.RUnlock()
	hlSymbol := c.convertSymbol(symbol)
	_, ok := c.symbols[hlSymbol]
	return ok
}

// GetMaintenanceMarginRate returns the maintenance margin rate
func (c *Client) GetMaintenanceMarginRate(positionValue float64) float64 {
	return 0.03 // Hyperliquid uses ~3% MMR
}

// GetFeeRate returns taker and maker fee rates
func (c *Client) GetFeeRate() (takerFee, makerFee float64) {
	return 0.00035, 0.0001 // Hyperliquid has lower fees
}
