package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ccxt-simulator/internal/exchange"
	"github.com/ccxt-simulator/internal/exchange/binance"
	"github.com/ccxt-simulator/internal/exchange/bybit"
	"github.com/ccxt-simulator/internal/exchange/okx"
	"github.com/redis/go-redis/v9"
)

// Default symbols to subscribe
var DefaultSymbols = []string{
	"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
	"ADAUSDT", "DOGEUSDT", "AVAXUSDT", "DOTUSDT", "LINKUSDT",
	"MATICUSDT", "LTCUSDT", "UNIUSDT", "ATOMUSDT", "ETCUSDT",
	"XLMUSDT", "FILUSDT", "TRXUSDT", "NEARUSDT", "AAVEUSDT",
}

// PriceService manages real-time price data from multiple exchanges
type PriceService struct {
	redis     *redis.Client
	providers map[string]exchange.PriceProvider
	prices    map[string]map[string]exchange.PriceUpdate // exchange -> symbol -> price
	pricesMux sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPriceService creates a new PriceService
func NewPriceService(redisClient *redis.Client) *PriceService {
	return &PriceService{
		redis:     redisClient,
		providers: make(map[string]exchange.PriceProvider),
		prices:    make(map[string]map[string]exchange.PriceUpdate),
	}
}

// Start starts the price service and connects to all exchanges
func (s *PriceService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Initialize exchange clients
	binanceClient := binance.NewClient()
	okxClient := okx.NewClient()
	bybitClient := bybit.NewClient()

	// Set this service as the subscriber for all exchanges
	binanceClient.SetSubscriber(s)
	okxClient.SetSubscriber(s)
	bybitClient.SetSubscriber(s)

	// Store providers
	s.providers["binance"] = binanceClient
	s.providers["okx"] = okxClient
	s.providers["bybit"] = bybitClient

	// Initialize price maps
	s.prices["binance"] = make(map[string]exchange.PriceUpdate)
	s.prices["okx"] = make(map[string]exchange.PriceUpdate)
	s.prices["bybit"] = make(map[string]exchange.PriceUpdate)

	// Connect to each exchange
	for name, provider := range s.providers {
		if err := provider.Connect(s.ctx); err != nil {
			log.Printf("[PriceService] Failed to connect to %s: %v", name, err)
			continue
		}

		// Subscribe to default symbols
		if err := provider.Subscribe(DefaultSymbols); err != nil {
			log.Printf("[PriceService] Failed to subscribe on %s: %v", name, err)
		}
	}

	log.Printf("[PriceService] Started with %d exchanges", len(s.providers))
	return nil
}

// OnPriceUpdate implements exchange.PriceSubscriber
func (s *PriceService) OnPriceUpdate(update exchange.PriceUpdate) {
	// Store in memory
	s.pricesMux.Lock()
	if s.prices[update.Exchange] == nil {
		s.prices[update.Exchange] = make(map[string]exchange.PriceUpdate)
	}
	s.prices[update.Exchange][update.Symbol] = update
	s.pricesMux.Unlock()

	// Store in Redis for persistence
	key := fmt.Sprintf("price:%s:%s", update.Exchange, update.Symbol)

	s.redis.HSet(s.ctx, key, map[string]interface{}{
		"price":     update.Price,
		"bid":       update.BidPrice,
		"ask":       update.AskPrice,
		"timestamp": update.Timestamp,
	})

	// Set expiry for the key (5 seconds)
	s.redis.Expire(s.ctx, key, 5*time.Second)

	// Publish price update for subscribers (e.g., trading engine)
	s.redis.Publish(s.ctx, "price_updates", fmt.Sprintf("%s:%s:%.8f", update.Exchange, update.Symbol, update.Price))
}

// GetPrice returns the current price for a symbol from a specific exchange
func (s *PriceService) GetPrice(exchangeName, symbol string) (float64, error) {
	// Try memory cache first
	s.pricesMux.RLock()
	if prices, ok := s.prices[exchangeName]; ok {
		if update, ok := prices[symbol]; ok {
			s.pricesMux.RUnlock()
			// Check if price is stale (> 5 seconds old)
			if time.Now().UnixMilli()-update.Timestamp < 5000 {
				return update.Price, nil
			}
		}
	}
	s.pricesMux.RUnlock()

	// Try Redis
	key := fmt.Sprintf("price:%s:%s", exchangeName, symbol)
	result, err := s.redis.HGet(s.ctx, key, "price").Float64()
	if err == nil {
		return result, nil
	}

	// Fallback to REST API
	provider, ok := s.providers[exchangeName]
	if !ok {
		return 0, fmt.Errorf("exchange not found: %s", exchangeName)
	}

	if adapter, ok := provider.(exchange.ExchangeAdapter); ok {
		return adapter.GetCurrentPrice(symbol)
	}

	return 0, fmt.Errorf("price not available for %s on %s", symbol, exchangeName)
}

// GetPriceUpdate returns the full price update for a symbol
func (s *PriceService) GetPriceUpdate(exchangeName, symbol string) (*exchange.PriceUpdate, error) {
	s.pricesMux.RLock()
	defer s.pricesMux.RUnlock()

	if prices, ok := s.prices[exchangeName]; ok {
		if update, ok := prices[symbol]; ok {
			return &update, nil
		}
	}

	return nil, fmt.Errorf("price not found for %s on %s", symbol, exchangeName)
}

// GetAllPrices returns all current prices for an exchange
func (s *PriceService) GetAllPrices(exchangeName string) map[string]float64 {
	s.pricesMux.RLock()
	defer s.pricesMux.RUnlock()

	result := make(map[string]float64)
	if prices, ok := s.prices[exchangeName]; ok {
		for symbol, update := range prices {
			result[symbol] = update.Price
		}
	}

	return result
}

// Subscribe subscribes to price updates for additional symbols
func (s *PriceService) Subscribe(exchangeName string, symbols []string) error {
	provider, ok := s.providers[exchangeName]
	if !ok {
		return fmt.Errorf("exchange not found: %s", exchangeName)
	}

	return provider.Subscribe(symbols)
}

// Unsubscribe unsubscribes from price updates
func (s *PriceService) Unsubscribe(exchangeName string, symbols []string) error {
	provider, ok := s.providers[exchangeName]
	if !ok {
		return fmt.Errorf("exchange not found: %s", exchangeName)
	}

	return provider.Unsubscribe(symbols)
}

// GetSymbolInfo returns trading pair information
func (s *PriceService) GetSymbolInfo(exchangeName, symbol string) (*exchange.SymbolInfo, error) {
	provider, ok := s.providers[exchangeName]
	if !ok {
		return nil, fmt.Errorf("exchange not found: %s", exchangeName)
	}

	return provider.GetSymbolInfo(symbol)
}

// GetProvider returns the exchange provider
func (s *PriceService) GetProvider(exchangeName string) (exchange.PriceProvider, bool) {
	provider, ok := s.providers[exchangeName]
	return provider, ok
}

// IsConnected checks if an exchange is connected
func (s *PriceService) IsConnected(exchangeName string) bool {
	provider, ok := s.providers[exchangeName]
	if !ok {
		return false
	}
	return provider.IsConnected()
}

// GetExchangeStatus returns connection status for all exchanges
func (s *PriceService) GetExchangeStatus() map[string]bool {
	status := make(map[string]bool)
	for name, provider := range s.providers {
		status[name] = provider.IsConnected()
	}
	return status
}

// Stop stops the price service
func (s *PriceService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}

	for name, provider := range s.providers {
		if err := provider.Close(); err != nil {
			log.Printf("[PriceService] Error closing %s: %v", name, err)
		}
	}

	s.wg.Wait()
	log.Printf("[PriceService] Stopped")
}
