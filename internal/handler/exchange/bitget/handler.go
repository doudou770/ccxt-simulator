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
	tradingService *service.TradingService
	priceService   *service.PriceService
}

// NewHandler creates a new Bitget handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService) *Handler {
	return &Handler{
		tradingService: tradingService,
		priceService:   priceService,
	}
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

	// Map position side
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

	// Map order type
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
	api := router.Group("/api/v2/mix")

	// Public endpoints
	market := api.Group("/market")
	{
		market.GET("/ticker", h.GetTicker)
	}

	// Private endpoints
	api.Use(authMiddleware)
	{
		account := api.Group("/account")
		{
			account.GET("/account", h.GetAccount)
			account.POST("/set-leverage", h.SetLeverage)
		}

		position := api.Group("/position")
		{
			position.GET("/all-position", h.GetPositions)
		}

		order := api.Group("/order")
		{
			order.POST("/place-order", h.PlaceOrder)
			order.POST("/cancel-order", h.CancelOrder)
		}
	}
}
