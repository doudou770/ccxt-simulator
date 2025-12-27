package models

import (
	"time"
)

// Trade represents a trade execution record
type Trade struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AccountID   uint      `gorm:"index;not null" json:"account_id"`
	OrderID     uint      `gorm:"index;not null" json:"order_id"`
	Symbol      string    `gorm:"size:20;not null;index" json:"symbol"`
	Side        OrderSide `gorm:"size:10;not null" json:"side"`
	Quantity    float64   `gorm:"type:decimal(20,8);not null" json:"quantity"`
	Price       float64   `gorm:"type:decimal(20,8);not null" json:"price"`
	Fee         float64   `gorm:"type:decimal(20,8)" json:"fee"`
	FeeCurrency string    `gorm:"size:10;default:'USDT'" json:"fee_currency"`
	RealizedPnL float64   `gorm:"type:decimal(20,8)" json:"realized_pnl"`
	IsMaker     bool      `gorm:"default:false" json:"is_maker"`
	ExecutedAt  time.Time `gorm:"index" json:"executed_at"`

	// Relations
	Account Account `gorm:"foreignKey:AccountID" json:"-"`
	Order   Order   `gorm:"foreignKey:OrderID" json:"-"`
}

// TableName specifies the table name for Trade model
func (Trade) TableName() string {
	return "trades"
}

// ClosedPnLRecord represents a closed position PnL record
type ClosedPnLRecord struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	AccountID    uint         `gorm:"index;not null" json:"account_id"`
	Symbol       string       `gorm:"size:20;not null;index" json:"symbol"`
	Side         PositionSide `gorm:"size:10;not null" json:"side"`
	Quantity     float64      `gorm:"type:decimal(20,8);not null" json:"quantity"`
	EntryPrice   float64      `gorm:"type:decimal(20,8);not null" json:"entry_price"`
	ExitPrice    float64      `gorm:"type:decimal(20,8);not null" json:"exit_price"`
	RealizedPnL  float64      `gorm:"type:decimal(20,8);not null" json:"realized_pnl"`
	TotalFee     float64      `gorm:"type:decimal(20,8)" json:"total_fee"`
	Leverage     int          `gorm:"not null" json:"leverage"`
	ClosedReason string       `gorm:"size:50" json:"closed_reason"` // manual, stop_loss, take_profit, liquidation
	OpenedAt     time.Time    `json:"opened_at"`
	ClosedAt     time.Time    `gorm:"index" json:"closed_at"`

	// Relations
	Account Account `gorm:"foreignKey:AccountID" json:"-"`
}

// TableName specifies the table name for ClosedPnLRecord model
func (ClosedPnLRecord) TableName() string {
	return "closed_pnl_records"
}
