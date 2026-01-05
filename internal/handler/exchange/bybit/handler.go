package bybit

import (
	"strconv"
	"time"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler handles Bybit-compatible API requests
type Handler struct {
	tradingService      *service.TradingService
	priceService        *service.PriceService
	exchangeInfoService *service.ExchangeInfoService
}

// NewHandler creates a new Bybit handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService, exchangeInfoService *service.ExchangeInfoService) *Handler {
	return &Handler{
		tradingService:      tradingService,
		priceService:        priceService,
		exchangeInfoService: exchangeInfoService,
	}
}

// GetServerTime handles GET /v5/market/time
func (h *Handler) GetServerTime(c *gin.Context) {
	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"timeSecond": strconv.FormatInt(time.Now().Unix(), 10),
			"timeNano":   strconv.FormatInt(time.Now().UnixNano(), 10),
		},
		"time": time.Now().UnixMilli(),
	})
}

// GetInstrumentsInfo handles GET /v5/market/instruments-info
func (h *Handler) GetInstrumentsInfo(c *gin.Context) {
	if h.exchangeInfoService != nil {
		data, err := h.exchangeInfoService.GetExchangeInfo("bybit")
		if err == nil && data != nil {
			c.JSON(200, data)
			return
		}
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"category": "linear",
			"list":     []gin.H{},
		},
		"time": time.Now().UnixMilli(),
	})
}

// GetWalletBalance handles GET /v5/account/wallet-balance
func (h *Handler) GetWalletBalance(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	balance, err := h.tradingService.GetBalance(account.ID, models.ExchangeBybit)
	if err != nil {
		h.errorResponse(c, 10000, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"list": []gin.H{
				{
					"accountType":           "UNIFIED",
					"accountIMRate":         "0",
					"accountMMRate":         "0",
					"totalEquity":           strconv.FormatFloat(balance["equity"], 'f', 8, 64),
					"totalWalletBalance":    strconv.FormatFloat(balance["balance"], 'f', 8, 64),
					"totalMarginBalance":    strconv.FormatFloat(balance["balance"], 'f', 8, 64),
					"totalAvailableBalance": strconv.FormatFloat(balance["available"], 'f', 8, 64),
					"totalPerpUPL":          strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
					"totalInitialMargin":    strconv.FormatFloat(balance["margin"], 'f', 8, 64),
					"coin": []gin.H{
						{
							"coin":                "USDT",
							"equity":              strconv.FormatFloat(balance["equity"], 'f', 8, 64),
							"walletBalance":       strconv.FormatFloat(balance["balance"], 'f', 8, 64),
							"availableToWithdraw": strconv.FormatFloat(balance["available"], 'f', 8, 64),
							"unrealisedPnl":       strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
							"cumRealisedPnl":      "0",
						},
					},
				},
			},
		},
		"time": time.Now().UnixMilli(),
	})
}

// GetPositionInfo handles GET /v5/position/list
func (h *Handler) GetPositionInfo(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	symbol := c.Query("symbol")
	positions, err := h.tradingService.GetPositions(account.ID, models.ExchangeBybit)
	if err != nil {
		h.errorResponse(c, 10000, err.Error())
		return
	}

	list := make([]gin.H, 0)
	for _, pos := range positions {
		if symbol != "" && pos.Symbol != symbol {
			continue
		}

		side := "Buy"
		if pos.Side == models.PositionSideShort {
			side = "Sell"
		}

		list = append(list, gin.H{
			"symbol":        pos.Symbol,
			"side":          side,
			"size":          strconv.FormatFloat(pos.Quantity, 'f', 8, 64),
			"avgPrice":      strconv.FormatFloat(pos.EntryPrice, 'f', 8, 64),
			"markPrice":     strconv.FormatFloat(pos.MarkPrice, 'f', 8, 64),
			"positionValue": strconv.FormatFloat(pos.MarkPrice*pos.Quantity, 'f', 8, 64),
			"leverage":      strconv.Itoa(pos.Leverage),
			"unrealisedPnl": strconv.FormatFloat(pos.UnrealizedPnL, 'f', 8, 64),
			"liqPrice":      strconv.FormatFloat(pos.LiquidationPrice, 'f', 8, 64),
			"tradeMode":     0,
			"positionIdx":   0,
			"riskId":        1,
			"createdTime":   strconv.FormatInt(pos.CreatedAt.UnixMilli(), 10),
			"updatedTime":   strconv.FormatInt(pos.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"category": "linear",
			"list":     list,
		},
		"time": time.Now().UnixMilli(),
	})
}

// CreateOrder handles POST /v5/order/create
func (h *Handler) CreateOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	var req struct {
		Category    string `json:"category"`
		Symbol      string `json:"symbol"`
		Side        string `json:"side"`
		OrderType   string `json:"orderType"`
		Qty         string `json:"qty"`
		Price       string `json:"price"`
		PositionIdx int    `json:"positionIdx"`
		ReduceOnly  bool   `json:"reduceOnly"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 10001, err.Error())
		return
	}

	quantity, _ := strconv.ParseFloat(req.Qty, 64)
	price, _ := strconv.ParseFloat(req.Price, 64)

	var posSide models.PositionSide
	if req.Side == "Buy" {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	var orderType models.OrderType
	switch req.OrderType {
	case "Limit":
		orderType = models.OrderTypeLimit
	default:
		orderType = models.OrderTypeMarket
	}

	var order *models.Order
	var err error

	if !req.ReduceOnly {
		openReq := &service.OpenPositionRequest{
			AccountID: account.ID,
			Symbol:    req.Symbol,
			Side:      posSide,
			Quantity:  quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.OpenPosition(openReq, models.ExchangeBybit)
	} else {
		closeReq := &service.ClosePositionRequest{
			AccountID: account.ID,
			Symbol:    req.Symbol,
			Side:      posSide,
			Quantity:  &quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.ClosePosition(closeReq, models.ExchangeBybit)
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"orderId":     strconv.Itoa(int(order.ID)),
			"orderLinkId": order.ClientOrderID,
		},
		"time": time.Now().UnixMilli(),
	})
}

// SetTradingStop handles POST /v5/position/trading-stop (SL/TP)
func (h *Handler) SetTradingStop(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	var req struct {
		Category     string `json:"category"`
		Symbol       string `json:"symbol"`
		TakeProfit   string `json:"takeProfit"`
		StopLoss     string `json:"stopLoss"`
		TpTriggerBy  string `json:"tpTriggerBy"`
		SlTriggerBy  string `json:"slTriggerBy"`
		TpslMode     string `json:"tpslMode"`
		TpOrderType  string `json:"tpOrderType"`
		SlOrderType  string `json:"slOrderType"`
		TpSize       string `json:"tpSize"`
		SlSize       string `json:"slSize"`
		TpLimitPrice string `json:"tpLimitPrice"`
		SlLimitPrice string `json:"slLimitPrice"`
		PositionIdx  int    `json:"positionIdx"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 10001, err.Error())
		return
	}

	// Determine position side from positionIdx
	var posSide models.PositionSide
	if req.PositionIdx == 1 {
		posSide = models.PositionSideLong
	} else if req.PositionIdx == 2 {
		posSide = models.PositionSideShort
	} else {
		posSide = models.PositionSideLong // Default
	}

	// Set Stop Loss
	if req.StopLoss != "" {
		sl, _ := strconv.ParseFloat(req.StopLoss, 64)
		if sl > 0 {
			h.tradingService.SetStopLoss(account.ID, req.Symbol, posSide, sl)
		}
	}

	// Set Take Profit
	if req.TakeProfit != "" {
		tp, _ := strconv.ParseFloat(req.TakeProfit, 64)
		if tp > 0 {
			h.tradingService.SetTakeProfit(account.ID, req.Symbol, posSide, tp)
		}
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result":  gin.H{},
		"time":    time.Now().UnixMilli(),
	})
}

// GetOpenOrders handles GET /v5/order/realtime
func (h *Handler) GetOpenOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	symbol := c.Query("symbol")
	orders, _ := h.tradingService.GetOpenOrders(account.ID, symbol)

	list := make([]gin.H, 0)
	for _, order := range orders {
		side := "Buy"
		if order.Side == models.OrderSideSell {
			side = "Sell"
		}

		list = append(list, gin.H{
			"orderId":     strconv.Itoa(int(order.ID)),
			"orderLinkId": order.ClientOrderID,
			"symbol":      order.Symbol,
			"side":        side,
			"orderType":   string(order.Type),
			"price":       strconv.FormatFloat(order.Price, 'f', 8, 64),
			"qty":         strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"cumExecQty":  strconv.FormatFloat(order.FilledQty, 'f', 8, 64),
			"orderStatus": string(order.Status),
			"createdTime": strconv.FormatInt(order.CreatedAt.UnixMilli(), 10),
			"updatedTime": strconv.FormatInt(order.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"category": "linear",
			"list":     list,
		},
		"time": time.Now().UnixMilli(),
	})
}

// CancelOrder handles POST /v5/order/cancel
func (h *Handler) CancelOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	var req struct {
		Category string `json:"category"`
		Symbol   string `json:"symbol"`
		OrderId  string `json:"orderId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 10001, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"orderId":     req.OrderId,
			"orderLinkId": "",
		},
		"time": time.Now().UnixMilli(),
	})
}

// CancelAllOrders handles POST /v5/order/cancel-all
func (h *Handler) CancelAllOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	var req struct {
		Category string `json:"category"`
		Symbol   string `json:"symbol"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 10001, err.Error())
		return
	}

	count, err := h.tradingService.CancelAllOrders(account.ID, req.Symbol)
	if err != nil {
		h.errorResponse(c, 10000, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"list":    []gin.H{},
			"success": strconv.FormatInt(count, 10),
		},
		"time": time.Now().UnixMilli(),
	})
}

// SetLeverage handles POST /v5/position/set-leverage
func (h *Handler) SetLeverage(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, 10003, "Invalid apiKey")
		return
	}

	var req struct {
		Category     string `json:"category"`
		Symbol       string `json:"symbol"`
		BuyLeverage  string `json:"buyLeverage"`
		SellLeverage string `json:"sellLeverage"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 10001, err.Error())
		return
	}

	leverage, _ := strconv.Atoi(req.BuyLeverage)
	if err := h.tradingService.SetLeverage(account.ID, req.Symbol, leverage); err != nil {
		h.errorResponse(c, 10000, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result":  gin.H{},
		"time":    time.Now().UnixMilli(),
	})
}

// GetTickers handles GET /v5/market/tickers
func (h *Handler) GetTickers(c *gin.Context) {
	symbol := c.Query("symbol")

	if symbol != "" {
		price, err := h.priceService.GetPrice("bybit", symbol)
		if err != nil {
			h.errorResponse(c, 10001, "Invalid symbol")
			return
		}

		c.JSON(200, gin.H{
			"retCode": 0,
			"retMsg":  "OK",
			"result": gin.H{
				"category": "linear",
				"list": []gin.H{
					{
						"symbol":          symbol,
						"lastPrice":       strconv.FormatFloat(price, 'f', 8, 64),
						"markPrice":       strconv.FormatFloat(price, 'f', 8, 64),
						"indexPrice":      strconv.FormatFloat(price, 'f', 8, 64),
						"prevPrice24h":    strconv.FormatFloat(price*0.98, 'f', 8, 64),
						"price24hPcnt":    "0.0200",
						"highPrice24h":    strconv.FormatFloat(price*1.02, 'f', 8, 64),
						"lowPrice24h":     strconv.FormatFloat(price*0.98, 'f', 8, 64),
						"volume24h":       "1000000",
						"turnover24h":     "100000000",
						"fundingRate":     "0.0001",
						"nextFundingTime": strconv.FormatInt(time.Now().Add(8*time.Hour).UnixMilli(), 10),
					},
				},
			},
			"time": time.Now().UnixMilli(),
		})
		return
	}

	// Return all prices
	prices := h.priceService.GetAllPrices("bybit")
	list := make([]gin.H, 0)
	for sym, price := range prices {
		list = append(list, gin.H{
			"symbol":    sym,
			"lastPrice": strconv.FormatFloat(price, 'f', 8, 64),
			"markPrice": strconv.FormatFloat(price, 'f', 8, 64),
		})
	}

	c.JSON(200, gin.H{
		"retCode": 0,
		"retMsg":  "OK",
		"result": gin.H{
			"category": "linear",
			"list":     list,
		},
		"time": time.Now().UnixMilli(),
	})
}

// Helper functions

func (h *Handler) errorResponse(c *gin.Context, code int, msg string) {
	c.JSON(200, gin.H{
		"retCode": code,
		"retMsg":  msg,
		"result":  gin.H{},
		"time":    time.Now().UnixMilli(),
	})
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch err {
	case service.ErrInsufficientBalance:
		h.errorResponse(c, 110007, "Insufficient account balance")
	case service.ErrInvalidSymbol:
		h.errorResponse(c, 10001, "Invalid symbol")
	case service.ErrInvalidQuantity:
		h.errorResponse(c, 10001, "Invalid qty")
	case service.ErrNoOpenPosition:
		h.errorResponse(c, 110028, "position not exist")
	default:
		h.errorResponse(c, 10000, err.Error())
	}
}

// RegisterRoutes registers Bybit-compatible routes
func (h *Handler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	v5 := router.Group("/v5")

	// Public endpoints
	market := v5.Group("/market")
	{
		market.GET("/time", h.GetServerTime)
		market.GET("/instruments-info", h.GetInstrumentsInfo)
		market.GET("/tickers", h.GetTickers)
	}

	// Private endpoints
	v5.Use(authMiddleware)
	{
		account := v5.Group("/account")
		{
			account.GET("/wallet-balance", h.GetWalletBalance)
		}

		position := v5.Group("/position")
		{
			position.GET("/list", h.GetPositionInfo)
			position.POST("/set-leverage", middleware.TradingLoggerMiddleware(), h.SetLeverage)
			position.POST("/trading-stop", middleware.TradingLoggerMiddleware(), h.SetTradingStop)
		}

		order := v5.Group("/order")
		{
			order.POST("/create", middleware.TradingLoggerMiddleware(), h.CreateOrder)
			order.POST("/cancel", middleware.TradingLoggerMiddleware(), h.CancelOrder)
			order.POST("/cancel-all", middleware.TradingLoggerMiddleware(), h.CancelAllOrders)
			order.GET("/realtime", h.GetOpenOrders)
		}
	}
}
