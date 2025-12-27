package repository

import (
	"errors"

	"github.com/ccxt-simulator/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

// PositionRepository handles position data access
type PositionRepository struct {
	db *gorm.DB
}

// NewPositionRepository creates a new PositionRepository
func NewPositionRepository(db *gorm.DB) *PositionRepository {
	return &PositionRepository{db: db}
}

// Create creates a new position
func (r *PositionRepository) Create(position *models.Position) error {
	return r.db.Create(position).Error
}

// GetByID retrieves a position by ID
func (r *PositionRepository) GetByID(id uint) (*models.Position, error) {
	var position models.Position
	result := r.db.First(&position, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrPositionNotFound
		}
		return nil, result.Error
	}
	return &position, nil
}

// GetByAccountID retrieves all positions for an account
func (r *PositionRepository) GetByAccountID(accountID uint) ([]models.Position, error) {
	var positions []models.Position
	result := r.db.Where("account_id = ?", accountID).Find(&positions)
	return positions, result.Error
}

// GetByAccountIDAndSymbol retrieves positions by account ID and symbol
func (r *PositionRepository) GetByAccountIDAndSymbol(accountID uint, symbol string) ([]models.Position, error) {
	var positions []models.Position
	result := r.db.Where("account_id = ? AND symbol = ?", accountID, symbol).Find(&positions)
	return positions, result.Error
}

// GetByAccountIDSymbolAndSide retrieves a position by account ID, symbol, and side
func (r *PositionRepository) GetByAccountIDSymbolAndSide(accountID uint, symbol string, side models.PositionSide) (*models.Position, error) {
	var position models.Position
	result := r.db.Where("account_id = ? AND symbol = ? AND side = ?", accountID, symbol, side).First(&position)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrPositionNotFound
		}
		return nil, result.Error
	}
	return &position, nil
}

// Update updates a position
func (r *PositionRepository) Update(position *models.Position) error {
	return r.db.Save(position).Error
}

// UpdateWithLock updates a position with row lock
func (r *PositionRepository) UpdateWithLock(id uint, updateFn func(*models.Position) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var position models.Position
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&position, id).Error; err != nil {
			return err
		}

		if err := updateFn(&position); err != nil {
			return err
		}

		return tx.Save(&position).Error
	})
}

// Delete soft deletes a position
func (r *PositionRepository) Delete(id uint) error {
	return r.db.Delete(&models.Position{}, id).Error
}

// DeleteByAccountID deletes all positions for an account
func (r *PositionRepository) DeleteByAccountID(accountID uint) error {
	return r.db.Where("account_id = ?", accountID).Delete(&models.Position{}).Error
}

// GetOpenPositionsCount counts open positions for an account
func (r *PositionRepository) GetOpenPositionsCount(accountID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Position{}).Where("account_id = ? AND quantity > 0", accountID).Count(&count).Error
	return count, err
}

// GetTotalUnrealizedPnL calculates total unrealized PnL for an account
func (r *PositionRepository) GetTotalUnrealizedPnL(accountID uint) (float64, error) {
	var total struct {
		Sum float64
	}
	err := r.db.Model(&models.Position{}).
		Select("COALESCE(SUM(unrealized_pnl), 0) as sum").
		Where("account_id = ?", accountID).
		Scan(&total).Error
	return total.Sum, err
}
