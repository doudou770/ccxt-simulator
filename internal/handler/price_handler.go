package handler

import (
	"github.com/ccxt-simulator/internal/service"
	"github.com/ccxt-simulator/pkg/response"
	"github.com/gin-gonic/gin"
)

// PriceHandler handles price-related API requests
type PriceHandler struct {
	priceService *service.PriceService
}

// NewPriceHandler creates a new PriceHandler
func NewPriceHandler(priceService *service.PriceService) *PriceHandler {
	return &PriceHandler{
		priceService: priceService,
	}
}

// GetPrice returns the current price for a symbol
// GET /api/v1/prices/:exchange/:symbol
func (h *PriceHandler) GetPrice(c *gin.Context) {
	exchangeName := c.Param("exchange")
	symbol := c.Param("symbol")

	price, err := h.priceService.GetPrice(exchangeName, symbol)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"exchange": exchangeName,
		"symbol":   symbol,
		"price":    price,
	})
}

// GetPrices returns all current prices for an exchange
// GET /api/v1/prices/:exchange
func (h *PriceHandler) GetPrices(c *gin.Context) {
	exchangeName := c.Param("exchange")

	prices := h.priceService.GetAllPrices(exchangeName)
	if len(prices) == 0 {
		response.NotFound(c, "no prices found for exchange")
		return
	}

	response.Success(c, gin.H{
		"exchange": exchangeName,
		"prices":   prices,
	})
}

// GetSymbolInfo returns trading pair information
// GET /api/v1/symbols/:exchange/:symbol
func (h *PriceHandler) GetSymbolInfo(c *gin.Context) {
	exchangeName := c.Param("exchange")
	symbol := c.Param("symbol")

	info, err := h.priceService.GetSymbolInfo(exchangeName, symbol)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, info)
}

// GetExchangeStatus returns connection status for all exchanges
// GET /api/v1/exchanges/status
func (h *PriceHandler) GetExchangeStatus(c *gin.Context) {
	status := h.priceService.GetExchangeStatus()
	response.Success(c, status)
}

// RegisterRoutes registers price routes
func (h *PriceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	prices := rg.Group("/prices")
	{
		prices.GET("/:exchange", h.GetPrices)
		prices.GET("/:exchange/:symbol", h.GetPrice)
	}

	symbols := rg.Group("/symbols")
	{
		symbols.GET("/:exchange/:symbol", h.GetSymbolInfo)
	}

	exchanges := rg.Group("/exchanges")
	{
		exchanges.GET("/status", h.GetExchangeStatus)
	}
}
