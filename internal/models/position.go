package models

import (
	"time"

	"gorm.io/gorm"
)

// PositionSide represents the position side
type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
	PositionSideBoth  PositionSide = "BOTH" // For one-way mode
)

// Position represents an open position
type Position struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	AccountID        uint           `gorm:"index;not null" json:"account_id"`
	Symbol           string         `gorm:"size:20;not null;index" json:"symbol"`
	Side             PositionSide   `gorm:"size:10;not null" json:"side"`
	Quantity         float64        `gorm:"type:decimal(20,8);not null" json:"quantity"`
	EntryPrice       float64        `gorm:"type:decimal(20,8);not null" json:"entry_price"`
	MarkPrice        float64        `gorm:"type:decimal(20,8)" json:"mark_price"`
	Leverage         int            `gorm:"not null" json:"leverage"`
	MarginMode       MarginMode     `gorm:"size:20;not null" json:"margin_mode"`
	Margin           float64        `gorm:"type:decimal(20,8)" json:"margin"`
	UnrealizedPnL    float64        `gorm:"type:decimal(20,8)" json:"unrealized_pnl"`
	LiquidationPrice float64        `gorm:"type:decimal(20,8)" json:"liquidation_price"`
	StopLoss         *float64       `gorm:"type:decimal(20,8)" json:"stop_loss,omitempty"`
	TakeProfit       *float64       `gorm:"type:decimal(20,8)" json:"take_profit,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Account Account `gorm:"foreignKey:AccountID" json:"-"`
}

// TableName specifies the table name for Position model
func (Position) TableName() string {
	return "positions"
}

// CalculateUnrealizedPnL calculates the unrealized PnL based on current mark price
func (p *Position) CalculateUnrealizedPnL(markPrice float64) float64 {
	if p.Side == PositionSideLong {
		return (markPrice - p.EntryPrice) * p.Quantity
	}
	return (p.EntryPrice - markPrice) * p.Quantity
}

// CalculateLiquidationPrice calculates the liquidation price
func (p *Position) CalculateLiquidationPrice(maintenanceMarginRate float64) float64 {
	if p.Side == PositionSideLong {
		return p.EntryPrice * (1 - 1/float64(p.Leverage) + maintenanceMarginRate)
	}
	return p.EntryPrice * (1 + 1/float64(p.Leverage) - maintenanceMarginRate)
}
