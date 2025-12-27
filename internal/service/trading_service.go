package service

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ccxt-simulator/internal/exchange"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidSymbol       = errors.New("invalid symbol")
	ErrInvalidQuantity     = errors.New("invalid quantity")
	ErrInvalidLeverage     = errors.New("invalid leverage")
	ErrPositionNotFound    = errors.New("position not found")
	ErrOrderNotFound       = errors.New("order not found")
	ErrNoOpenPosition      = errors.New("no open position to close")
	ErrInvalidOrderType    = errors.New("invalid order type")
)

const (
	defaultSlippage = 0.0001 // 0.01% slippage for market orders
)

// TradingService handles trading operations
type TradingService struct {
	accountRepo   *repository.AccountRepository
	positionRepo  *repository.PositionRepository
	orderRepo     *repository.OrderRepository
	tradeRepo     *repository.TradeRepository
	closedPnLRepo *repository.ClosedPnLRepository
	priceService  *PriceService

	leverageCache map[uint]map[string]int // accountID -> symbol -> leverage
	cacheMux      sync.RWMutex
}

// NewTradingService creates a new TradingService
func NewTradingService(
	accountRepo *repository.AccountRepository,
	positionRepo *repository.PositionRepository,
	orderRepo *repository.OrderRepository,
	tradeRepo *repository.TradeRepository,
	closedPnLRepo *repository.ClosedPnLRepository,
	priceService *PriceService,
) *TradingService {
	return &TradingService{
		accountRepo:   accountRepo,
		positionRepo:  positionRepo,
		orderRepo:     orderRepo,
		tradeRepo:     tradeRepo,
		closedPnLRepo: closedPnLRepo,
		priceService:  priceService,
		leverageCache: make(map[uint]map[string]int),
	}
}

// OpenPositionRequest represents a request to open a position
type OpenPositionRequest struct {
	AccountID  uint                `json:"account_id"`
	Symbol     string              `json:"symbol" binding:"required"`
	Side       models.PositionSide `json:"side" binding:"required"`
	Quantity   float64             `json:"quantity" binding:"required,gt=0"`
	Leverage   int                 `json:"leverage" binding:"omitempty,min=1,max=125"`
	OrderType  models.OrderType    `json:"order_type"`
	Price      float64             `json:"price"` // For limit orders
	StopLoss   *float64            `json:"stop_loss"`
	TakeProfit *float64            `json:"take_profit"`
	ReduceOnly bool                `json:"reduce_only"`
}

// ClosePositionRequest represents a request to close a position
type ClosePositionRequest struct {
	AccountID uint                `json:"account_id"`
	Symbol    string              `json:"symbol" binding:"required"`
	Side      models.PositionSide `json:"side"`
	Quantity  *float64            `json:"quantity"` // nil means close all
	OrderType models.OrderType    `json:"order_type"`
	Price     float64             `json:"price"` // For limit orders
}

// OpenPosition opens a new position or adds to an existing one
func (s *TradingService) OpenPosition(req *OpenPositionRequest, exchangeType models.ExchangeType) (*models.Order, *models.Position, error) {
	// Get account
	account, err := s.accountRepo.GetByID(req.AccountID)
	if err != nil {
		return nil, nil, err
	}

	// Validate symbol
	symbolInfo, err := s.priceService.GetSymbolInfo(string(exchangeType), req.Symbol)
	if err != nil {
		return nil, nil, ErrInvalidSymbol
	}

	// Get current price
	currentPrice, err := s.priceService.GetPrice(string(exchangeType), req.Symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get price: %w", err)
	}

	// Apply slippage for market orders
	executionPrice := currentPrice
	if req.OrderType == "" || req.OrderType == models.OrderTypeMarket {
		req.OrderType = models.OrderTypeMarket
		if req.Side == models.PositionSideLong {
			executionPrice = currentPrice * (1 + defaultSlippage)
		} else {
			executionPrice = currentPrice * (1 - defaultSlippage)
		}
	} else if req.OrderType == models.OrderTypeLimit {
		executionPrice = req.Price
	}

	// Round price to precision
	executionPrice = s.roundPrice(executionPrice, symbolInfo)

	// Get or set leverage
	leverage := req.Leverage
	if leverage == 0 {
		leverage = s.getLeverage(account.ID, req.Symbol, account.DefaultLeverage)
	}

	// Validate quantity
	if req.Quantity < symbolInfo.MinQty || req.Quantity > symbolInfo.MaxQty {
		return nil, nil, ErrInvalidQuantity
	}

	// Calculate required margin
	positionValue := executionPrice * req.Quantity
	requiredMargin := positionValue / float64(leverage)
	fee := positionValue * account.TakerFeeRate

	// Check balance
	if account.BalanceUSDT < requiredMargin+fee {
		return nil, nil, ErrInsufficientBalance
	}

	// Create order
	order := &models.Order{
		AccountID:     req.AccountID,
		ClientOrderID: uuid.New().String(),
		Symbol:        req.Symbol,
		Side:          s.getSide(req.Side, true),
		PositionSide:  req.Side,
		Type:          req.OrderType,
		Quantity:      req.Quantity,
		Price:         req.Price,
		Status:        models.OrderStatusNew,
		ReduceOnly:    req.ReduceOnly,
	}

	if err := s.orderRepo.Create(order); err != nil {
		return nil, nil, fmt.Errorf("failed to create order: %w", err)
	}

	// For market orders, execute immediately
	if req.OrderType == models.OrderTypeMarket {
		return s.executeOpenOrder(order, account, symbolInfo, executionPrice, leverage, fee, req.StopLoss, req.TakeProfit)
	}

	// For limit orders, return pending order
	return order, nil, nil
}

// executeOpenOrder executes a market open order
func (s *TradingService) executeOpenOrder(
	order *models.Order,
	account *models.Account,
	symbolInfo *exchange.SymbolInfo,
	executionPrice float64,
	leverage int,
	fee float64,
	stopLoss, takeProfit *float64,
) (*models.Order, *models.Position, error) {
	positionValue := executionPrice * order.Quantity
	margin := positionValue / float64(leverage)

	// Update order status
	order.Status = models.OrderStatusFilled
	order.FilledQty = order.Quantity
	order.AvgPrice = executionPrice
	if err := s.orderRepo.Update(order); err != nil {
		return nil, nil, err
	}

	// Create trade record
	trade := &models.Trade{
		AccountID:   order.AccountID,
		OrderID:     order.ID,
		Symbol:      order.Symbol,
		Side:        order.Side,
		Quantity:    order.Quantity,
		Price:       executionPrice,
		Fee:         fee,
		FeeCurrency: "USDT",
		IsMaker:     false,
		ExecutedAt:  time.Now(),
	}
	if err := s.tradeRepo.Create(trade); err != nil {
		return nil, nil, err
	}

	// Check for existing position
	existingPosition, err := s.positionRepo.GetByAccountIDSymbolAndSide(order.AccountID, order.Symbol, order.PositionSide)
	var position *models.Position

	if err == nil && existingPosition != nil {
		// Add to existing position
		totalQty := existingPosition.Quantity + order.Quantity
		avgPrice := (existingPosition.EntryPrice*existingPosition.Quantity + executionPrice*order.Quantity) / totalQty

		existingPosition.Quantity = totalQty
		existingPosition.EntryPrice = avgPrice
		existingPosition.Margin += margin
		existingPosition.LiquidationPrice = s.calculateLiquidationPrice(avgPrice, leverage, order.PositionSide)

		if stopLoss != nil {
			existingPosition.StopLoss = stopLoss
		}
		if takeProfit != nil {
			existingPosition.TakeProfit = takeProfit
		}

		if err := s.positionRepo.Update(existingPosition); err != nil {
			return nil, nil, err
		}
		position = existingPosition
	} else {
		// Create new position
		position = &models.Position{
			AccountID:        order.AccountID,
			Symbol:           order.Symbol,
			Side:             order.PositionSide,
			Quantity:         order.Quantity,
			EntryPrice:       executionPrice,
			MarkPrice:        executionPrice,
			Leverage:         leverage,
			MarginMode:       account.MarginMode,
			Margin:           margin,
			LiquidationPrice: s.calculateLiquidationPrice(executionPrice, leverage, order.PositionSide),
			StopLoss:         stopLoss,
			TakeProfit:       takeProfit,
		}

		if err := s.positionRepo.Create(position); err != nil {
			return nil, nil, err
		}
	}

	// Deduct margin and fee from balance
	account.BalanceUSDT -= (margin + fee)
	if err := s.accountRepo.Update(account); err != nil {
		return nil, nil, err
	}

	return order, position, nil
}

// ClosePosition closes an existing position
func (s *TradingService) ClosePosition(req *ClosePositionRequest, exchangeType models.ExchangeType) (*models.Order, *models.ClosedPnLRecord, error) {
	// Get account
	account, err := s.accountRepo.GetByID(req.AccountID)
	if err != nil {
		return nil, nil, err
	}

	// Find position
	position, err := s.positionRepo.GetByAccountIDSymbolAndSide(req.AccountID, req.Symbol, req.Side)
	if err != nil {
		return nil, nil, ErrNoOpenPosition
	}

	// Determine close quantity
	closeQty := position.Quantity
	if req.Quantity != nil && *req.Quantity > 0 {
		if *req.Quantity > position.Quantity {
			return nil, nil, ErrInvalidQuantity
		}
		closeQty = *req.Quantity
	}

	// Get current price
	currentPrice, err := s.priceService.GetPrice(string(exchangeType), req.Symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get price: %w", err)
	}

	// Apply slippage
	executionPrice := currentPrice
	if req.OrderType == "" || req.OrderType == models.OrderTypeMarket {
		req.OrderType = models.OrderTypeMarket
		if req.Side == models.PositionSideLong {
			// Closing long = selling, price goes down
			executionPrice = currentPrice * (1 - defaultSlippage)
		} else {
			// Closing short = buying, price goes up
			executionPrice = currentPrice * (1 + defaultSlippage)
		}
	}

	// Calculate PnL
	var realizedPnL float64
	if position.Side == models.PositionSideLong {
		realizedPnL = (executionPrice - position.EntryPrice) * closeQty
	} else {
		realizedPnL = (position.EntryPrice - executionPrice) * closeQty
	}

	// Calculate fee
	positionValue := executionPrice * closeQty
	fee := positionValue * account.TakerFeeRate

	// Create close order
	order := &models.Order{
		AccountID:     req.AccountID,
		ClientOrderID: uuid.New().String(),
		Symbol:        req.Symbol,
		Side:          s.getSide(req.Side, false),
		PositionSide:  req.Side,
		Type:          req.OrderType,
		Quantity:      closeQty,
		Price:         req.Price,
		Status:        models.OrderStatusFilled,
		FilledQty:     closeQty,
		AvgPrice:      executionPrice,
		ReduceOnly:    true,
		ClosePosition: closeQty == position.Quantity,
	}

	if err := s.orderRepo.Create(order); err != nil {
		return nil, nil, err
	}

	// Create trade record
	trade := &models.Trade{
		AccountID:   order.AccountID,
		OrderID:     order.ID,
		Symbol:      order.Symbol,
		Side:        order.Side,
		Quantity:    closeQty,
		Price:       executionPrice,
		Fee:         fee,
		FeeCurrency: "USDT",
		RealizedPnL: realizedPnL,
		IsMaker:     false,
		ExecutedAt:  time.Now(),
	}
	if err := s.tradeRepo.Create(trade); err != nil {
		return nil, nil, err
	}

	// Calculate margin to return
	marginToReturn := position.Margin * (closeQty / position.Quantity)

	// Update or delete position
	var closedPnL *models.ClosedPnLRecord
	if closeQty >= position.Quantity {
		// Full close
		closedPnL = &models.ClosedPnLRecord{
			AccountID:    req.AccountID,
			Symbol:       req.Symbol,
			Side:         position.Side,
			Quantity:     position.Quantity,
			EntryPrice:   position.EntryPrice,
			ExitPrice:    executionPrice,
			RealizedPnL:  realizedPnL,
			TotalFee:     fee,
			Leverage:     position.Leverage,
			ClosedReason: "manual",
			OpenedAt:     position.CreatedAt,
			ClosedAt:     time.Now(),
		}
		if err := s.closedPnLRepo.Create(closedPnL); err != nil {
			return nil, nil, err
		}

		if err := s.positionRepo.Delete(position.ID); err != nil {
			return nil, nil, err
		}
	} else {
		// Partial close
		position.Quantity -= closeQty
		position.Margin -= marginToReturn
		if err := s.positionRepo.Update(position); err != nil {
			return nil, nil, err
		}
	}

	// Update account balance
	account.BalanceUSDT += marginToReturn + realizedPnL - fee
	if err := s.accountRepo.Update(account); err != nil {
		return nil, nil, err
	}

	return order, closedPnL, nil
}

// GetPositions returns all positions for an account
func (s *TradingService) GetPositions(accountID uint, exchangeType models.ExchangeType) ([]models.Position, error) {
	positions, err := s.positionRepo.GetByAccountID(accountID)
	if err != nil {
		return nil, err
	}

	// Update mark price and unrealized PnL
	for i := range positions {
		price, err := s.priceService.GetPrice(string(exchangeType), positions[i].Symbol)
		if err == nil {
			positions[i].MarkPrice = price
			positions[i].UnrealizedPnL = positions[i].CalculateUnrealizedPnL(price)
		}
	}

	return positions, nil
}

// GetBalance returns account balance with unrealized PnL
func (s *TradingService) GetBalance(accountID uint, exchangeType models.ExchangeType) (map[string]float64, error) {
	account, err := s.accountRepo.GetByID(accountID)
	if err != nil {
		return nil, err
	}

	positions, err := s.GetPositions(accountID, exchangeType)
	if err != nil {
		return nil, err
	}

	var totalUnrealizedPnL float64
	var totalMargin float64
	for _, pos := range positions {
		totalUnrealizedPnL += pos.UnrealizedPnL
		totalMargin += pos.Margin
	}

	return map[string]float64{
		"balance":         account.BalanceUSDT,
		"available":       account.BalanceUSDT - totalMargin,
		"margin":          totalMargin,
		"unrealized_pnl":  totalUnrealizedPnL,
		"equity":          account.BalanceUSDT + totalUnrealizedPnL,
		"initial_balance": account.InitialBalance,
	}, nil
}

// SetLeverage sets the leverage for a symbol
func (s *TradingService) SetLeverage(accountID uint, symbol string, leverage int) error {
	if leverage < 1 || leverage > 125 {
		return ErrInvalidLeverage
	}

	s.cacheMux.Lock()
	defer s.cacheMux.Unlock()

	if s.leverageCache[accountID] == nil {
		s.leverageCache[accountID] = make(map[string]int)
	}
	s.leverageCache[accountID][symbol] = leverage

	return nil
}

// SetStopLoss sets stop loss for a position
func (s *TradingService) SetStopLoss(accountID uint, symbol string, side models.PositionSide, stopLoss float64) error {
	position, err := s.positionRepo.GetByAccountIDSymbolAndSide(accountID, symbol, side)
	if err != nil {
		return ErrPositionNotFound
	}

	position.StopLoss = &stopLoss
	return s.positionRepo.Update(position)
}

// SetTakeProfit sets take profit for a position
func (s *TradingService) SetTakeProfit(accountID uint, symbol string, side models.PositionSide, takeProfit float64) error {
	position, err := s.positionRepo.GetByAccountIDSymbolAndSide(accountID, symbol, side)
	if err != nil {
		return ErrPositionNotFound
	}

	position.TakeProfit = &takeProfit
	return s.positionRepo.Update(position)
}

// CancelAllOrders cancels all open orders
func (s *TradingService) CancelAllOrders(accountID uint, symbol string) (int64, error) {
	if symbol != "" {
		return s.orderRepo.CancelAllOpenOrdersBySymbol(accountID, symbol)
	}
	return s.orderRepo.CancelAllOpenOrders(accountID)
}

// GetOrderStatus returns order status
func (s *TradingService) GetOrderStatus(accountID uint, orderID uint) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if order.AccountID != accountID {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

// GetClosedPnL returns closed PnL records
func (s *TradingService) GetClosedPnL(accountID uint, page, pageSize int) ([]models.ClosedPnLRecord, int64, error) {
	return s.closedPnLRepo.GetByAccountIDPaginated(accountID, page, pageSize)
}

// Helper functions

func (s *TradingService) getLeverage(accountID uint, symbol string, defaultLeverage int) int {
	s.cacheMux.RLock()
	defer s.cacheMux.RUnlock()

	if leverages, ok := s.leverageCache[accountID]; ok {
		if lev, ok := leverages[symbol]; ok {
			return lev
		}
	}
	return defaultLeverage
}

func (s *TradingService) getSide(positionSide models.PositionSide, isOpen bool) models.OrderSide {
	if isOpen {
		if positionSide == models.PositionSideLong {
			return models.OrderSideBuy
		}
		return models.OrderSideSell
	}
	// Closing
	if positionSide == models.PositionSideLong {
		return models.OrderSideSell
	}
	return models.OrderSideBuy
}

func (s *TradingService) calculateLiquidationPrice(entryPrice float64, leverage int, side models.PositionSide) float64 {
	maintenanceMarginRate := 0.004 // Default MMR

	if side == models.PositionSideLong {
		return entryPrice * (1 - 1/float64(leverage) + maintenanceMarginRate)
	}
	return entryPrice * (1 + 1/float64(leverage) - maintenanceMarginRate)
}

func (s *TradingService) roundPrice(price float64, symbolInfo *exchange.SymbolInfo) float64 {
	if symbolInfo.TickSize > 0 {
		return math.Round(price/symbolInfo.TickSize) * symbolInfo.TickSize
	}
	precision := math.Pow(10, float64(symbolInfo.PricePrecision))
	return math.Round(price*precision) / precision
}
