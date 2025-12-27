package handler

import (
	"errors"
	"strconv"

	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/repository"
	"github.com/ccxt-simulator/internal/service"
	"github.com/ccxt-simulator/pkg/response"
	"github.com/gin-gonic/gin"
)

// AccountHandler handles account API requests
type AccountHandler struct {
	accountService *service.AccountService
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(accountService *service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

// CreateAccount handles account creation
// POST /api/v1/accounts
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req service.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	account, err := h.accountService.CreateAccount(userID, &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, account)
}

// GetAccounts handles getting all accounts for the authenticated user
// GET /api/v1/accounts
func (h *AccountHandler) GetAccounts(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Parse pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	accounts, total, err := h.accountService.GetAccountsPaginated(userID, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessPaginated(c, accounts, total, page, pageSize)
}

// GetAccount handles getting a single account
// GET /api/v1/accounts/:id
func (h *AccountHandler) GetAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	account, err := h.accountService.GetAccountByID(userID, uint(accountID))
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			response.NotFound(c, "account not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, account)
}

// UpdateAccount handles updating an account
// PUT /api/v1/accounts/:id
func (h *AccountHandler) UpdateAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	var req service.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	account, err := h.accountService.UpdateAccount(userID, uint(accountID), &req)
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			response.NotFound(c, "account not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, account)
}

// DeleteAccount handles deleting an account
// DELETE /api/v1/accounts/:id
func (h *AccountHandler) DeleteAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	err = h.accountService.DeleteAccount(userID, uint(accountID))
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			response.NotFound(c, "account not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "account deleted"})
}

// ResetAPIKey handles resetting the API key for an account
// POST /api/v1/accounts/:id/reset-key
func (h *AccountHandler) ResetAPIKey(c *gin.Context) {
	userID := middleware.GetUserID(c)

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	account, err := h.accountService.ResetAPIKey(userID, uint(accountID))
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			response.NotFound(c, "account not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, account)
}

// AddBalance handles adding balance to an account
// POST /api/v1/accounts/:id/add-balance
func (h *AccountHandler) AddBalance(c *gin.Context) {
	userID := middleware.GetUserID(c)

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	account, err := h.accountService.AddBalance(userID, uint(accountID), req.Amount)
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			response.NotFound(c, "account not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, account)
}

// RegisterRoutes registers account routes
func (h *AccountHandler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	accounts := rg.Group("/accounts")
	accounts.Use(authMiddleware)
	{
		accounts.POST("", h.CreateAccount)
		accounts.GET("", h.GetAccounts)
		accounts.GET("/:id", h.GetAccount)
		accounts.PUT("/:id", h.UpdateAccount)
		accounts.DELETE("/:id", h.DeleteAccount)
		accounts.POST("/:id/reset-key", h.ResetAPIKey)
		accounts.POST("/:id/add-balance", h.AddBalance)
	}
}
