package okx

import (
	"strconv"
	"strings"
	"time"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler handles OKX-compatible API requests
type Handler struct {
	tradingService      *service.TradingService
	priceService        *service.PriceService
	exchangeInfoService *service.ExchangeInfoService
}

// NewHandler creates a new OKX handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService, exchangeInfoService *service.ExchangeInfoService) *Handler {
	return &Handler{
		tradingService:      tradingService,
		priceService:        priceService,
		exchangeInfoService: exchangeInfoService,
	}
}

// GetTime handles GET /api/v5/public/time
func (h *Handler) GetTime(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"ts": strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
		},
	})
}

// GetInstruments handles GET /api/v5/public/instruments
func (h *Handler) GetInstruments(c *gin.Context) {
	if h.exchangeInfoService != nil {
		data, err := h.exchangeInfoService.GetExchangeInfo("okx")
		if err == nil && data != nil {
			c.JSON(200, data)
			return
		}
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{},
	})
}

// GetBalance handles GET /api/v5/account/balance
func (h *Handler) GetBalance(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	balance, err := h.tradingService.GetBalance(account.ID, models.ExchangeOKX)
	if err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"totalEq":     strconv.FormatFloat(balance["equity"], 'f', 8, 64),
				"isoEq":       "0",
				"adjEq":       strconv.FormatFloat(balance["equity"], 'f', 8, 64),
				"ordFroz":     "0",
				"imr":         strconv.FormatFloat(balance["margin"], 'f', 8, 64),
				"mmr":         "0",
				"notionalUsd": strconv.FormatFloat(balance["margin"]*10, 'f', 8, 64),
				"mgnRatio":    "999",
				"details": []gin.H{
					{
						"ccy":       "USDT",
						"eq":        strconv.FormatFloat(balance["equity"], 'f', 8, 64),
						"cashBal":   strconv.FormatFloat(balance["balance"], 'f', 8, 64),
						"availBal":  strconv.FormatFloat(balance["available"], 'f', 8, 64),
						"frozenBal": strconv.FormatFloat(balance["margin"], 'f', 8, 64),
						"upl":       strconv.FormatFloat(balance["unrealized_pnl"], 'f', 8, 64),
						"uplLiab":   "0",
					},
				},
				"uTime": strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
		},
	})
}

// GetPositions handles GET /api/v5/account/positions
func (h *Handler) GetPositions(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	instId := c.Query("instId")
	positions, err := h.tradingService.GetPositions(account.ID, models.ExchangeOKX)
	if err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	data := make([]gin.H, 0)
	for _, pos := range positions {
		okxInstId := convertToOKXSymbol(pos.Symbol)
		if instId != "" && okxInstId != instId {
			continue
		}

		posSide := "long"
		if pos.Side == models.PositionSideShort {
			posSide = "short"
		}

		data = append(data, gin.H{
			"instId":   okxInstId,
			"instType": "SWAP",
			"mgnMode":  string(pos.MarginMode),
			"posId":    strconv.Itoa(int(pos.ID)),
			"posSide":  posSide,
			"pos":      strconv.FormatFloat(pos.Quantity, 'f', 8, 64),
			"avgPx":    strconv.FormatFloat(pos.EntryPrice, 'f', 8, 64),
			"markPx":   strconv.FormatFloat(pos.MarkPrice, 'f', 8, 64),
			"upl":      strconv.FormatFloat(pos.UnrealizedPnL, 'f', 8, 64),
			"uplRatio": strconv.FormatFloat(pos.UnrealizedPnL/pos.Margin, 'f', 8, 64),
			"lever":    strconv.Itoa(pos.Leverage),
			"liqPx":    strconv.FormatFloat(pos.LiquidationPrice, 'f', 8, 64),
			"margin":   strconv.FormatFloat(pos.Margin, 'f', 8, 64),
			"cTime":    strconv.FormatInt(pos.CreatedAt.UnixMilli(), 10),
			"uTime":    strconv.FormatInt(pos.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": data,
	})
}

// CreateOrder handles POST /api/v5/trade/order
func (h *Handler) CreateOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	var req struct {
		InstId     string `json:"instId"`
		TdMode     string `json:"tdMode"`
		Side       string `json:"side"`
		PosSide    string `json:"posSide"`
		OrdType    string `json:"ordType"`
		Sz         string `json:"sz"`
		Px         string `json:"px"`
		ReduceOnly string `json:"reduceOnly"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	symbol := convertFromOKXSymbol(req.InstId)
	quantity, _ := strconv.ParseFloat(req.Sz, 64)
	price, _ := strconv.ParseFloat(req.Px, 64)

	var posSide models.PositionSide
	if req.PosSide == "long" {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	var orderType models.OrderType
	switch req.OrdType {
	case "limit":
		orderType = models.OrderTypeLimit
	default:
		orderType = models.OrderTypeMarket
	}

	isReduceOnly := req.ReduceOnly == "true"
	var order *models.Order
	var err error

	if !isReduceOnly {
		openReq := &service.OpenPositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.OpenPosition(openReq, models.ExchangeOKX)
	} else {
		closeReq := &service.ClosePositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  &quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.ClosePosition(closeReq, models.ExchangeOKX)
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"ordId":   strconv.Itoa(int(order.ID)),
				"clOrdId": order.ClientOrderID,
				"tag":     "",
				"sCode":   "0",
				"sMsg":    "",
			},
		},
	})
}

// CreateAlgoOrder handles POST /api/v5/trade/order-algo (SL/TP orders)
func (h *Handler) CreateAlgoOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	var req struct {
		InstId      string `json:"instId"`
		TdMode      string `json:"tdMode"`
		Side        string `json:"side"`
		PosSide     string `json:"posSide"`
		OrdType     string `json:"ordType"` // conditional
		Sz          string `json:"sz"`
		TpTriggerPx string `json:"tpTriggerPx"`
		TpOrdPx     string `json:"tpOrdPx"`
		SlTriggerPx string `json:"slTriggerPx"`
		SlOrdPx     string `json:"slOrdPx"`
		ReduceOnly  string `json:"reduceOnly"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	symbol := convertFromOKXSymbol(req.InstId)
	quantity, _ := strconv.ParseFloat(req.Sz, 64)

	var posSide models.PositionSide
	if req.PosSide == "long" {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	var orderType models.OrderType
	var triggerPrice float64

	if req.SlTriggerPx != "" {
		orderType = models.OrderTypeStopMarket
		triggerPrice, _ = strconv.ParseFloat(req.SlTriggerPx, 64)
	} else if req.TpTriggerPx != "" {
		orderType = models.OrderTypeTakeProfit
		triggerPrice, _ = strconv.ParseFloat(req.TpTriggerPx, 64)
	}

	closeReq := &service.ClosePositionRequest{
		AccountID:  account.ID,
		Symbol:     symbol,
		Side:       posSide,
		Quantity:   &quantity,
		OrderType:  orderType,
		StopPrice:  triggerPrice,
		ReduceOnly: true,
	}

	order, _, err := h.tradingService.ClosePosition(closeReq, models.ExchangeOKX)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"algoId":      strconv.Itoa(int(order.ID)),
				"algoClOrdId": order.ClientOrderID,
				"sCode":       "0",
				"sMsg":        "",
			},
		},
	})
}

// GetOpenAlgoOrders handles GET /api/v5/trade/orders-algo-pending
func (h *Handler) GetOpenAlgoOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	instId := c.Query("instId")
	symbol := ""
	if instId != "" {
		symbol = convertFromOKXSymbol(instId)
	}

	orders, _ := h.tradingService.GetOpenAlgoOrders(account.ID, symbol)

	data := make([]gin.H, 0)
	for _, order := range orders {
		data = append(data, gin.H{
			"algoId":      strconv.Itoa(int(order.ID)),
			"algoClOrdId": order.ClientOrderID,
			"instId":      convertToOKXSymbol(order.Symbol),
			"instType":    "SWAP",
			"ordType":     "conditional",
			"sz":          strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"triggerPx":   strconv.FormatFloat(order.StopPrice, 'f', 8, 64),
			"state":       "live",
			"cTime":       strconv.FormatInt(order.CreatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": data,
	})
}

// CancelAlgoOrder handles POST /api/v5/trade/cancel-algos
func (h *Handler) CancelAlgoOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	var req []struct {
		AlgoId string `json:"algoId"`
		InstId string `json:"instId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	data := make([]gin.H, 0)
	for _, r := range req {
		data = append(data, gin.H{
			"algoId": r.AlgoId,
			"sCode":  "0",
			"sMsg":   "",
		})
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": data,
	})
}

// CancelOrder handles POST /api/v5/trade/cancel-order
func (h *Handler) CancelOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	var req struct {
		InstId string `json:"instId"`
		OrdId  string `json:"ordId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"ordId":   req.OrdId,
				"clOrdId": "",
				"sCode":   "0",
				"sMsg":    "",
			},
		},
	})
}

// CancelBatchOrders handles POST /api/v5/trade/cancel-batch-orders
func (h *Handler) CancelBatchOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	var req []struct {
		InstId string `json:"instId"`
		OrdId  string `json:"ordId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	data := make([]gin.H, 0)
	for _, r := range req {
		data = append(data, gin.H{
			"ordId":   r.OrdId,
			"clOrdId": "",
			"sCode":   "0",
			"sMsg":    "",
		})
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": data,
	})
}

// GetOpenOrders handles GET /api/v5/trade/orders-pending
func (h *Handler) GetOpenOrders(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	instId := c.Query("instId")
	symbol := ""
	if instId != "" {
		symbol = convertFromOKXSymbol(instId)
	}

	orders, _ := h.tradingService.GetOpenOrders(account.ID, symbol)

	data := make([]gin.H, 0)
	for _, order := range orders {
		posSide := "long"
		if order.PositionSide == models.PositionSideShort {
			posSide = "short"
		}

		data = append(data, gin.H{
			"instId":  convertToOKXSymbol(order.Symbol),
			"ordId":   strconv.Itoa(int(order.ID)),
			"clOrdId": order.ClientOrderID,
			"px":      strconv.FormatFloat(order.Price, 'f', 8, 64),
			"sz":      strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"fillSz":  strconv.FormatFloat(order.FilledQty, 'f', 8, 64),
			"side":    strings.ToLower(string(order.Side)),
			"posSide": posSide,
			"ordType": strings.ToLower(string(order.Type)),
			"state":   "live",
			"cTime":   strconv.FormatInt(order.CreatedAt.UnixMilli(), 10),
			"uTime":   strconv.FormatInt(order.UpdatedAt.UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": data,
	})
}

// SetLeverage handles POST /api/v5/account/set-leverage
func (h *Handler) SetLeverage(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		h.errorResponse(c, "50111", "API key is invalid")
		return
	}

	var req struct {
		InstId  string `json:"instId"`
		Lever   string `json:"lever"`
		MgnMode string `json:"mgnMode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	symbol := convertFromOKXSymbol(req.InstId)
	leverage, _ := strconv.Atoi(req.Lever)

	if err := h.tradingService.SetLeverage(account.ID, symbol, leverage); err != nil {
		h.errorResponse(c, "50000", err.Error())
		return
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"instId":  req.InstId,
				"lever":   req.Lever,
				"mgnMode": req.MgnMode,
			},
		},
	})
}

// GetMarkPrice handles GET /api/v5/public/mark-price
func (h *Handler) GetMarkPrice(c *gin.Context) {
	instId := c.Query("instId")
	symbol := convertFromOKXSymbol(instId)

	price, err := h.priceService.GetPrice("okx", symbol)
	if err != nil {
		h.errorResponse(c, "51001", "Invalid instId")
		return
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": []gin.H{
			{
				"instId":   instId,
				"instType": "SWAP",
				"markPx":   strconv.FormatFloat(price, 'f', 8, 64),
				"ts":       strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
		},
	})
}

// GetTickers handles GET /api/v5/market/tickers
func (h *Handler) GetTickers(c *gin.Context) {
	instType := c.Query("instType")
	_ = instType

	prices := h.priceService.GetAllPrices("okx")
	data := make([]gin.H, 0)

	for sym, price := range prices {
		data = append(data, gin.H{
			"instId":   convertToOKXSymbol(sym),
			"instType": "SWAP",
			"last":     strconv.FormatFloat(price, 'f', 8, 64),
			"askPx":    strconv.FormatFloat(price*1.0001, 'f', 8, 64),
			"bidPx":    strconv.FormatFloat(price*0.9999, 'f', 8, 64),
			"ts":       strconv.FormatInt(time.Now().UnixMilli(), 10),
		})
	}

	c.JSON(200, gin.H{
		"code": "0",
		"msg":  "",
		"data": data,
	})
}

// Helper functions

func convertToOKXSymbol(symbol string) string {
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		base := symbol[:len(symbol)-4]
		return base + "-USDT-SWAP"
	}
	return symbol
}

func convertFromOKXSymbol(instId string) string {
	parts := make([]byte, 0)
	for _, c := range instId {
		if c != '-' {
			parts = append(parts, byte(c))
		}
	}
	result := string(parts)
	if len(result) > 4 && result[len(result)-4:] == "SWAP" {
		return result[:len(result)-4]
	}
	return result
}

func (h *Handler) errorResponse(c *gin.Context, code, msg string) {
	c.JSON(200, gin.H{
		"code": code,
		"msg":  msg,
		"data": []gin.H{},
	})
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch err {
	case service.ErrInsufficientBalance:
		h.errorResponse(c, "51008", "Order placement failed due to insufficient balance")
	case service.ErrInvalidSymbol:
		h.errorResponse(c, "51001", "Instrument ID does not exist")
	case service.ErrInvalidQuantity:
		h.errorResponse(c, "51001", "Order quantity must be greater than 0")
	case service.ErrNoOpenPosition:
		h.errorResponse(c, "51010", "No positions to close")
	default:
		h.errorResponse(c, "50000", err.Error())
	}
}

// RegisterRoutes registers OKX-compatible routes
func (h *Handler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	api := router.Group("/api/v5")

	// Public endpoints (no auth)
	publicApi := api.Group("/public")
	{
		publicApi.GET("/time", h.GetTime)
		publicApi.GET("/instruments", h.GetInstruments)
		publicApi.GET("/mark-price", h.GetMarkPrice)
	}

	// Market endpoints (no auth)
	marketApi := api.Group("/market")
	{
		marketApi.GET("/tickers", h.GetTickers)
	}

	// Private endpoints (require auth)
	api.Use(authMiddleware)
	{
		account := api.Group("/account")
		{
			account.GET("/balance", h.GetBalance)
			account.GET("/positions", h.GetPositions)
			account.POST("/set-leverage", middleware.TradingLoggerMiddleware(), h.SetLeverage)
		}

		trade := api.Group("/trade")
		{
			trade.POST("/order", middleware.TradingLoggerMiddleware(), h.CreateOrder)
			trade.POST("/cancel-order", middleware.TradingLoggerMiddleware(), h.CancelOrder)
			trade.POST("/cancel-batch-orders", middleware.TradingLoggerMiddleware(), h.CancelBatchOrders)
			trade.GET("/orders-pending", h.GetOpenOrders)
			// Algo orders (SL/TP)
			trade.POST("/order-algo", middleware.TradingLoggerMiddleware(), h.CreateAlgoOrder)
			trade.POST("/cancel-algos", middleware.TradingLoggerMiddleware(), h.CancelAlgoOrder)
			trade.GET("/orders-algo-pending", h.GetOpenAlgoOrders)
		}
	}
}
