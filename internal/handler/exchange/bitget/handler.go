package bitget

import (
	"strconv"
	"time"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler handles Bitget-compatible API requests
type Handler struct {
	tradingService      *service.TradingService
	priceService        *service.PriceService
	exchangeInfoService *service.ExchangeInfoService
}

// NewHandler creates a new Bitget handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService, exchangeInfoService *service.ExchangeInfoService) *Handler {
	return &Handler{
		tradingService:      tradingService,
		priceService:        priceService,
		exchangeInfoService: exchangeInfoService,
	}
}

// GetServerTime handles GET /api/v2/public/time
func (h *Handler) GetServerTime(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"serverTime": strconv.FormatInt(time.Now().UnixMilli(), 10),
		},
	})
}

// GetContracts handles GET /api/v2/mix/market/contracts
func (h *Handler) GetContracts(c *gin.Context) {
	if h.exchangeInfoService != nil {
		data, err := h.exchangeInfoService.GetExchangeInfo("bitget")
		if err == nil && data != nil {
			c.JSON(200, data)
			return
		}
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data":        []gin.H{},
	})
}

// GetAccount handles GET /api/v2/mix/account/account
func (h *Handler) GetAccount(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	balance, err := h.tradingService.GetBalance(account.ID, models.ExchangeBitget)
	if err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"marginCoin":        "USDT",
			"locked":            "0",
			"available":         strconv.FormatFloat(balance["available"], 'f', 8, 64),
			"crossMaxAvailable": strconv.FormatFloat(balance["available"], 'f', 8, 64),
			"fixedMaxAvailable": strconv.FormatFloat(balance["available"], 'f', 8, 64),
			"maxTransferOut":    strconv.FormatFloat(balance["available"], 'f', 8, 64),
			"equity":            strconv.FormatFloat(balance["equity"], 'f', 8, 64),
			"usdtEquity":        strconv.FormatFloat(balance["equity"], 'f', 8, 64),
			"accountBalance":    strconv.FormatFloat(balance["balance"], 'f', 8, 64),
			"unrealizedPL":      strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
		},
	})
}

// GetPositions handles GET /api/v2/mix/position/all-position
func (h *Handler) GetPositions(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	positions, err := h.tradingService.GetPositions(account.ID, models.ExchangeBitget)
	if err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	data := make([]gin.H, 0)
	for _, pos := range positions {
		holdSide := "long"
		if pos.Side == models.PositionSideShort {
			holdSide = "short"
		}

		data = append(data, gin.H{
			"symbol":            pos.Symbol,
			"marginCoin":        "USDT",
			"holdSide":          holdSide,
			"openDelegateCount": "0",
			"margin":            strconv.FormatFloat(pos.Margin, 'f', 8, 64),
			"available":         strconv.FormatFloat(pos.Quantity, 'f', 8, 64),
			"locked":            "0",
			"total":             strconv.FormatFloat(pos.Quantity, 'f', 8, 64),
			"leverage":          strconv.Itoa(pos.Leverage),
			"achievedProfits":   "0",
			"averageOpenPrice":  strconv.FormatFloat(pos.EntryPrice, 'f', 8, 64),
			"marginMode":        string(pos.MarginMode),
			"holdMode":          "single_hold",
			"unrealizedPL":      strconv.FormatFloat(pos.UnrealizedPnL, 'f', 8, 64),
			"liquidationPrice":  strconv.FormatFloat(pos.LiquidationPrice, 'f', 8, 64),
			"keepMarginRate":    "0.004",
			"marketPrice":       strconv.FormatFloat(pos.MarkPrice, 'f', 8, 64),
			"cTime":             strconv.FormatInt(pos.CreatedAt.UnixMilli(), 10),
			"uTime":             strconv.FormatInt(pos.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data":        data,
	})
}

// PlaceOrder handles POST /api/v2/mix/order/place-order
func (h *Handler) PlaceOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	var req struct {
		Symbol      string `json:"symbol"`
		ProductType string `json:"productType"`
		MarginMode  string `json:"marginMode"`
		MarginCoin  string `json:"marginCoin"`
		Size        string `json:"size"`
		Price       string `json:"price"`
		Side        string `json:"side"`
		TradeSide   string `json:"tradeSide"`
		OrderType   string `json:"orderType"`
		ReduceOnly  string `json:"reduceOnly"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "40001", err.Error())
		return
	}

	quantity, _ := strconv.ParseFloat(req.Size, 64)
	price, _ := strconv.ParseFloat(req.Price, 64)

	var posSide models.PositionSide
	if req.TradeSide == "open" {
		if req.Side == "buy" {
			posSide = models.PositionSideLong
		} else {
			posSide = models.PositionSideShort
		}
	} else {
		if req.Side == "sell" {
			posSide = models.PositionSideLong
		} else {
			posSide = models.PositionSideShort
		}
	}

	var orderType models.OrderType
	if req.OrderType == "limit" {
		orderType = models.OrderTypeLimit
	} else {
		orderType = models.OrderTypeMarket
	}

	isClose := req.TradeSide == "close" || req.ReduceOnly == "YES"
	var order *models.Order
	var err error

	if !isClose {
		openReq := &service.OpenPositionRequest{
			AccountID: account.ID,
			Symbol:    req.Symbol,
			Side:      posSide,
			Quantity:  quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.OpenPosition(openReq, models.ExchangeBitget)
	} else {
		closeReq := &service.ClosePositionRequest{
			AccountID: account.ID,
			Symbol:    req.Symbol,
			Side:      posSide,
			Quantity:  &quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.ClosePosition(closeReq, models.ExchangeBitget)
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"orderId":   strconv.Itoa(int(order.ID)),
			"clientOid": order.ClientOrderID,
		},
	})
}

// PlacePlanOrder handles POST /api/v2/mix/order/place-plan-order (SL/TP)
func (h *Handler) PlacePlanOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	var req struct {
		Symbol       string `json:"symbol"`
		ProductType  string `json:"productType"`
		MarginMode   string `json:"marginMode"`
		MarginCoin   string `json:"marginCoin"`
		Size         string `json:"size"`
		Price        string `json:"price"`
		Side         string `json:"side"`
		TradeSide    string `json:"tradeSide"`
		OrderType    string `json:"orderType"`
		TriggerPrice string `json:"triggerPrice"`
		TriggerType  string `json:"triggerType"` // mark_price, fill_price
		PlanType     string `json:"planType"`    // normal_plan, profit_plan, loss_plan
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "40001", err.Error())
		return
	}

	quantity, _ := strconv.ParseFloat(req.Size, 64)
	triggerPrice, _ := strconv.ParseFloat(req.TriggerPrice, 64)

	var posSide models.PositionSide
	if req.Side == "buy" {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	var orderType models.OrderType
	switch req.PlanType {
	case "loss_plan":
		orderType = models.OrderTypeStopMarket
	case "profit_plan":
		orderType = models.OrderTypeTakeProfit
	default:
		orderType = models.OrderTypeStopMarket
	}

	closeReq := &service.ClosePositionRequest{
		AccountID:  account.ID,
		Symbol:     req.Symbol,
		Side:       posSide,
		Quantity:   &quantity,
		OrderType:  orderType,
		StopPrice:  triggerPrice,
		ReduceOnly: true,
	}

	order, _, err := h.tradingService.ClosePosition(closeReq, models.ExchangeBitget)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"orderId":   strconv.Itoa(int(order.ID)),
			"clientOid": order.ClientOrderID,
		},
	})
}

// GetOpenOrders handles GET /api/v2/mix/order/orders-pending
func (h *Handler) GetOpenOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	symbol := c.Query("symbol")
	orders, _ := h.tradingService.GetOpenOrders(account.ID, symbol)

	data := make([]gin.H, 0)
	for _, order := range orders {
		side := "buy"
		if order.Side == models.OrderSideSell {
			side = "sell"
		}

		data = append(data, gin.H{
			"orderId":   strconv.Itoa(int(order.ID)),
			"clientOid": order.ClientOrderID,
			"symbol":    order.Symbol,
			"side":      side,
			"orderType": string(order.Type),
			"price":     strconv.FormatFloat(order.Price, 'f', 8, 64),
			"size":      strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"filledQty": strconv.FormatFloat(order.FilledQty, 'f', 8, 64),
			"state":     string(order.Status),
			"cTime":     strconv.FormatInt(order.CreatedAt.UnixMilli(), 10),
			"uTime":     strconv.FormatInt(order.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"entrustedList": data,
		},
	})
}

// GetPendingPlanOrders handles GET /api/v2/mix/order/orders-plan-pending (SL/TP orders)
func (h *Handler) GetPendingPlanOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	symbol := c.Query("symbol")
	orders, _ := h.tradingService.GetOpenAlgoOrders(account.ID, symbol)

	data := make([]gin.H, 0)
	for _, order := range orders {
		planType := "loss_plan"
		if order.Type == models.OrderTypeTakeProfit {
			planType = "profit_plan"
		}

		data = append(data, gin.H{
			"orderId":      strconv.Itoa(int(order.ID)),
			"clientOid":    order.ClientOrderID,
			"symbol":       order.Symbol,
			"planType":     planType,
			"triggerPrice": strconv.FormatFloat(order.StopPrice, 'f', 8, 64),
			"size":         strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"state":        "not_trigger",
			"cTime":        strconv.FormatInt(order.CreatedAt.UnixMilli(), 10),
			"uTime":        strconv.FormatInt(order.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"entrustedList": data,
		},
	})
}

// CancelPlanOrder handles POST /api/v2/mix/order/cancel-plan-order
func (h *Handler) CancelPlanOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	var req struct {
		Symbol      string `json:"symbol"`
		OrderId     string `json:"orderId"`
		ProductType string `json:"productType"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "40001", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"orderId":   req.OrderId,
			"clientOid": "",
		},
	})
}

// CancelOrder handles POST /api/v2/mix/order/cancel-order
func (h *Handler) CancelOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	var req struct {
		Symbol      string `json:"symbol"`
		OrderId     string `json:"orderId"`
		ProductType string `json:"productType"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "40001", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"orderId":   req.OrderId,
			"clientOid": "",
		},
	})
}

// CancelAllOrders handles POST /api/v2/mix/order/cancel-all-orders
func (h *Handler) CancelAllOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	var req struct {
		Symbol      string `json:"symbol"`
		ProductType string `json:"productType"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "40001", err.Error())
		return
	}

	count, _ := h.tradingService.CancelAllOrders(account.ID, req.Symbol)

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"successList": []gin.H{},
			"failureList": []gin.H{},
			"result":      count > 0,
		},
	})
}

// SetLeverage handles POST /api/v2/mix/account/set-leverage
func (h *Handler) SetLeverage(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "40001", "Invalid API key")
		return
	}

	var req struct {
		Symbol      string `json:"symbol"`
		ProductType string `json:"productType"`
		MarginCoin  string `json:"marginCoin"`
		Leverage    string `json:"leverage"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "40001", err.Error())
		return
	}

	leverage, _ := strconv.Atoi(req.Leverage)
	if err := h.tradingService.SetLeverage(account.ID, req.Symbol, leverage); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data": gin.H{
			"symbol":        req.Symbol,
			"marginCoin":    req.MarginCoin,
			"longLeverage":  req.Leverage,
			"shortLeverage": req.Leverage,
		},
	})
}

// GetTicker handles GET /api/v2/mix/market/ticker
func (h *Handler) GetTicker(c *gin.Context) {
	symbol := c.Query("symbol")

	if symbol != "" {
		price, err := h.priceService.GetPrice("bitget", symbol)
		if err != nil {
			h.errorResponse(c, "40001", "Invalid symbol")
			return
		}

		c.JSON(200, gin.H{
			"code":        "00000",
			"msg":         "success",
			"requestTime": time.Now().UnixMilli(),
			"data": []gin.H{
				{
					"symbol":          symbol,
					"lastPr":          strconv.FormatFloat(price, 'f', 8, 64),
					"markPrice":       strconv.FormatFloat(price, 'f', 8, 64),
					"indexPrice":      strconv.FormatFloat(price, 'f', 8, 64),
					"high24h":         strconv.FormatFloat(price*1.02, 'f', 8, 64),
					"low24h":          strconv.FormatFloat(price*0.98, 'f', 8, 64),
					"fundingRate":     "0.0001",
					"nextFundingTime": strconv.FormatInt(time.Now().Add(8*time.Hour).UnixMilli(), 10),
					"ts":              strconv.FormatInt(time.Now().UnixMilli(), 10),
				},
			},
		})
		return
	}

	// Return all prices
	prices := h.priceService.GetAllPrices("bitget")
	data := make([]gin.H, 0)
	for sym, price := range prices {
		data = append(data, gin.H{
			"symbol":    sym,
			"lastPr":    strconv.FormatFloat(price, 'f', 8, 64),
			"markPrice": strconv.FormatFloat(price, 'f', 8, 64),
		})
	}

	c.JSON(200, gin.H{
		"code":        "00000",
		"msg":         "success",
		"requestTime": time.Now().UnixMilli(),
		"data":        data,
	})
}

// Helper functions

func (h *Handler) errorResponse(c *gin.Context, code, msg string) {
	c.JSON(200, gin.H{
		"code":        code,
		"msg":         msg,
		"requestTime": time.Now().UnixMilli(),
		"data":        nil,
	})
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch err {
	case service.ErrInsufficientBalance:
		h.errorResponse(c, "45110", "Insufficient balance")
	case service.ErrInvalidSymbol:
		h.errorResponse(c, "40018", "Invalid symbol")
	case service.ErrInvalidQuantity:
		h.errorResponse(c, "40012", "Invalid size")
	case service.ErrNoOpenPosition:
		h.errorResponse(c, "45112", "No position to close")
	default:
		h.errorResponse(c, "50000", err.Error())
	}
}

// RegisterRoutes registers Bitget-compatible routes
func (h *Handler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	api := router.Group("/api/v2")

	// Public endpoints
	publicApi := api.Group("/public")
	{
		publicApi.GET("/time", h.GetServerTime)
	}

	mixApi := api.Group("/mix")

	// Public market endpoints
	market := mixApi.Group("/market")
	{
		market.GET("/contracts", h.GetContracts)
		market.GET("/ticker", h.GetTicker)
	}

	// Private endpoints
	mixApi.Use(authMiddleware)
	{
		account := mixApi.Group("/account")
		{
			account.GET("/account", h.GetAccount)
			account.POST("/set-leverage", h.SetLeverage)
		}

		position := mixApi.Group("/position")
		{
			position.GET("/all-position", h.GetPositions)
		}

		order := mixApi.Group("/order")
		{
			order.POST("/place-order", h.PlaceOrder)
			order.POST("/cancel-order", h.CancelOrder)
			order.POST("/cancel-all-orders", h.CancelAllOrders)
			order.GET("/orders-pending", h.GetOpenOrders)
			// Plan orders (SL/TP)
			order.POST("/place-plan-order", h.PlacePlanOrder)
			order.POST("/cancel-plan-order", h.CancelPlanOrder)
			order.GET("/orders-plan-pending", h.GetPendingPlanOrders)
		}
	}
}
