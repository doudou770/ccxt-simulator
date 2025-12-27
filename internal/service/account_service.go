package service

import (
	"fmt"

	"github.com/ccxt-simulator/internal/config"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/repository"
	"github.com/ccxt-simulator/pkg/crypto"
	"github.com/ccxt-simulator/pkg/keygen"
)

// AccountService handles account operations
type AccountService struct {
	accountRepo      *repository.AccountRepository
	encryptionConfig config.EncryptionConfig
	baseURL          string
}

// NewAccountService creates a new AccountService
func NewAccountService(
	accountRepo *repository.AccountRepository,
	encryptionConfig config.EncryptionConfig,
	baseURL string,
) *AccountService {
	return &AccountService{
		accountRepo:      accountRepo,
		encryptionConfig: encryptionConfig,
		baseURL:          baseURL,
	}
}

// CreateAccountRequest represents the create account request
type CreateAccountRequest struct {
	ExchangeType    models.ExchangeType `json:"exchange_type" binding:"required,oneof=binance okx bybit bitget hyperliquid"`
	InitialBalance  float64             `json:"initial_balance" binding:"required,gt=0"`
	MarginMode      models.MarginMode   `json:"margin_mode" binding:"omitempty,oneof=cross isolated"`
	HedgeMode       bool                `json:"hedge_mode"`
	DefaultLeverage int                 `json:"default_leverage" binding:"omitempty,min=1,max=125"`
}

// CreateAccount creates a new simulated exchange account
func (s *AccountService) CreateAccount(userID uint, req *CreateAccountRequest) (*models.AccountResponse, error) {
	// Set defaults
	if req.MarginMode == "" {
		req.MarginMode = models.MarginModeCross
	}
	if req.DefaultLeverage == 0 {
		req.DefaultLeverage = 20
	}

	// Generate API keys
	keys, err := keygen.GenerateAPIKey(string(req.ExchangeType))
	if err != nil {
		return nil, fmt.Errorf("failed to generate API keys: %w", err)
	}

	// Encrypt API secret
	encryptedSecret, err := crypto.EncryptAES(keys.APISecret, s.encryptionConfig.AESKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API secret: %w", err)
	}

	// Encrypt passphrase if present (OKX)
	var encryptedPassphrase string
	if keys.Passphrase != "" {
		encryptedPassphrase, err = crypto.EncryptAES(keys.Passphrase, s.encryptionConfig.AESKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt passphrase: %w", err)
		}
	}

	// Create account
	account := &models.Account{
		UserID:              userID,
		ExchangeType:        req.ExchangeType,
		APIKey:              keys.APIKey,
		APISecretEncrypted:  encryptedSecret,
		PassphraseEncrypted: encryptedPassphrase,
		BalanceUSDT:         req.InitialBalance,
		InitialBalance:      req.InitialBalance,
		MarginMode:          req.MarginMode,
		HedgeMode:           req.HedgeMode,
		DefaultLeverage:     req.DefaultLeverage,
		MakerFeeRate:        0.0002,
		TakerFeeRate:        0.0004,
	}

	if err := s.accountRepo.Create(account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Build response
	return s.buildAccountResponse(account, keys.APISecret, keys.Passphrase), nil
}

// GetAccounts retrieves all accounts for a user
func (s *AccountService) GetAccounts(userID uint) ([]models.AccountResponse, error) {
	accounts, err := s.accountRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	responses := make([]models.AccountResponse, len(accounts))
	for i, account := range accounts {
		responses[i] = *s.buildAccountResponse(&account, "", "")
	}

	return responses, nil
}

// GetAccountsPaginated retrieves accounts with pagination
func (s *AccountService) GetAccountsPaginated(userID uint, page, pageSize int) ([]models.AccountResponse, int64, error) {
	accounts, total, err := s.accountRepo.GetByUserIDPaginated(userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]models.AccountResponse, len(accounts))
	for i, account := range accounts {
		responses[i] = *s.buildAccountResponse(&account, "", "")
	}

	return responses, total, nil
}

// GetAccountByID retrieves an account by ID
func (s *AccountService) GetAccountByID(userID, accountID uint) (*models.AccountResponse, error) {
	account, err := s.accountRepo.GetByIDAndUserID(accountID, userID)
	if err != nil {
		return nil, err
	}

	return s.buildAccountResponse(account, "", ""), nil
}

// GetAccountByAPIKey retrieves an account by API key
func (s *AccountService) GetAccountByAPIKey(apiKey string) (*models.Account, error) {
	return s.accountRepo.GetByAPIKey(apiKey)
}

// UpdateAccountRequest represents the update account request
type UpdateAccountRequest struct {
	MarginMode      *models.MarginMode `json:"margin_mode" binding:"omitempty,oneof=cross isolated"`
	HedgeMode       *bool              `json:"hedge_mode"`
	DefaultLeverage *int               `json:"default_leverage" binding:"omitempty,min=1,max=125"`
}

// UpdateAccount updates an account
func (s *AccountService) UpdateAccount(userID, accountID uint, req *UpdateAccountRequest) (*models.AccountResponse, error) {
	account, err := s.accountRepo.GetByIDAndUserID(accountID, userID)
	if err != nil {
		return nil, err
	}

	if req.MarginMode != nil {
		account.MarginMode = *req.MarginMode
	}
	if req.HedgeMode != nil {
		account.HedgeMode = *req.HedgeMode
	}
	if req.DefaultLeverage != nil {
		account.DefaultLeverage = *req.DefaultLeverage
	}

	if err := s.accountRepo.Update(account); err != nil {
		return nil, err
	}

	return s.buildAccountResponse(account, "", ""), nil
}

// DeleteAccount deletes an account
func (s *AccountService) DeleteAccount(userID, accountID uint) error {
	// Verify ownership
	_, err := s.accountRepo.GetByIDAndUserID(accountID, userID)
	if err != nil {
		return err
	}

	return s.accountRepo.Delete(accountID)
}

// ResetAPIKey resets the API key and secret for an account
func (s *AccountService) ResetAPIKey(userID, accountID uint) (*models.AccountResponse, error) {
	account, err := s.accountRepo.GetByIDAndUserID(accountID, userID)
	if err != nil {
		return nil, err
	}

	// Generate new API keys
	keys, err := keygen.GenerateAPIKey(string(account.ExchangeType))
	if err != nil {
		return nil, fmt.Errorf("failed to generate API keys: %w", err)
	}

	// Encrypt new API secret
	encryptedSecret, err := crypto.EncryptAES(keys.APISecret, s.encryptionConfig.AESKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API secret: %w", err)
	}

	// Encrypt passphrase if present (OKX)
	var encryptedPassphrase string
	if keys.Passphrase != "" {
		encryptedPassphrase, err = crypto.EncryptAES(keys.Passphrase, s.encryptionConfig.AESKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt passphrase: %w", err)
		}
	}

	// Update account
	account.APIKey = keys.APIKey
	account.APISecretEncrypted = encryptedSecret
	account.PassphraseEncrypted = encryptedPassphrase

	if err := s.accountRepo.Update(account); err != nil {
		return nil, err
	}

	return s.buildAccountResponse(account, keys.APISecret, keys.Passphrase), nil
}

// AddBalance adds balance to an account (for testing/admin purposes)
func (s *AccountService) AddBalance(userID, accountID uint, amount float64) (*models.AccountResponse, error) {
	account, err := s.accountRepo.GetByIDAndUserID(accountID, userID)
	if err != nil {
		return nil, err
	}

	account.BalanceUSDT += amount
	if err := s.accountRepo.Update(account); err != nil {
		return nil, err
	}

	return s.buildAccountResponse(account, "", ""), nil
}

// buildAccountResponse builds an AccountResponse from an Account
func (s *AccountService) buildAccountResponse(account *models.Account, apiSecret, passphrase string) *models.AccountResponse {
	return &models.AccountResponse{
		ID:              account.ID,
		ExchangeType:    account.ExchangeType,
		APIKey:          account.APIKey,
		APISecret:       apiSecret,
		Passphrase:      passphrase,
		BalanceUSDT:     account.BalanceUSDT,
		InitialBalance:  account.InitialBalance,
		MarginMode:      account.MarginMode,
		HedgeMode:       account.HedgeMode,
		DefaultLeverage: account.DefaultLeverage,
		MakerFeeRate:    account.MakerFeeRate,
		TakerFeeRate:    account.TakerFeeRate,
		EndpointURL:     s.getEndpointURL(account.ExchangeType),
		CreatedAt:       account.CreatedAt,
	}
}

// getEndpointURL returns the endpoint URL for an exchange type
func (s *AccountService) getEndpointURL(exchangeType models.ExchangeType) string {
	return fmt.Sprintf("https://sim-%s.%s", exchangeType, s.baseURL)
}
