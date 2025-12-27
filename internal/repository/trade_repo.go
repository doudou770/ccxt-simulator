package repository

import (
	"time"

	"github.com/ccxt-simulator/internal/models"
	"gorm.io/gorm"
)

// TradeRepository handles trade data access
type TradeRepository struct {
	db *gorm.DB
}

// NewTradeRepository creates a new TradeRepository
func NewTradeRepository(db *gorm.DB) *TradeRepository {
	return &TradeRepository{db: db}
}

// Create creates a new trade
func (r *TradeRepository) Create(trade *models.Trade) error {
	return r.db.Create(trade).Error
}

// GetByAccountID retrieves all trades for an account
func (r *TradeRepository) GetByAccountID(accountID uint) ([]models.Trade, error) {
	var trades []models.Trade
	result := r.db.Where("account_id = ?", accountID).Order("executed_at DESC").Find(&trades)
	return trades, result.Error
}

// GetByAccountIDPaginated retrieves trades with pagination
func (r *TradeRepository) GetByAccountIDPaginated(accountID uint, page, pageSize int) ([]models.Trade, int64, error) {
	var trades []models.Trade
	var total int64

	if err := r.db.Model(&models.Trade{}).Where("account_id = ?", accountID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	result := r.db.Where("account_id = ?", accountID).
		Order("executed_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&trades)

	return trades, total, result.Error
}

// GetByOrderID retrieves all trades for an order
func (r *TradeRepository) GetByOrderID(orderID uint) ([]models.Trade, error) {
	var trades []models.Trade
	result := r.db.Where("order_id = ?", orderID).Find(&trades)
	return trades, result.Error
}

// GetBySymbol retrieves trades by symbol
func (r *TradeRepository) GetBySymbol(accountID uint, symbol string) ([]models.Trade, error) {
	var trades []models.Trade
	result := r.db.Where("account_id = ? AND symbol = ?", accountID, symbol).Order("executed_at DESC").Find(&trades)
	return trades, result.Error
}

// GetTradesAfter retrieves trades executed after a specific time
func (r *TradeRepository) GetTradesAfter(accountID uint, after time.Time, limit int) ([]models.Trade, error) {
	var trades []models.Trade
	result := r.db.Where("account_id = ? AND executed_at > ?", accountID, after).
		Order("executed_at DESC").
		Limit(limit).
		Find(&trades)
	return trades, result.Error
}

// GetTotalFees calculates total fees paid
func (r *TradeRepository) GetTotalFees(accountID uint) (float64, error) {
	var total struct {
		Sum float64
	}
	err := r.db.Model(&models.Trade{}).
		Select("COALESCE(SUM(fee), 0) as sum").
		Where("account_id = ?", accountID).
		Scan(&total).Error
	return total.Sum, err
}

// GetTotalRealizedPnL calculates total realized PnL
func (r *TradeRepository) GetTotalRealizedPnL(accountID uint) (float64, error) {
	var total struct {
		Sum float64
	}
	err := r.db.Model(&models.Trade{}).
		Select("COALESCE(SUM(realized_pnl), 0) as sum").
		Where("account_id = ?", accountID).
		Scan(&total).Error
	return total.Sum, err
}

// ClosedPnLRepository handles closed PnL record data access
type ClosedPnLRepository struct {
	db *gorm.DB
}

// NewClosedPnLRepository creates a new ClosedPnLRepository
func NewClosedPnLRepository(db *gorm.DB) *ClosedPnLRepository {
	return &ClosedPnLRepository{db: db}
}

// Create creates a new closed PnL record
func (r *ClosedPnLRepository) Create(record *models.ClosedPnLRecord) error {
	return r.db.Create(record).Error
}

// GetByAccountID retrieves all closed PnL records for an account
func (r *ClosedPnLRepository) GetByAccountID(accountID uint) ([]models.ClosedPnLRecord, error) {
	var records []models.ClosedPnLRecord
	result := r.db.Where("account_id = ?", accountID).Order("closed_at DESC").Find(&records)
	return records, result.Error
}

// GetByAccountIDPaginated retrieves closed PnL records with pagination
func (r *ClosedPnLRepository) GetByAccountIDPaginated(accountID uint, page, pageSize int) ([]models.ClosedPnLRecord, int64, error) {
	var records []models.ClosedPnLRecord
	var total int64

	if err := r.db.Model(&models.ClosedPnLRecord{}).Where("account_id = ?", accountID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	result := r.db.Where("account_id = ?", accountID).
		Order("closed_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&records)

	return records, total, result.Error
}

// GetBySymbol retrieves closed PnL records by symbol
func (r *ClosedPnLRepository) GetBySymbol(accountID uint, symbol string) ([]models.ClosedPnLRecord, error) {
	var records []models.ClosedPnLRecord
	result := r.db.Where("account_id = ? AND symbol = ?", accountID, symbol).Order("closed_at DESC").Find(&records)
	return records, result.Error
}

// GetTotalClosedPnL calculates total closed PnL
func (r *ClosedPnLRepository) GetTotalClosedPnL(accountID uint) (float64, error) {
	var total struct {
		Sum float64
	}
	err := r.db.Model(&models.ClosedPnLRecord{}).
		Select("COALESCE(SUM(realized_pnl), 0) as sum").
		Where("account_id = ?", accountID).
		Scan(&total).Error
	return total.Sum, err
}

// GetClosedPnLByDateRange retrieves closed PnL within a date range
func (r *ClosedPnLRepository) GetClosedPnLByDateRange(accountID uint, start, end time.Time) ([]models.ClosedPnLRecord, error) {
	var records []models.ClosedPnLRecord
	result := r.db.Where("account_id = ? AND closed_at >= ? AND closed_at <= ?", accountID, start, end).
		Order("closed_at DESC").
		Find(&records)
	return records, result.Error
}
