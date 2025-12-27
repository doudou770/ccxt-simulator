package repository

import (
	"errors"

	"github.com/ccxt-simulator/internal/models"
	"gorm.io/gorm"
)

var (
	ErrAccountNotFound = errors.New("account not found")
)

// AccountRepository handles account data access
type AccountRepository struct {
	db *gorm.DB
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create creates a new account
func (r *AccountRepository) Create(account *models.Account) error {
	return r.db.Create(account).Error
}

// GetByID retrieves an account by ID
func (r *AccountRepository) GetByID(id uint) (*models.Account, error) {
	var account models.Account
	result := r.db.First(&account, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, result.Error
	}
	return &account, nil
}

// GetByIDAndUserID retrieves an account by ID and user ID
func (r *AccountRepository) GetByIDAndUserID(id, userID uint) (*models.Account, error) {
	var account models.Account
	result := r.db.Where("id = ? AND user_id = ?", id, userID).First(&account)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, result.Error
	}
	return &account, nil
}

// GetByAPIKey retrieves an account by API key
func (r *AccountRepository) GetByAPIKey(apiKey string) (*models.Account, error) {
	var account models.Account
	result := r.db.Where("api_key = ?", apiKey).First(&account)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, result.Error
	}
	return &account, nil
}

// GetByUserID retrieves all accounts for a user
func (r *AccountRepository) GetByUserID(userID uint) ([]models.Account, error) {
	var accounts []models.Account
	result := r.db.Where("user_id = ?", userID).Find(&accounts)
	if result.Error != nil {
		return nil, result.Error
	}
	return accounts, nil
}

// GetByUserIDPaginated retrieves accounts for a user with pagination
func (r *AccountRepository) GetByUserIDPaginated(userID uint, page, pageSize int) ([]models.Account, int64, error) {
	var accounts []models.Account
	var total int64

	// Count total
	if err := r.db.Model(&models.Account{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	result := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&accounts)

	if result.Error != nil {
		return nil, 0, result.Error
	}

	return accounts, total, nil
}

// Update updates an account
func (r *AccountRepository) Update(account *models.Account) error {
	return r.db.Save(account).Error
}

// UpdateBalance updates the account balance
func (r *AccountRepository) UpdateBalance(id uint, balance float64) error {
	return r.db.Model(&models.Account{}).Where("id = ?", id).Update("balance_usdt", balance).Error
}

// Delete soft deletes an account
func (r *AccountRepository) Delete(id uint) error {
	return r.db.Delete(&models.Account{}, id).Error
}

// DeleteByUserID soft deletes all accounts for a user
func (r *AccountRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.Account{}).Error
}

// CountByUserID counts accounts for a user
func (r *AccountRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Account{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// CountByUserIDAndExchange counts accounts for a user with specific exchange type
func (r *AccountRepository) CountByUserIDAndExchange(userID uint, exchangeType models.ExchangeType) (int64, error) {
	var count int64
	err := r.db.Model(&models.Account{}).
		Where("user_id = ? AND exchange_type = ?", userID, exchangeType).
		Count(&count).Error
	return count, err
}
