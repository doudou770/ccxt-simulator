package binance

import (
	"strconv"
	"time"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler handles Binance-compatible API requests
type Handler struct {
	tradingService *service.TradingService
	priceService   *service.PriceService
}

// NewHandler creates a new Binance handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService) *Handler {
	return &Handler{
		tradingService: tradingService,
		priceService:   priceService,
	}
}

// GetBalance handles GET /fapi/v2/balance
func (h *Handler) GetBalance(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	balance, err := h.tradingService.GetBalance(account.ID, models.ExchangeBinance)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	// Format as Binance response
	c.JSON(200, []gin.H{
		{
			"accountAlias":       "SgsR",
			"asset":              "USDT",
			"balance":            strconv.FormatFloat(balance["balance"], 'f', 8, 64),
			"crossWalletBalance": strconv.FormatFloat(balance["balance"], 'f', 8, 64),
			"crossUnPnl":         strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
			"availableBalance":   strconv.FormatFloat(balance["available"], 'f', 8, 64),
			"maxWithdrawAmount":  strconv.FormatFloat(balance["available"], 'f', 8, 64),
			"marginAvailable":    true,
			"updateTime":         time.Now().UnixMilli(),
		},
	})
}

// GetPositionRisk handles GET /fapi/v2/positionRisk
func (h *Handler) GetPositionRisk(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.Query("symbol")
	positions, err := h.tradingService.GetPositions(account.ID, models.ExchangeBinance)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	result := make([]gin.H, 0)
	for _, pos := range positions {
		if symbol != "" && pos.Symbol != symbol {
			continue
		}
		result = append(result, gin.H{
			"symbol":           pos.Symbol,
			"positionAmt":      strconv.FormatFloat(pos.Quantity, 'f', 8, 64),
			"entryPrice":       strconv.FormatFloat(pos.EntryPrice, 'f', 8, 64),
			"markPrice":        strconv.FormatFloat(pos.MarkPrice, 'f', 8, 64),
			"unRealizedProfit": strconv.FormatFloat(pos.UnrealizedPnL, 'f', 8, 64),
			"liquidationPrice": strconv.FormatFloat(pos.LiquidationPrice, 'f', 8, 64),
			"leverage":         strconv.Itoa(pos.Leverage),
			"marginType":       string(pos.MarginMode),
			"isolatedMargin":   strconv.FormatFloat(pos.Margin, 'f', 8, 64),
			"isAutoAddMargin":  "false",
			"positionSide":     string(pos.Side),
			"updateTime":       pos.UpdatedAt.UnixMilli(),
		})
	}

	c.JSON(200, result)
}

// CreateOrder handles POST /fapi/v1/order
func (h *Handler) CreateOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	// Parse request
	symbol := c.PostForm("symbol")
	side := c.PostForm("side")
	positionSide := c.PostForm("positionSide")
	orderType := c.PostForm("type")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	reduceOnly := c.PostForm("reduceOnly") == "true"
	closePosition := c.PostForm("closePosition") == "true"

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	// Determine if opening or closing
	isOpen := !reduceOnly && !closePosition

	// Map position side
	var posSide models.PositionSide
	if positionSide == "LONG" {
		posSide = models.PositionSideLong
	} else if positionSide == "SHORT" {
		posSide = models.PositionSideShort
	} else {
		// One-way mode: infer from side
		if side == "BUY" {
			posSide = models.PositionSideLong
		} else {
			posSide = models.PositionSideShort
		}
	}

	// Map order type
	var oType models.OrderType
	switch orderType {
	case "LIMIT":
		oType = models.OrderTypeLimit
	case "STOP", "STOP_MARKET":
		oType = models.OrderTypeStopMarket
	case "TAKE_PROFIT", "TAKE_PROFIT_MARKET":
		oType = models.OrderTypeTakeProfit
	default:
		oType = models.OrderTypeMarket
	}

	var order *models.Order
	var err error

	if isOpen {
		// Open position
		req := &service.OpenPositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  quantity,
			OrderType: oType,
			Price:     price,
		}
		order, _, err = h.tradingService.OpenPosition(req, models.ExchangeBinance)
	} else {
		// Close position
		req := &service.ClosePositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  &quantity,
			OrderType: oType,
			Price:     price,
		}
		order, _, err = h.tradingService.ClosePosition(req, models.ExchangeBinance)
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	// Format as Binance response
	c.JSON(200, gin.H{
		"orderId":       order.ID,
		"symbol":        order.Symbol,
		"status":        string(order.Status),
		"clientOrderId": order.ClientOrderID,
		"price":         strconv.FormatFloat(order.Price, 'f', 8, 64),
		"avgPrice":      strconv.FormatFloat(order.AvgPrice, 'f', 8, 64),
		"origQty":       strconv.FormatFloat(order.Quantity, 'f', 8, 64),
		"executedQty":   strconv.FormatFloat(order.FilledQty, 'f', 8, 64),
		"type":          string(order.Type),
		"side":          string(order.Side),
		"positionSide":  string(order.PositionSide),
		"reduceOnly":    order.ReduceOnly,
		"closePosition": order.ClosePosition,
		"updateTime":    order.UpdatedAt.UnixMilli(),
	})
}

// CancelOrder handles DELETE /fapi/v1/order
func (h *Handler) CancelOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.Query("symbol")
	orderIDStr := c.Query("orderId")

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	if orderIDStr == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'orderId' was not sent."})
		return
	}

	orderID, _ := strconv.ParseUint(orderIDStr, 10, 64)
	order, err := h.tradingService.GetOrderStatus(account.ID, uint(orderID))
	if err != nil {
		c.JSON(400, gin.H{"code": -2011, "msg": "Unknown order sent."})
		return
	}

	c.JSON(200, gin.H{
		"orderId":       order.ID,
		"symbol":        order.Symbol,
		"status":        "CANCELED",
		"clientOrderId": order.ClientOrderID,
		"origQty":       strconv.FormatFloat(order.Quantity, 'f', 8, 64),
		"executedQty":   strconv.FormatFloat(order.FilledQty, 'f', 8, 64),
		"type":          string(order.Type),
		"side":          string(order.Side),
		"updateTime":    time.Now().UnixMilli(),
	})
}

// CancelAllOpenOrders handles DELETE /fapi/v1/allOpenOrders
func (h *Handler) CancelAllOpenOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	count, err := h.tradingService.CancelAllOrders(account.ID, symbol)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "The operation of cancel all open orders is done.",
		"data": count,
	})
}

// SetLeverage handles POST /fapi/v1/leverage
func (h *Handler) SetLeverage(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.PostForm("symbol")
	leverage, _ := strconv.Atoi(c.PostForm("leverage"))

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	if err := h.tradingService.SetLeverage(account.ID, symbol, leverage); err != nil {
		c.JSON(400, gin.H{"code": -4028, "msg": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"leverage":         leverage,
		"maxNotionalValue": "1000000",
		"symbol":           symbol,
	})
}

// GetMarkPrice handles GET /fapi/v1/premiumIndex
func (h *Handler) GetMarkPrice(c *gin.Context) {
	symbol := c.Query("symbol")

	price, err := h.priceService.GetPrice("binance", symbol)
	if err != nil {
		c.JSON(400, gin.H{"code": -1121, "msg": "Invalid symbol."})
		return
	}

	c.JSON(200, gin.H{
		"symbol":               symbol,
		"markPrice":            strconv.FormatFloat(price, 'f', 8, 64),
		"indexPrice":           strconv.FormatFloat(price, 'f', 8, 64),
		"estimatedSettlePrice": strconv.FormatFloat(price, 'f', 8, 64),
		"lastFundingRate":      "0.00010000",
		"nextFundingTime":      time.Now().Add(8 * time.Hour).UnixMilli(),
		"time":                 time.Now().UnixMilli(),
	})
}

// GetQueryOrder handles GET /fapi/v1/order
func (h *Handler) GetQueryOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	orderIDStr := c.Query("orderId")
	orderID, _ := strconv.ParseUint(orderIDStr, 10, 64)

	order, err := h.tradingService.GetOrderStatus(account.ID, uint(orderID))
	if err != nil {
		c.JSON(400, gin.H{"code": -2013, "msg": "Order does not exist."})
		return
	}

	c.JSON(200, gin.H{
		"orderId":       order.ID,
		"symbol":        order.Symbol,
		"status":        string(order.Status),
		"clientOrderId": order.ClientOrderID,
		"price":         strconv.FormatFloat(order.Price, 'f', 8, 64),
		"avgPrice":      strconv.FormatFloat(order.AvgPrice, 'f', 8, 64),
		"origQty":       strconv.FormatFloat(order.Quantity, 'f', 8, 64),
		"executedQty":   strconv.FormatFloat(order.FilledQty, 'f', 8, 64),
		"cumQuote":      strconv.FormatFloat(order.AvgPrice*order.FilledQty, 'f', 8, 64),
		"type":          string(order.Type),
		"side":          string(order.Side),
		"positionSide":  string(order.PositionSide),
		"reduceOnly":    order.ReduceOnly,
		"closePosition": order.ClosePosition,
		"time":          order.CreatedAt.UnixMilli(),
		"updateTime":    order.UpdatedAt.UnixMilli(),
	})
}

// GetExchangeInfo handles GET /fapi/v1/exchangeInfo
func (h *Handler) GetExchangeInfo(c *gin.Context) {
	// Return minimal exchange info
	c.JSON(200, gin.H{
		"timezone":   "UTC",
		"serverTime": time.Now().UnixMilli(),
		"symbols":    []gin.H{}, // Would be populated with actual symbol data
	})
}

// handleError maps service errors to Binance error codes
func (h *Handler) handleError(c *gin.Context, err error) {
	switch err {
	case service.ErrInsufficientBalance:
		c.JSON(400, gin.H{"code": -2019, "msg": "Margin is insufficient."})
	case service.ErrInvalidSymbol:
		c.JSON(400, gin.H{"code": -1121, "msg": "Invalid symbol."})
	case service.ErrInvalidQuantity:
		c.JSON(400, gin.H{"code": -1013, "msg": "Invalid quantity."})
	case service.ErrNoOpenPosition:
		c.JSON(400, gin.H{"code": -2022, "msg": "Position side not match."})
	default:
		c.JSON(500, gin.H{"code": -1, "msg": err.Error()})
	}
}

// RegisterRoutes registers Binance-compatible routes
func (h *Handler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	fapi := router.Group("/fapi")
	fapi.Use(authMiddleware)
	{
		// V1 endpoints
		v1 := fapi.Group("/v1")
		{
			v1.POST("/order", h.CreateOrder)
			v1.DELETE("/order", h.CancelOrder)
			v1.GET("/order", h.GetQueryOrder)
			v1.DELETE("/allOpenOrders", h.CancelAllOpenOrders)
			v1.POST("/leverage", h.SetLeverage)
			v1.GET("/premiumIndex", h.GetMarkPrice)
			v1.GET("/exchangeInfo", h.GetExchangeInfo)
		}

		// V2 endpoints
		v2 := fapi.Group("/v2")
		{
			v2.GET("/balance", h.GetBalance)
			v2.GET("/positionRisk", h.GetPositionRisk)
		}
	}
}
