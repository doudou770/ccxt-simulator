package bybit

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
	bybitWSURL           = "wss://stream.bybit.com/v5/public/linear"
	bybitRestURL         = "https://api.bybit.com"
	pingInterval         = 20 * time.Second
	reconnectDelay       = 5 * time.Second
	maxReconnectAttempts = 10
)

// Client is a Bybit WebSocket client
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

// NewClient creates a new Bybit WebSocket client
func NewClient() *Client {
	return &Client{
		wsURL:      bybitWSURL,
		restURL:    bybitRestURL,
		symbols:    make(map[string]*exchange.SymbolInfo),
		subscribed: make(map[string]bool),
	}
}

// ExchangeName returns the exchange name
func (c *Client) ExchangeName() string {
	return "bybit"
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
		log.Printf("[Bybit] Warning: failed to load symbol info: %v", err)
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
		return fmt.Errorf("failed to connect to Bybit WebSocket: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.reconnectAttempts = 0

	log.Printf("[Bybit] WebSocket connected")

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
		c.subscribed[strings.ToUpper(symbol)] = true
	}
	c.subscribedMux.Unlock()

	return c.subscribe(symbols)
}

func (c *Client) subscribe(symbols []string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	args := make([]string, len(symbols))
	for i, symbol := range symbols {
		args[i] = "tickers." + strings.ToUpper(symbol)
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

	log.Printf("[Bybit] Subscribed to %d symbols", len(symbols))
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

	args := make([]string, len(symbols))
	for i, symbol := range symbols {
		args[i] = "tickers." + strings.ToUpper(symbol)
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
				log.Printf("[Bybit] WebSocket error: %v", err)
			}
			c.handleDisconnect()
			continue
		}

		c.handleMessage(message)
	}
}

func (c *Client) handleMessage(message []byte) {
	var data struct {
		Topic string `json:"topic"`
		Data  struct {
			Symbol    string `json:"symbol"`
			MarkPrice string `json:"markPrice"`
			Bid1Price string `json:"bid1Price"`
			Ask1Price string `json:"ask1Price"`
		} `json:"data"`
		Ts int64 `json:"ts"`
	}

	if err := json.Unmarshal(message, &data); err != nil {
		return
	}

	if !strings.HasPrefix(data.Topic, "tickers.") {
		return
	}

	price, _ := strconv.ParseFloat(data.Data.MarkPrice, 64)
	bidPrice, _ := strconv.ParseFloat(data.Data.Bid1Price, 64)
	askPrice, _ := strconv.ParseFloat(data.Data.Ask1Price, 64)

	update := exchange.PriceUpdate{
		Exchange:  "bybit",
		Symbol:    data.Data.Symbol,
		Price:     price,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		Timestamp: data.Ts,
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
		log.Printf("[Bybit] Attempting reconnect %d/%d", c.reconnectAttempts, maxReconnectAttempts)

		if err := c.connect(); err != nil {
			log.Printf("[Bybit] Reconnect failed: %v", err)
			continue
		}

		return
	}

	log.Printf("[Bybit] Max reconnect attempts reached")
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

			msg := map[string]string{"op": "ping"}
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("[Bybit] Ping failed: %v", err)
			}
		}
	}
}

func (c *Client) loadSymbolInfo() error {
	resp, err := http.Get(c.restURL + "/v5/market/instruments-info?category=linear")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			List []struct {
				Symbol      string `json:"symbol"`
				BaseCoin    string `json:"baseCoin"`
				QuoteCoin   string `json:"quoteCoin"`
				PriceFilter struct {
					TickSize string `json:"tickSize"`
				} `json:"priceFilter"`
				LotSizeFilter struct {
					MinOrderQty string `json:"minOrderQty"`
					MaxOrderQty string `json:"maxOrderQty"`
					QtyStep     string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.symbolsMux.Lock()
	defer c.symbolsMux.Unlock()

	for _, s := range result.Result.List {
		tickSize, _ := strconv.ParseFloat(s.PriceFilter.TickSize, 64)
		minQty, _ := strconv.ParseFloat(s.LotSizeFilter.MinOrderQty, 64)
		maxQty, _ := strconv.ParseFloat(s.LotSizeFilter.MaxOrderQty, 64)
		stepSize, _ := strconv.ParseFloat(s.LotSizeFilter.QtyStep, 64)

		info := &exchange.SymbolInfo{
			Symbol:     s.Symbol,
			BaseAsset:  s.BaseCoin,
			QuoteAsset: s.QuoteCoin,
			TickSize:   tickSize,
			MinQty:     minQty,
			MaxQty:     maxQty,
			StepSize:   stepSize,
		}

		c.symbols[s.Symbol] = info
	}

	log.Printf("[Bybit] Loaded %d symbols", len(c.symbols))
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

	log.Printf("[Bybit] WebSocket closed")
	return nil
}

// GetCurrentPrice returns the current price
func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	resp, err := http.Get(fmt.Sprintf("%s/v5/market/tickers?category=linear&symbol=%s", c.restURL, strings.ToUpper(symbol)))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			List []struct {
				MarkPrice string `json:"markPrice"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Result.List) == 0 {
		return 0, fmt.Errorf("no price data")
	}

	return strconv.ParseFloat(result.Result.List[0].MarkPrice, 64)
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
		return 0.005
	case positionValue <= 250000:
		return 0.01
	default:
		return 0.025
	}
}

// GetFeeRate returns taker and maker fee rates
func (c *Client) GetFeeRate() (takerFee, makerFee float64) {
	return 0.0006, 0.0001
}
