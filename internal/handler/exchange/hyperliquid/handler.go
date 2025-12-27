package hyperliquid

import (
	"strconv"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler handles Hyperliquid-compatible API requests
type Handler struct {
	tradingService *service.TradingService
	priceService   *service.PriceService
}

// NewHandler creates a new Hyperliquid handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService) *Handler {
	return &Handler{
		tradingService: tradingService,
		priceService:   priceService,
	}
}

// GetUserState handles POST /info (type: clearinghouseState)
func (h *Handler) GetUserState(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	balance, err := h.tradingService.GetBalance(account.ID, models.ExchangeHyperliquid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	positions, err := h.tradingService.GetPositions(account.ID, models.ExchangeHyperliquid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	assetPositions := make([]gin.H, 0)
	for _, pos := range positions {
		szi := pos.Quantity
		if pos.Side == models.PositionSideShort {
			szi = -szi
		}

		assetPositions = append(assetPositions, gin.H{
			"type": "oneWay",
			"position": gin.H{
				"coin":          convertSymbol(pos.Symbol),
				"szi":           strconv.FormatFloat(szi, 'f', 8, 64),
				"entryPx":       strconv.FormatFloat(pos.EntryPrice, 'f', 8, 64),
				"positionValue": strconv.FormatFloat(pos.MarkPrice*pos.Quantity, 'f', 8, 64),
				"unrealizedPnl": strconv.FormatFloat(pos.UnrealizedPnL, 'f', 8, 64),
				"leverage": gin.H{
					"type":  "cross",
					"value": pos.Leverage,
				},
				"liquidationPx": strconv.FormatFloat(pos.LiquidationPrice, 'f', 8, 64),
				"marginUsed":    strconv.FormatFloat(pos.Margin, 'f', 8, 64),
			},
		})
	}

	c.JSON(200, gin.H{
		"marginSummary": gin.H{
			"accountValue":    strconv.FormatFloat(balance["equity"], 'f', 8, 64),
			"totalNtlPos":     strconv.FormatFloat(balance["margin"]*10, 'f', 8, 64),
			"totalRawUsd":     strconv.FormatFloat(balance["balance"], 'f', 8, 64),
			"totalMarginUsed": strconv.FormatFloat(balance["margin"], 'f', 8, 64),
		},
		"crossMarginSummary": gin.H{
			"accountValue":    strconv.FormatFloat(balance["equity"], 'f', 8, 64),
			"totalNtlPos":     strconv.FormatFloat(balance["margin"]*10, 'f', 8, 64),
			"totalRawUsd":     strconv.FormatFloat(balance["balance"], 'f', 8, 64),
			"totalMarginUsed": strconv.FormatFloat(balance["margin"], 'f', 8, 64),
		},
		"withdrawable":   strconv.FormatFloat(balance["available"], 'f', 8, 64),
		"assetPositions": assetPositions,
	})
}

// PlaceOrder handles POST /exchange (action: order)
func (h *Handler) PlaceOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Action string `json:"action"`
		Orders []struct {
			A int    `json:"a"` // asset index
			B bool   `json:"b"` // is buy
			P string `json:"p"` // price
			S string `json:"s"` // size
			R bool   `json:"r"` // reduce only
			T gin.H  `json:"t"` // order type
		} `json:"orders"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Action != "order" || len(req.Orders) == 0 {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// Process first order
	orderReq := req.Orders[0]
	symbol := "BTCUSDT" // Would need asset index mapping
	quantity, _ := strconv.ParseFloat(orderReq.S, 64)
	price, _ := strconv.ParseFloat(orderReq.P, 64)

	var posSide models.PositionSide
	if orderReq.B {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	orderType := models.OrderTypeLimit
	if orderReq.T != nil {
		if _, ok := orderReq.T["market"]; ok {
			orderType = models.OrderTypeMarket
		}
	}

	var order *models.Order
	var err error

	if !orderReq.R {
		openReq := &service.OpenPositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.OpenPosition(openReq, models.ExchangeHyperliquid)
	} else {
		closeReq := &service.ClosePositionRequest{
			AccountID: account.ID,
			Symbol:    symbol,
			Side:      posSide,
			Quantity:  &quantity,
			OrderType: orderType,
			Price:     price,
		}
		order, _, err = h.tradingService.ClosePosition(closeReq, models.ExchangeHyperliquid)
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"response": gin.H{
			"type": "order",
			"data": gin.H{
				"statuses": []gin.H{
					{
						"resting": gin.H{
							"oid": order.ID,
						},
					},
				},
			},
		},
	})
}

// CancelOrder handles POST /exchange (action: cancel)
func (h *Handler) CancelOrder(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"response": gin.H{
			"type": "cancel",
			"data": gin.H{
				"statuses": []string{"success"},
			},
		},
	})
}

// SetLeverage handles POST /exchange (action: updateLeverage)
func (h *Handler) SetLeverage(c *gin.Context) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Action   string `json:"action"`
		Asset    int    `json:"asset"`
		IsCross  bool   `json:"isCross"`
		Leverage int    `json:"leverage"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	symbol := "BTCUSDT" // Would need asset index mapping
	if err := h.tradingService.SetLeverage(account.ID, symbol, req.Leverage); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"response": gin.H{
			"type": "updateLeverage",
		},
	})
}

// GetAllMids handles POST /info (type: allMids)
func (h *Handler) GetAllMids(c *gin.Context) {
	prices := h.priceService.GetAllPrices("hyperliquid")

	mids := make(map[string]string)
	for symbol, price := range prices {
		// Convert to HL format (remove USDT suffix)
		hlSymbol := convertSymbol(symbol)
		mids[hlSymbol] = strconv.FormatFloat(price, 'f', 8, 64)
	}

	c.JSON(200, mids)
}

// InfoHandler handles POST /info route
func (h *Handler) InfoHandler(c *gin.Context) {
	var req struct {
		Type string `json:"type"`
		User string `json:"user"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	switch req.Type {
	case "allMids":
		h.GetAllMids(c)
	case "clearinghouseState":
		h.GetUserState(c)
	case "meta":
		h.GetMeta(c)
	default:
		c.JSON(400, gin.H{"error": "Unknown info type"})
	}
}

// GetMeta handles POST /info (type: meta)
func (h *Handler) GetMeta(c *gin.Context) {
	c.JSON(200, gin.H{
		"universe": []gin.H{
			{"name": "BTC", "szDecimals": 5},
			{"name": "ETH", "szDecimals": 4},
			{"name": "SOL", "szDecimals": 2},
		},
	})
}

// ExchangeHandler handles POST /exchange route
func (h *Handler) ExchangeHandler(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}

	// Peek at the action without consuming body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	switch req.Action {
	case "order":
		h.PlaceOrder(c)
	case "cancel":
		h.CancelOrder(c)
	case "updateLeverage":
		h.SetLeverage(c)
	default:
		c.JSON(400, gin.H{"error": "Unknown action"})
	}
}

// Helper functions

func convertSymbol(symbol string) string {
	// BTCUSDT -> BTC
	if len(symbol) > 4 && (symbol[len(symbol)-4:] == "USDT" || symbol[len(symbol)-3:] == "USD") {
		if symbol[len(symbol)-4:] == "USDT" {
			return symbol[:len(symbol)-4]
		}
		return symbol[:len(symbol)-3]
	}
	return symbol
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch err {
	case service.ErrInsufficientBalance:
		c.JSON(400, gin.H{"error": "Insufficient margin"})
	case service.ErrInvalidSymbol:
		c.JSON(400, gin.H{"error": "Invalid asset"})
	case service.ErrInvalidQuantity:
		c.JSON(400, gin.H{"error": "Invalid size"})
	case service.ErrNoOpenPosition:
		c.JSON(400, gin.H{"error": "No position to reduce"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}

// RegisterRoutes registers Hyperliquid-compatible routes
func (h *Handler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	// Public info endpoint
	router.POST("/info", h.InfoHandler)

	// Private exchange endpoint
	exchange := router.Group("/exchange")
	exchange.Use(authMiddleware)
	{
		exchange.POST("", h.ExchangeHandler)
	}
}
