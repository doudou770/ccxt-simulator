package handler

import (
	"errors"
	"strconv"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/repository"
	"github.com/ccxt-simulator/internal/service"
	"github.com/ccxt-simulator/pkg/response"
	"github.com/gin-gonic/gin"
)

// TradingHandler handles trading API requests
type TradingHandler struct {
	tradingService *service.TradingService
	accountService *service.AccountService
}

// NewTradingHandler creates a new TradingHandler
func NewTradingHandler(tradingService *service.TradingService, accountService *service.AccountService) *TradingHandler {
	return &TradingHandler{
		tradingService: tradingService,
		accountService: accountService,
	}
}

// getAccountAndExchange extracts account and validates ownership
func (h *TradingHandler) getAccountAndExchange(c *gin.Context) (*models.Account, models.ExchangeType, error) {
	userID := middleware.GetUserID(c)

	accountIDStr := c.Param("account_id")
	accountID, err := strconv.ParseUint(accountIDStr, 10, 64)
	if err != nil {
		return nil, "", errors.New("invalid account id")
	}

	account, err := h.accountService.GetAccountByID(userID, uint(accountID))
	if err != nil {
		return nil, "", err
	}

	// Create a models.Account from the response
	acc := &models.Account{
		ID:           account.ID,
		ExchangeType: account.ExchangeType,
	}

	return acc, account.ExchangeType, nil
}

// OpenLong opens a long position
// POST /api/v1/trading/:account_id/open-long
func (h *TradingHandler) OpenLong(c *gin.Context) {
	account, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol     string   `json:"symbol" binding:"required"`
		Quantity   float64  `json:"quantity" binding:"required,gt=0"`
		Leverage   int      `json:"leverage"`
		StopLoss   *float64 `json:"stop_loss"`
		TakeProfit *float64 `json:"take_profit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	openReq := &service.OpenPositionRequest{
		AccountID:  account.ID,
		Symbol:     req.Symbol,
		Side:       models.PositionSideLong,
		Quantity:   req.Quantity,
		Leverage:   req.Leverage,
		OrderType:  models.OrderTypeMarket,
		StopLoss:   req.StopLoss,
		TakeProfit: req.TakeProfit,
	}

	order, position, err := h.tradingService.OpenPosition(openReq, exchangeType)
	if err != nil {
		h.handleTradingError(c, err)
		return
	}

	response.Success(c, gin.H{
		"order":    order,
		"position": position,
	})
}

// OpenShort opens a short position
// POST /api/v1/trading/:account_id/open-short
func (h *TradingHandler) OpenShort(c *gin.Context) {
	account, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol     string   `json:"symbol" binding:"required"`
		Quantity   float64  `json:"quantity" binding:"required,gt=0"`
		Leverage   int      `json:"leverage"`
		StopLoss   *float64 `json:"stop_loss"`
		TakeProfit *float64 `json:"take_profit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	openReq := &service.OpenPositionRequest{
		AccountID:  account.ID,
		Symbol:     req.Symbol,
		Side:       models.PositionSideShort,
		Quantity:   req.Quantity,
		Leverage:   req.Leverage,
		OrderType:  models.OrderTypeMarket,
		StopLoss:   req.StopLoss,
		TakeProfit: req.TakeProfit,
	}

	order, position, err := h.tradingService.OpenPosition(openReq, exchangeType)
	if err != nil {
		h.handleTradingError(c, err)
		return
	}

	response.Success(c, gin.H{
		"order":    order,
		"position": position,
	})
}

// CloseLong closes a long position
// POST /api/v1/trading/:account_id/close-long
func (h *TradingHandler) CloseLong(c *gin.Context) {
	account, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol   string   `json:"symbol" binding:"required"`
		Quantity *float64 `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	closeReq := &service.ClosePositionRequest{
		AccountID: account.ID,
		Symbol:    req.Symbol,
		Side:      models.PositionSideLong,
		Quantity:  req.Quantity,
		OrderType: models.OrderTypeMarket,
	}

	order, closedPnL, err := h.tradingService.ClosePosition(closeReq, exchangeType)
	if err != nil {
		h.handleTradingError(c, err)
		return
	}

	response.Success(c, gin.H{
		"order":      order,
		"closed_pnl": closedPnL,
	})
}

// CloseShort closes a short position
// POST /api/v1/trading/:account_id/close-short
func (h *TradingHandler) CloseShort(c *gin.Context) {
	account, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol   string   `json:"symbol" binding:"required"`
		Quantity *float64 `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	closeReq := &service.ClosePositionRequest{
		AccountID: account.ID,
		Symbol:    req.Symbol,
		Side:      models.PositionSideShort,
		Quantity:  req.Quantity,
		OrderType: models.OrderTypeMarket,
	}

	order, closedPnL, err := h.tradingService.ClosePosition(closeReq, exchangeType)
	if err != nil {
		h.handleTradingError(c, err)
		return
	}

	response.Success(c, gin.H{
		"order":      order,
		"closed_pnl": closedPnL,
	})
}

// GetBalance returns account balance
// GET /api/v1/trading/:account_id/balance
func (h *TradingHandler) GetBalance(c *gin.Context) {
	account, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	balance, err := h.tradingService.GetBalance(account.ID, exchangeType)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, balance)
}

// GetPositions returns all positions
// GET /api/v1/trading/:account_id/positions
func (h *TradingHandler) GetPositions(c *gin.Context) {
	account, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	positions, err := h.tradingService.GetPositions(account.ID, exchangeType)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, positions)
}

// SetLeverage sets leverage for a symbol
// POST /api/v1/trading/:account_id/leverage
func (h *TradingHandler) SetLeverage(c *gin.Context) {
	account, _, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol   string `json:"symbol" binding:"required"`
		Leverage int    `json:"leverage" binding:"required,min=1,max=125"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.tradingService.SetLeverage(account.ID, req.Symbol, req.Leverage); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"symbol":   req.Symbol,
		"leverage": req.Leverage,
	})
}

// SetStopLoss sets stop loss for a position
// POST /api/v1/trading/:account_id/stop-loss
func (h *TradingHandler) SetStopLoss(c *gin.Context) {
	account, _, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol   string              `json:"symbol" binding:"required"`
		Side     models.PositionSide `json:"side" binding:"required"`
		StopLoss float64             `json:"stop_loss" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.tradingService.SetStopLoss(account.ID, req.Symbol, req.Side, req.StopLoss); err != nil {
		h.handleTradingError(c, err)
		return
	}

	response.Success(c, gin.H{"message": "stop loss set"})
}

// SetTakeProfit sets take profit for a position
// POST /api/v1/trading/:account_id/take-profit
func (h *TradingHandler) SetTakeProfit(c *gin.Context) {
	account, _, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req struct {
		Symbol     string              `json:"symbol" binding:"required"`
		Side       models.PositionSide `json:"side" binding:"required"`
		TakeProfit float64             `json:"take_profit" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.tradingService.SetTakeProfit(account.ID, req.Symbol, req.Side, req.TakeProfit); err != nil {
		h.handleTradingError(c, err)
		return
	}

	response.Success(c, gin.H{"message": "take profit set"})
}

// CancelAllOrders cancels all open orders
// DELETE /api/v1/trading/:account_id/orders
func (h *TradingHandler) CancelAllOrders(c *gin.Context) {
	account, _, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	symbol := c.Query("symbol")

	count, err := h.tradingService.CancelAllOrders(account.ID, symbol)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"canceled_count": count,
	})
}

// GetOrderStatus returns order status
// GET /api/v1/trading/:account_id/orders/:order_id
func (h *TradingHandler) GetOrderStatus(c *gin.Context) {
	account, _, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid order id")
		return
	}

	order, err := h.tradingService.GetOrderStatus(account.ID, uint(orderID))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, order)
}

// GetClosedPnL returns closed PnL records
// GET /api/v1/trading/:account_id/closed-pnl
func (h *TradingHandler) GetClosedPnL(c *gin.Context) {
	account, _, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	records, total, err := h.tradingService.GetClosedPnL(account.ID, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessPaginated(c, records, total, page, pageSize)
}

// GetMarketPrice returns current market price
// GET /api/v1/trading/:account_id/price/:symbol
func (h *TradingHandler) GetMarketPrice(c *gin.Context) {
	_, exchangeType, err := h.getAccountAndExchange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	symbol := c.Param("symbol")

	// This handler is already part of priceService but provided here for trading API consistency
	c.Redirect(302, "/api/v1/prices/"+string(exchangeType)+"/"+symbol)
}

// handleTradingError handles common trading errors
func (h *TradingHandler) handleTradingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInsufficientBalance):
		response.Error(c, 400, -2019, "insufficient balance")
	case errors.Is(err, service.ErrInvalidSymbol):
		response.Error(c, 400, -1121, "invalid symbol")
	case errors.Is(err, service.ErrInvalidQuantity):
		response.Error(c, 400, -1013, "invalid quantity")
	case errors.Is(err, service.ErrInvalidLeverage):
		response.Error(c, 400, -4028, "leverage is invalid")
	case errors.Is(err, service.ErrNoOpenPosition):
		response.Error(c, 400, -2022, "no position to close")
	case errors.Is(err, service.ErrPositionNotFound):
		response.Error(c, 404, -2022, "position not found")
	case errors.Is(err, repository.ErrAccountNotFound):
		response.NotFound(c, "account not found")
	default:
		response.InternalError(c, err.Error())
	}
}

// RegisterRoutes registers trading routes
func (h *TradingHandler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	trading := rg.Group("/trading")
	trading.Use(authMiddleware)
	{
		// Open positions
		trading.POST("/:account_id/open-long", h.OpenLong)
		trading.POST("/:account_id/open-short", h.OpenShort)

		// Close positions
		trading.POST("/:account_id/close-long", h.CloseLong)
		trading.POST("/:account_id/close-short", h.CloseShort)

		// Account info
		trading.GET("/:account_id/balance", h.GetBalance)
		trading.GET("/:account_id/positions", h.GetPositions)

		// Settings
		trading.POST("/:account_id/leverage", h.SetLeverage)
		trading.POST("/:account_id/stop-loss", h.SetStopLoss)
		trading.POST("/:account_id/take-profit", h.SetTakeProfit)

		// Orders
		trading.DELETE("/:account_id/orders", h.CancelAllOrders)
		trading.GET("/:account_id/orders/:order_id", h.GetOrderStatus)

		// PnL
		trading.GET("/:account_id/closed-pnl", h.GetClosedPnL)

		// Price
		trading.GET("/:account_id/price/:symbol", h.GetMarketPrice)
	}
}
