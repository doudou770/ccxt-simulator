package exchange

import (
	"context"
)

// PriceUpdate represents a real-time price update from an exchange
type PriceUpdate struct {
	Exchange  string  `json:"exchange"`
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	BidPrice  float64 `json:"bid_price"`
	AskPrice  float64 `json:"ask_price"`
	Timestamp int64   `json:"timestamp"`
}

// SymbolInfo represents trading pair information
type SymbolInfo struct {
	Symbol            string  `json:"symbol"`
	BaseAsset         string  `json:"base_asset"`
	QuoteAsset        string  `json:"quote_asset"`
	PricePrecision    int     `json:"price_precision"`
	QuantityPrecision int     `json:"quantity_precision"`
	MinQty            float64 `json:"min_qty"`
	MaxQty            float64 `json:"max_qty"`
	MinNotional       float64 `json:"min_notional"`
	TickSize          float64 `json:"tick_size"`
	StepSize          float64 `json:"step_size"`
}

// PriceSubscriber is an interface for components that receive price updates
type PriceSubscriber interface {
	OnPriceUpdate(update PriceUpdate)
}

// PriceProvider is an interface for exchange WebSocket price streams
type PriceProvider interface {
	// Connect establishes WebSocket connection to the exchange
	Connect(ctx context.Context) error

	// Subscribe subscribes to price updates for given symbols
	Subscribe(symbols []string) error

	// Unsubscribe unsubscribes from price updates for given symbols
	Unsubscribe(symbols []string) error

	// SetSubscriber sets the price update subscriber
	SetSubscriber(subscriber PriceSubscriber)

	// GetSymbolInfo returns trading pair information
	GetSymbolInfo(symbol string) (*SymbolInfo, error)

	// GetAllSymbols returns all available trading symbols
	GetAllSymbols() ([]string, error)

	// Close closes the WebSocket connection
	Close() error

	// ExchangeName returns the exchange name
	ExchangeName() string

	// IsConnected returns whether the WebSocket is connected
	IsConnected() bool
}

// ExchangeAdapter is the main interface for exchange operations
type ExchangeAdapter interface {
	PriceProvider

	// GetCurrentPrice returns the current price for a symbol
	GetCurrentPrice(symbol string) (float64, error)

	// ValidateSymbol checks if a symbol is valid
	ValidateSymbol(symbol string) bool

	// GetMaintenanceMarginRate returns the maintenance margin rate for a position value
	GetMaintenanceMarginRate(positionValue float64) float64

	// GetFeeRate returns taker and maker fee rates
	GetFeeRate() (takerFee, makerFee float64)
}
