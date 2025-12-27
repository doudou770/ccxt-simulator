package okx

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
	okxWSURL             = "wss://ws.okx.com:8443/ws/v5/public"
	okxRestURL           = "https://www.okx.com"
	pingInterval         = 25 * time.Second
	reconnectDelay       = 5 * time.Second
	maxReconnectAttempts = 10
)

// Client is an OKX WebSocket client
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

// NewClient creates a new OKX WebSocket client
func NewClient() *Client {
	return &Client{
		wsURL:      okxWSURL,
		restURL:    okxRestURL,
		symbols:    make(map[string]*exchange.SymbolInfo),
		subscribed: make(map[string]bool),
	}
}

// ExchangeName returns the exchange name
func (c *Client) ExchangeName() string {
	return "okx"
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
		log.Printf("[OKX] Warning: failed to load symbol info: %v", err)
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
		return fmt.Errorf("failed to connect to OKX WebSocket: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.reconnectAttempts = 0

	log.Printf("[OKX] WebSocket connected")

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

	args := make([]map[string]string, len(symbols))
	for i, symbol := range symbols {
		args[i] = map[string]string{
			"channel": "mark-price",
			"instId":  c.convertSymbol(symbol),
		}
	}

	msg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	c.connMux.RLock()
	err := c.conn.WriteJSON(msg)
	c.connMux.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Printf("[OKX] Subscribed to %d symbols", len(symbols))
	return nil
}

// Unsubscribe unsubscribes from price updates
func (c *Client) Unsubscribe(symbols []string) error {
	c.subscribedMux.Lock()
	for _, symbol := range symbols {
		delete(c.subscribed, c.convertSymbol(symbol))
	}
	c.subscribedMux.Unlock()

	if !c.IsConnected() {
		return nil
	}

	args := make([]map[string]string, len(symbols))
	for i, symbol := range symbols {
		args[i] = map[string]string{
			"channel": "mark-price",
			"instId":  c.convertSymbol(symbol),
		}
	}

	msg := map[string]interface{}{
		"op":   "unsubscribe",
		"args": args,
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
				log.Printf("[OKX] WebSocket error: %v", err)
			}
			c.handleDisconnect()
			continue
		}

		c.handleMessage(message)
	}
}

func (c *Client) handleMessage(message []byte) {
	var data struct {
		Arg struct {
			Channel string `json:"channel"`
			InstId  string `json:"instId"`
		} `json:"arg"`
		Data []struct {
			MarkPx string `json:"markPx"`
			Ts     string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &data); err != nil {
		return
	}

	if data.Arg.Channel != "mark-price" || len(data.Data) == 0 {
		return
	}

	price, _ := strconv.ParseFloat(data.Data[0].MarkPx, 64)
	ts, _ := strconv.ParseInt(data.Data[0].Ts, 10, 64)

	// Convert OKX symbol back to standard format
	symbol := c.convertToStandardSymbol(data.Arg.InstId)

	update := exchange.PriceUpdate{
		Exchange:  "okx",
		Symbol:    symbol,
		Price:     price,
		Timestamp: ts,
	}

	c.subMux.RLock()
	subscriber := c.subscriber
	c.subMux.RUnlock()

	if subscriber != nil {
		subscriber.OnPriceUpdate(update)
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
		log.Printf("[OKX] Attempting reconnect %d/%d", c.reconnectAttempts, maxReconnectAttempts)

		if err := c.connect(); err != nil {
			log.Printf("[OKX] Reconnect failed: %v", err)
			continue
		}

		return
	}

	log.Printf("[OKX] Max reconnect attempts reached")
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

			if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
				log.Printf("[OKX] Ping failed: %v", err)
			}
		}
	}
}

func (c *Client) loadSymbolInfo() error {
	resp, err := http.Get(c.restURL + "/api/v5/public/instruments?instType=SWAP")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			InstId   string `json:"instId"`
			BaseCcy  string `json:"baseCcy"`
			QuoteCcy string `json:"quoteCcy"`
			TickSz   string `json:"tickSz"`
			LotSz    string `json:"lotSz"`
			MinSz    string `json:"minSz"`
			CtVal    string `json:"ctVal"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.symbolsMux.Lock()
	defer c.symbolsMux.Unlock()

	for _, s := range result.Data {
		tickSize, _ := strconv.ParseFloat(s.TickSz, 64)
		stepSize, _ := strconv.ParseFloat(s.LotSz, 64)
		minQty, _ := strconv.ParseFloat(s.MinSz, 64)

		info := &exchange.SymbolInfo{
			Symbol:     c.convertToStandardSymbol(s.InstId),
			BaseAsset:  s.BaseCcy,
			QuoteAsset: s.QuoteCcy,
			TickSize:   tickSize,
			StepSize:   stepSize,
			MinQty:     minQty,
		}

		c.symbols[info.Symbol] = info
	}

	log.Printf("[OKX] Loaded %d symbols", len(c.symbols))
	return nil
}

// convertSymbol converts standard symbol (BTCUSDT) to OKX format (BTC-USDT-SWAP)
func (c *Client) convertSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return base + "-USDT-SWAP"
	}
	return symbol
}

// convertToStandardSymbol converts OKX format to standard symbol
func (c *Client) convertToStandardSymbol(okxSymbol string) string {
	// BTC-USDT-SWAP -> BTCUSDT
	parts := strings.Split(okxSymbol, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return okxSymbol
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

	log.Printf("[OKX] WebSocket closed")
	return nil
}

// GetCurrentPrice returns the current price
func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	instId := c.convertSymbol(symbol)
	resp, err := http.Get(fmt.Sprintf("%s/api/v5/market/mark-price?instId=%s", c.restURL, instId))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			MarkPx string `json:"markPx"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Data) == 0 {
		return 0, fmt.Errorf("no price data")
	}

	return strconv.ParseFloat(result.Data[0].MarkPx, 64)
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
	return 0.0005, 0.0002
}
