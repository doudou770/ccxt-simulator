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
	tradingService      *service.TradingService
	priceService        *service.PriceService
	exchangeInfoService *service.ExchangeInfoService
}

// NewHandler creates a new Binance handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService, exchangeInfoService *service.ExchangeInfoService) *Handler {
	return &Handler{
		tradingService:      tradingService,
		priceService:        priceService,
		exchangeInfoService: exchangeInfoService,
	}
}

// GetTime handles GET /fapi/v1/time
func (h *Handler) GetTime(c *gin.Context) {
	c.JSON(200, gin.H{
		"serverTime": time.Now().UnixMilli(),
	})
}

// GetAccount handles GET /fapi/v2/account
func (h *Handler) GetAccount(c *gin.Context) {
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

	positions, _ := h.tradingService.GetPositions(account.ID, models.ExchangeBinance)

	positionList := make([]gin.H, 0)
	for _, pos := range positions {
		positionList = append(positionList, gin.H{
			"symbol":           pos.Symbol,
			"positionAmt":      strconv.FormatFloat(pos.Quantity, 'f', 8, 64),
			"entryPrice":       strconv.FormatFloat(pos.EntryPrice, 'f', 8, 64),
			"markPrice":        strconv.FormatFloat(pos.MarkPrice, 'f', 8, 64),
			"unRealizedProfit": strconv.FormatFloat(pos.UnrealizedPnL, 'f', 8, 64),
			"liquidationPrice": strconv.FormatFloat(pos.LiquidationPrice, 'f', 8, 64),
			"leverage":         strconv.Itoa(pos.Leverage),
			"marginType":       string(pos.MarginMode),
			"positionSide":     string(pos.Side),
			"updateTime":       pos.UpdatedAt.UnixMilli(),
		})
	}

	c.JSON(200, gin.H{
		"feeTier":                     0,
		"canTrade":                    true,
		"canDeposit":                  true,
		"canWithdraw":                 true,
		"updateTime":                  time.Now().UnixMilli(),
		"totalInitialMargin":          strconv.FormatFloat(balance["margin"], 'f', 8, 64),
		"totalMaintMargin":            strconv.FormatFloat(balance["margin"]*0.5, 'f', 8, 64),
		"totalWalletBalance":          strconv.FormatFloat(balance["balance"], 'f', 8, 64),
		"totalUnrealizedProfit":       strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
		"totalMarginBalance":          strconv.FormatFloat(balance["equity"], 'f', 8, 64),
		"totalPositionInitialMargin":  strconv.FormatFloat(balance["margin"], 'f', 8, 64),
		"totalOpenOrderInitialMargin": "0",
		"totalCrossWalletBalance":     strconv.FormatFloat(balance["balance"], 'f', 8, 64),
		"totalCrossUnPnl":             strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
		"availableBalance":            strconv.FormatFloat(balance["available"], 'f', 8, 64),
		"maxWithdrawAmount":           strconv.FormatFloat(balance["available"], 'f', 8, 64),
		"assets": []gin.H{
			{
				"asset":                  "USDT",
				"walletBalance":          strconv.FormatFloat(balance["balance"], 'f', 8, 64),
				"unrealizedProfit":       strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
				"marginBalance":          strconv.FormatFloat(balance["equity"], 'f', 8, 64),
				"maintMargin":            strconv.FormatFloat(balance["margin"]*0.5, 'f', 8, 64),
				"initialMargin":          strconv.FormatFloat(balance["margin"], 'f', 8, 64),
				"positionInitialMargin":  strconv.FormatFloat(balance["margin"], 'f', 8, 64),
				"openOrderInitialMargin": "0",
				"maxWithdrawAmount":      strconv.FormatFloat(balance["available"], 'f', 8, 64),
				"crossWalletBalance":     strconv.FormatFloat(balance["balance"], 'f', 8, 64),
				"crossUnPnl":             strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
				"availableBalance":       strconv.FormatFloat(balance["available"], 'f', 8, 64),
				"marginAvailable":        true,
				"updateTime":             time.Now().UnixMilli(),
			},
		},
		"positions": positionList,
	})
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

// SetMarginType handles POST /fapi/v1/marginType
func (h *Handler) SetMarginType(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.PostForm("symbol")
	marginType := c.PostForm("marginType")

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "success",
	})
	_ = marginType
}

// GetOpenOrders handles GET /fapi/v1/openOrders
func (h *Handler) GetOpenOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.Query("symbol")
	orders, _ := h.tradingService.GetOpenOrders(account.ID, symbol)

	result := make([]gin.H, 0)
	for _, order := range orders {
		result = append(result, h.formatOrder(&order))
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

	symbol := c.PostForm("symbol")
	side := c.PostForm("side")
	positionSide := c.PostForm("positionSide")
	orderType := c.PostForm("type")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	stopPrice, _ := strconv.ParseFloat(c.PostForm("stopPrice"), 64)
	reduceOnly := c.PostForm("reduceOnly") == "true"
	closePosition := c.PostForm("closePosition") == "true"

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	var posSide models.PositionSide
	if positionSide == "LONG" {
		posSide = models.PositionSideLong
	} else if positionSide == "SHORT" {
		posSide = models.PositionSideShort
	} else {
		if side == "BUY" {
			posSide = models.PositionSideLong
		} else {
			posSide = models.PositionSideShort
		}
	}

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

	// Check if this is a conditional order (SL/TP)
	// STOP_MARKET and TAKE_PROFIT are conditional orders that should NOT execute immediately
	// They should only create an order with status NEW and wait for price trigger
	isConditionalOrder := oType == models.OrderTypeStopMarket || oType == models.OrderTypeTakeProfit

	// Determine if this is a close position order
	// In hedge mode: LONG+SELL or SHORT+BUY = close position
	// In one-way mode (positionSide=BOTH or empty): use reduceOnly/closePosition flags
	isClosing := false
	if positionSide == "LONG" && side == "SELL" {
		// Hedge mode: selling on LONG position side = closing long
		isClosing = true
	} else if positionSide == "SHORT" && side == "BUY" {
		// Hedge mode: buying on SHORT position side = closing short
		isClosing = true
	} else if reduceOnly || closePosition {
		// One-way mode or explicit reduce-only flag
		isClosing = true
	}

	// Open position if not closing and not a conditional order
	isOpen := !isClosing && !isConditionalOrder

	var order *models.Order
	var err error

	if isConditionalOrder {
		// For conditional orders (SL/TP), just create the order without executing
		// The order will be triggered when price reaches the stop price
		order, err = h.tradingService.CreateConditionalOrder(&service.ConditionalOrderRequest{
			AccountID:     account.ID,
			Symbol:        symbol,
			Side:          posSide,
			Quantity:      quantity,
			OrderType:     oType,
			StopPrice:     stopPrice,
			Price:         price,
			ReduceOnly:    reduceOnly,
			ClosePosition: closePosition,
		}, models.ExchangeBinance)
	} else if isOpen {
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
		req := &service.ClosePositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  &quantity,
			OrderType: oType,
			Price:     price,
			StopPrice: stopPrice,
		}
		order, _, err = h.tradingService.ClosePosition(req, models.ExchangeBinance)
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, h.formatOrder(order))
}

// CreateAlgoOrder handles POST /fapi/v1/algoOrder (new Binance algo order API)
func (h *Handler) CreateAlgoOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.PostForm("symbol")
	_ = c.PostForm("side") // side is inferred from positionSide
	positionSide := c.PostForm("positionSide")
	orderType := c.PostForm("orderType") // algoOrder uses orderType instead of type
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	triggerPrice, _ := strconv.ParseFloat(c.PostForm("triggerPrice"), 64) // new param name
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	closePosition := c.PostForm("closePosition") == "true"
	reduceOnly := c.PostForm("reduceOnly") == "true"

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	var posSide models.PositionSide
	if positionSide == "LONG" {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	var oType models.OrderType
	switch orderType {
	case "STOP", "STOP_MARKET":
		oType = models.OrderTypeStopMarket
	case "TAKE_PROFIT", "TAKE_PROFIT_MARKET":
		oType = models.OrderTypeTakeProfit
	case "TRAILING_STOP_MARKET":
		oType = models.OrderTypeTrailingStop
	default:
		oType = models.OrderTypeStopMarket
	}

	// Algo orders are conditional orders (SL/TP) - they should NOT execute immediately
	// They just create an order entry and wait for price trigger
	order, err := h.tradingService.CreateConditionalOrder(&service.ConditionalOrderRequest{
		AccountID:     account.ID,
		Symbol:        symbol,
		Side:          posSide,
		Quantity:      quantity,
		OrderType:     oType,
		StopPrice:     triggerPrice,
		Price:         price,
		ClosePosition: closePosition,
		ReduceOnly:    reduceOnly,
	}, models.ExchangeBinance)

	if err != nil {
		h.handleError(c, err)
		return
	}

	// Return algo order format
	c.JSON(200, gin.H{
		"clientAlgoId": order.ClientOrderID,
		"algoId":       order.ID,
		"success":      true,
		"code":         "200",
		"msg":          "OK",
	})
}

// GetOpenAlgoOrders handles GET /fapi/v1/openAlgoOrders
func (h *Handler) GetOpenAlgoOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.Query("symbol")
	orders, _ := h.tradingService.GetOpenAlgoOrders(account.ID, symbol)

	result := make([]gin.H, 0)
	for _, order := range orders {
		result = append(result, gin.H{
			"algoId":        order.ID,
			"clientAlgoId":  order.ClientOrderID,
			"symbol":        order.Symbol,
			"side":          string(order.Side),
			"positionSide":  string(order.PositionSide),
			"orderType":     string(order.Type),
			"triggerPrice":  strconv.FormatFloat(order.StopPrice, 'f', 8, 64),
			"quantity":      strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"reduceOnly":    order.ReduceOnly,
			"closePosition": order.ClosePosition,
			"algoStatus":    "NEW",
			"bookTime":      order.CreatedAt.UnixMilli(),
			"updateTime":    order.UpdatedAt.UnixMilli(),
		})
	}

	c.JSON(200, gin.H{
		"total":  len(result),
		"orders": result,
	})
}

// CancelAlgoOrder handles DELETE /fapi/v1/algoOrder
func (h *Handler) CancelAlgoOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"code": -2015, "msg": "Invalid API-key."})
		return
	}

	symbol := c.Query("symbol")
	algoIDStr := c.Query("algoId")

	if symbol == "" {
		c.JSON(400, gin.H{"code": -1102, "msg": "Mandatory parameter 'symbol' was not sent."})
		return
	}

	algoID, _ := strconv.ParseUint(algoIDStr, 10, 64)

	c.JSON(200, gin.H{
		"algoId":  algoID,
		"success": true,
		"code":    "200",
		"msg":     "OK",
	})
}

// CancelAllOpenAlgoOrders handles DELETE /fapi/v1/allOpenAlgoOrders
func (h *Handler) CancelAllOpenAlgoOrders(c *gin.Context) {
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

	count, _ := h.tradingService.CancelAllAlgoOrders(account.ID, symbol)

	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "The operation of cancel all open algo orders is done.",
		"data": count,
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

	// Parse form data from body
	c.Request.ParseForm()

	// Try to get symbol from query first, then from form body
	symbol := c.Query("symbol")
	if symbol == "" {
		symbol = c.PostForm("symbol")
	}

	// Try to get leverage from query first, then from form body
	leverageStr := c.Query("leverage")
	if leverageStr == "" {
		leverageStr = c.PostForm("leverage")
	}
	leverage, _ := strconv.Atoi(leverageStr)

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

// GetTickerPrice handles GET /fapi/v2/ticker/price
func (h *Handler) GetTickerPrice(c *gin.Context) {
	symbol := c.Query("symbol")

	if symbol != "" {
		price, err := h.priceService.GetPrice("binance", symbol)
		if err != nil {
			c.JSON(400, gin.H{"code": -1121, "msg": "Invalid symbol."})
			return
		}
		c.JSON(200, gin.H{
			"symbol": symbol,
			"price":  strconv.FormatFloat(price, 'f', 8, 64),
			"time":   time.Now().UnixMilli(),
		})
		return
	}

	// Return all prices
	prices := h.priceService.GetAllPrices("binance")
	result := make([]gin.H, 0)
	for sym, price := range prices {
		result = append(result, gin.H{
			"symbol": sym,
			"price":  strconv.FormatFloat(price, 'f', 8, 64),
			"time":   time.Now().UnixMilli(),
		})
	}
	c.JSON(200, result)
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

	c.JSON(200, h.formatOrder(order))
}

// GetExchangeInfo handles GET /fapi/v1/exchangeInfo
func (h *Handler) GetExchangeInfo(c *gin.Context) {
	if h.exchangeInfoService != nil {
		data, err := h.exchangeInfoService.GetExchangeInfo("binance")
		if err == nil && data != nil {
			c.JSON(200, data)
			return
		}
	}

	// Fallback
	c.JSON(200, gin.H{
		"timezone":        "UTC",
		"serverTime":      time.Now().UnixMilli(),
		"futuresType":     "U_MARGINED",
		"rateLimits":      []gin.H{},
		"exchangeFilters": []gin.H{},
		"symbols":         []gin.H{},
	})
}

// formatOrder formats an order for Binance response
func (h *Handler) formatOrder(order *models.Order) gin.H {
	return gin.H{
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
		"stopPrice":     strconv.FormatFloat(order.StopPrice, 'f', 8, 64),
		"reduceOnly":    order.ReduceOnly,
		"closePosition": order.ClosePosition,
		"time":          order.CreatedAt.UnixMilli(),
		"updateTime":    order.UpdatedAt.UnixMilli(),
	}
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

	// Public endpoints (no auth)
	fapi.GET("/v1/time", h.GetTime)
	fapi.GET("/v1/exchangeInfo", h.GetExchangeInfo)
	fapi.GET("/v1/premiumIndex", h.GetMarkPrice)
	fapi.GET("/v2/ticker/price", h.GetTickerPrice)

	// Private endpoints (require auth)
	fapi.Use(authMiddleware)
	{
		// V1 endpoints
		v1 := fapi.Group("/v1")
		{
			v1.POST("/order", h.CreateOrder)
			v1.DELETE("/order", h.CancelOrder)
			v1.GET("/order", h.GetQueryOrder)
			v1.GET("/openOrders", h.GetOpenOrders)
			v1.DELETE("/allOpenOrders", h.CancelAllOpenOrders)
			v1.POST("/leverage", h.SetLeverage)
			v1.POST("/marginType", h.SetMarginType)
			// Algo orders (SL/TP)
			v1.POST("/algoOrder", h.CreateAlgoOrder)
			v1.DELETE("/algoOrder", h.CancelAlgoOrder)
			v1.GET("/openAlgoOrders", h.GetOpenAlgoOrders)
			v1.DELETE("/allOpenAlgoOrders", h.CancelAllOpenAlgoOrders)
		}

		// V2 endpoints
		v2 := fapi.Group("/v2")
		{
			v2.GET("/account", h.GetAccount)
			v2.GET("/balance", h.GetBalance)
			v2.GET("/positionRisk", h.GetPositionRisk)
		}
	}
}
