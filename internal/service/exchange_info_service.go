package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ExchangeInfoService caches exchange info from real exchanges
type ExchangeInfoService struct {
	redis          *redis.Client
	cache          map[string]interface{}
	cacheMux       sync.RWMutex
	updateInterval time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewExchangeInfoService creates a new ExchangeInfoService
func NewExchangeInfoService(redisClient *redis.Client) *ExchangeInfoService {
	return &ExchangeInfoService{
		redis:          redisClient,
		cache:          make(map[string]interface{}),
		updateInterval: 1 * time.Hour,
	}
}

// Start starts the periodic update of exchange info
func (s *ExchangeInfoService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Initial load
	s.updateAllExchangeInfo()

	// Start periodic update
	go s.updateLoop()

	return nil
}

// Stop stops the service
func (s *ExchangeInfoService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *ExchangeInfoService) updateLoop() {
	ticker := time.NewTicker(s.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateAllExchangeInfo()
		}
	}
}

func (s *ExchangeInfoService) updateAllExchangeInfo() {
	exchanges := []string{"binance", "okx", "bybit", "bitget", "hyperliquid"}

	for _, exchange := range exchanges {
		if err := s.fetchExchangeInfo(exchange); err != nil {
			log.Printf("[ExchangeInfo] Failed to update %s: %v", exchange, err)
		} else {
			log.Printf("[ExchangeInfo] Updated %s", exchange)
		}
	}
}

func (s *ExchangeInfoService) fetchExchangeInfo(exchange string) error {
	var url string

	switch exchange {
	case "binance":
		url = "https://fapi.binance.com/fapi/v1/exchangeInfo"
	case "okx":
		url = "https://www.okx.com/api/v5/public/instruments?instType=SWAP"
	case "bybit":
		url = "https://api.bybit.com/v5/market/instruments-info?category=linear"
	case "bitget":
		url = "https://api.bitget.com/api/v2/mix/market/contracts?productType=USDT-FUTURES"
	case "hyperliquid":
		// Hyperliquid uses POST for meta
		return s.fetchHyperliquidMeta()
	default:
		return fmt.Errorf("unknown exchange: %s", exchange)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	// Store in cache
	s.cacheMux.Lock()
	s.cache[exchange] = data
	s.cacheMux.Unlock()

	// Store in Redis
	jsonData, _ := json.Marshal(data)
	s.redis.Set(s.ctx, "exchangeinfo:"+exchange, jsonData, 2*time.Hour)

	return nil
}

func (s *ExchangeInfoService) fetchHyperliquidMeta() error {
	client := &http.Client{Timeout: 30 * time.Second}

	body := []byte(`{"type":"meta"}`)
	req, _ := http.NewRequest("POST", "https://api.hyperliquid.xyz/info",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	s.cacheMux.Lock()
	s.cache["hyperliquid"] = data
	s.cacheMux.Unlock()

	jsonData, _ := json.Marshal(data)
	s.redis.Set(s.ctx, "exchangeinfo:hyperliquid", jsonData, 2*time.Hour)

	return nil
}

// GetExchangeInfo returns cached exchange info
func (s *ExchangeInfoService) GetExchangeInfo(exchange string) (interface{}, error) {
	// Try memory cache first
	s.cacheMux.RLock()
	if data, ok := s.cache[exchange]; ok {
		s.cacheMux.RUnlock()
		return data, nil
	}
	s.cacheMux.RUnlock()

	// Try Redis
	jsonData, err := s.redis.Get(s.ctx, "exchangeinfo:"+exchange).Bytes()
	if err == nil {
		var data interface{}
		if json.Unmarshal(jsonData, &data) == nil {
			s.cacheMux.Lock()
			s.cache[exchange] = data
			s.cacheMux.Unlock()
			return data, nil
		}
	}

	// Fetch fresh data
	if err := s.fetchExchangeInfo(exchange); err != nil {
		return nil, err
	}

	s.cacheMux.RLock()
	data := s.cache[exchange]
	s.cacheMux.RUnlock()

	return data, nil
}

// Helper for json.NewReader
type jsonReader struct {
	data []byte
	pos  int
}

func (r *jsonReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func init() {
	// Ensure json.NewReader is available
}

// Create a simple reader from map
func jsonReaderFromMap(m map[string]string) *jsonReader {
	data, _ := json.Marshal(m)
	return &jsonReader{data: data}
}

// NewReader creates a json reader from a map
func newJSONReader(m map[string]string) *jsonReader {
	return jsonReaderFromMap(m)
}
