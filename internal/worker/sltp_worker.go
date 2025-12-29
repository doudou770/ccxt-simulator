package worker

import (
	"log"
	"time"

	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/repository"
	"github.com/ccxt-simulator/internal/service"
)

// SLTPWorker monitors pending stop-loss and take-profit orders
// and triggers execution when price conditions are met
type SLTPWorker struct {
	tradingService *service.TradingService
	orderRepo      *repository.OrderRepository
	interval       time.Duration
	stopChan       chan struct{}
}

// NewSLTPWorker creates a new SL/TP monitoring worker
func NewSLTPWorker(
	tradingService *service.TradingService,
	orderRepo *repository.OrderRepository,
	interval time.Duration,
) *SLTPWorker {
	if interval <= 0 {
		interval = 1 * time.Second // Default 1 second check interval
	}
	return &SLTPWorker{
		tradingService: tradingService,
		orderRepo:      orderRepo,
		interval:       interval,
		stopChan:       make(chan struct{}),
	}
}

// Start begins the monitoring loop
func (w *SLTPWorker) Start() {
	log.Printf("SL/TP Worker started with interval: %v", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.checkAndTriggerOrders()
		case <-w.stopChan:
			log.Println("SL/TP Worker stopped")
			return
		}
	}
}

// Stop stops the monitoring loop
func (w *SLTPWorker) Stop() {
	close(w.stopChan)
}

// checkAndTriggerOrders checks all pending stop orders and triggers if conditions are met
func (w *SLTPWorker) checkAndTriggerOrders() {
	// Get all pending stop orders
	orders, err := w.orderRepo.GetAllPendingStopOrders()
	if err != nil {
		log.Printf("SL/TP Worker: failed to get pending orders: %v", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	priceService := w.tradingService.GetPriceService()

	for _, order := range orders {
		// Get current price for the symbol
		// Try each exchange until we get a price
		var currentPrice float64
		var priceErr error

		exchanges := []string{"binance", "okx", "bybit", "bitget"}
		for _, exchange := range exchanges {
			currentPrice, priceErr = priceService.GetPrice(exchange, order.Symbol)
			if priceErr == nil && currentPrice > 0 {
				break
			}
		}

		if priceErr != nil || currentPrice <= 0 {
			// Skip if no price available
			continue
		}

		// Check if order should be triggered
		if w.shouldTrigger(&order, currentPrice) {
			log.Printf("SL/TP Worker: triggering order %d (type=%s, symbol=%s, stopPrice=%.8f, currentPrice=%.8f)",
				order.ID, order.Type, order.Symbol, order.StopPrice, currentPrice)

			// Determine exchange type from account (for now default to binance)
			exchangeType := models.ExchangeBinance

			// Execute the triggered order
			closedPnL, err := w.tradingService.ExecuteTriggeredOrder(&order, exchangeType)
			if err != nil {
				log.Printf("SL/TP Worker: failed to execute order %d: %v", order.ID, err)
				continue
			}

			if closedPnL != nil {
				log.Printf("SL/TP Worker: order %d executed, PnL=%.8f, reason=%s",
					order.ID, closedPnL.RealizedPnL, closedPnL.ClosedReason)
			}
		}
	}
}

// shouldTrigger checks if the order should be triggered based on current price
// Trigger logic:
// | Order Type    | Position Side | Trigger Condition       |
// |---------------|---------------|-------------------------|
// | STOP_MARKET   | LONG          | markPrice <= stopPrice  |
// | STOP_MARKET   | SHORT         | markPrice >= stopPrice  |
// | TAKE_PROFIT   | LONG          | markPrice >= stopPrice  |
// | TAKE_PROFIT   | SHORT         | markPrice <= stopPrice  |
func (w *SLTPWorker) shouldTrigger(order *models.Order, currentPrice float64) bool {
	stopPrice := order.StopPrice
	if stopPrice <= 0 {
		return false
	}

	isStopLoss := order.Type == models.OrderTypeStopMarket || order.Type == models.OrderTypeStopLoss
	isTakeProfit := order.Type == models.OrderTypeTakeProfit

	switch {
	case isStopLoss && order.PositionSide == models.PositionSideLong:
		// Long position stop loss: trigger when price falls to or below stop price
		return currentPrice <= stopPrice

	case isStopLoss && order.PositionSide == models.PositionSideShort:
		// Short position stop loss: trigger when price rises to or above stop price
		return currentPrice >= stopPrice

	case isTakeProfit && order.PositionSide == models.PositionSideLong:
		// Long position take profit: trigger when price rises to or above stop price
		return currentPrice >= stopPrice

	case isTakeProfit && order.PositionSide == models.PositionSideShort:
		// Short position take profit: trigger when price falls to or below stop price
		return currentPrice <= stopPrice

	default:
		return false
	}
}
