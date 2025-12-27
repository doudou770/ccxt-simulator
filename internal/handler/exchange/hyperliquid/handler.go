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
	tradingService      *service.TradingService
	priceService        *service.PriceService
	exchangeInfoService *service.ExchangeInfoService
}

// NewHandler creates a new Hyperliquid handler
func NewHandler(tradingService *service.TradingService, priceService *service.PriceService, exchangeInfoService *service.ExchangeInfoService) *Handler {
	return &Handler{
		tradingService:      tradingService,
		priceService:        priceService,
		exchangeInfoService: exchangeInfoService,
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

// GetOpenOrders handles POST /info (type: openOrders)
func (h *Handler) GetOpenOrders(c *gin.Context, user string) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	orders, _ := h.tradingService.GetOpenOrders(account.ID, "")

	result := make([]gin.H, 0)
	for _, order := range orders {
		result = append(result, gin.H{
			"coin":      convertSymbol(order.Symbol),
			"oid":       order.ID,
			"cloid":     order.ClientOrderID,
			"side":      string(order.Side),
			"limitPx":   strconv.FormatFloat(order.Price, 'f', 8, 64),
			"sz":        strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"origSz":    strconv.FormatFloat(order.Quantity, 'f', 8, 64),
			"timestamp": order.CreatedAt.UnixMilli(),
		})
	}

	c.JSON(200, result)
}

// PlaceOrder handles POST /exchange (action: order)
func (h *Handler) PlaceOrder(c *gin.Context, req map[string]interface{}) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	orders, ok := req["orders"].([]interface{})
	if !ok || len(orders) == 0 {
		c.JSON(400, gin.H{"error": "Invalid orders"})
		return
	}

	// Process first order
	orderMap := orders[0].(map[string]interface{})

	// Get asset and convert to symbol
	a, _ := orderMap["a"].(float64)
	symbol := assetIndexToSymbol(int(a))

	// Get order details
	isBuy, _ := orderMap["b"].(bool)
	priceStr, _ := orderMap["p"].(string)
	sizeStr, _ := orderMap["s"].(string)
	reduceOnly, _ := orderMap["r"].(bool)

	quantity, _ := strconv.ParseFloat(sizeStr, 64)
	price, _ := strconv.ParseFloat(priceStr, 64)

	var posSide models.PositionSide
	if isBuy {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	orderType := models.OrderTypeLimit
	if t, ok := orderMap["t"].(map[string]interface{}); ok {
		if _, isMarket := t["market"]; isMarket {
			orderType = models.OrderTypeMarket
		}
	}

	var order *models.Order
	var err error

	if !reduceOnly {
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

// PlaceTpSl handles POST /exchange (action: order with tpsl)
func (h *Handler) PlaceTpSl(c *gin.Context, req map[string]interface{}) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	orders, ok := req["orders"].([]interface{})
	if !ok || len(orders) == 0 {
		c.JSON(400, gin.H{"error": "Invalid orders"})
		return
	}

	orderMap := orders[0].(map[string]interface{})

	a, _ := orderMap["a"].(float64)
	symbol := assetIndexToSymbol(int(a))

	isBuy, _ := orderMap["b"].(bool)
	sizeStr, _ := orderMap["s"].(string)
	quantity, _ := strconv.ParseFloat(sizeStr, 64)

	var posSide models.PositionSide
	if isBuy {
		posSide = models.PositionSideLong
	} else {
		posSide = models.PositionSideShort
	}

	// Determine TP or SL based on trigger
	var orderType models.OrderType
	var triggerPrice float64

	if t, ok := orderMap["t"].(map[string]interface{}); ok {
		if trigger, ok := t["trigger"].(map[string]interface{}); ok {
			if tp, ok := trigger["triggerPx"].(string); ok {
				triggerPrice, _ = strconv.ParseFloat(tp, 64)
			}
			if isMarket, _ := trigger["isMarket"].(bool); isMarket {
				// Check if it's TP or SL based on direction
				if tpsl, ok := trigger["tpsl"].(string); ok {
					if tpsl == "tp" {
						orderType = models.OrderTypeTakeProfit
					} else {
						orderType = models.OrderTypeStopMarket
					}
				}
			}
		}
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

	order, _, err := h.tradingService.ClosePosition(closeReq, models.ExchangeHyperliquid)
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
func (h *Handler) SetLeverage(c *gin.Context, req map[string]interface{}) {
	account := middleware.GetAccount(c)
	if account == nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	asset, _ := req["asset"].(float64)
	leverage, _ := req["leverage"].(float64)

	symbol := assetIndexToSymbol(int(asset))

	if err := h.tradingService.SetLeverage(account.ID, symbol, int(leverage)); err != nil {
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
		hlSymbol := convertSymbol(symbol)
		mids[hlSymbol] = strconv.FormatFloat(price, 'f', 8, 64)
	}

	c.JSON(200, mids)
}

// GetMeta handles POST /info (type: meta)
func (h *Handler) GetMeta(c *gin.Context) {
	if h.exchangeInfoService != nil {
		data, err := h.exchangeInfoService.GetExchangeInfo("hyperliquid")
		if err == nil && data != nil {
			c.JSON(200, data)
			return
		}
	}

	c.JSON(200, gin.H{
		"universe": []gin.H{
			{"name": "BTC", "szDecimals": 5},
			{"name": "ETH", "szDecimals": 4},
			{"name": "SOL", "szDecimals": 2},
			{"name": "DOGE", "szDecimals": 0},
			{"name": "XRP", "szDecimals": 1},
		},
	})
}

// InfoHandler handles POST /info route
func (h *Handler) InfoHandler(c *gin.Context) {
	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	infoType, _ := req["type"].(string)
	user, _ := req["user"].(string)

	switch infoType {
	case "allMids":
		h.GetAllMids(c)
	case "clearinghouseState":
		h.GetUserState(c)
	case "meta":
		h.GetMeta(c)
	case "openOrders":
		h.GetOpenOrders(c, user)
	default:
		c.JSON(400, gin.H{"error": "Unknown info type"})
	}
}

// ExchangeHandler handles POST /exchange route
func (h *Handler) ExchangeHandler(c *gin.Context) {
	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	action, _ := req["action"].(string)

	switch action {
	case "order":
		// Check if it has trigger (TP/SL order)
		if orders, ok := req["orders"].([]interface{}); ok && len(orders) > 0 {
			if orderMap, ok := orders[0].(map[string]interface{}); ok {
				if t, ok := orderMap["t"].(map[string]interface{}); ok {
					if _, hasTrigger := t["trigger"]; hasTrigger {
						h.PlaceTpSl(c, req)
						return
					}
				}
			}
		}
		h.PlaceOrder(c, req)
	case "cancel":
		h.CancelOrder(c)
	case "updateLeverage":
		h.SetLeverage(c, req)
	default:
		c.JSON(400, gin.H{"error": "Unknown action"})
	}
}

// Helper functions

func convertSymbol(symbol string) string {
	// BTCUSDT -> BTC
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	if len(symbol) > 3 && symbol[len(symbol)-3:] == "USD" {
		return symbol[:len(symbol)-3]
	}
	return symbol
}

func assetIndexToSymbol(index int) string {
	// HL uses asset indices, convert to symbol
	assets := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "DOGEUSDT", "XRPUSDT"}
	if index >= 0 && index < len(assets) {
		return assets[index]
	}
	return "BTCUSDT"
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
