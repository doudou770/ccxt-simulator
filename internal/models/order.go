package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderType represents the order type
type OrderType string

const (
	OrderTypeMarket       OrderType = "MARKET"
	OrderTypeLimit        OrderType = "LIMIT"
	OrderTypeStopLoss     OrderType = "STOP_LOSS"
	OrderTypeTakeProfit   OrderType = "TAKE_PROFIT"
	OrderTypeStopMarket   OrderType = "STOP_MARKET"
	OrderTypeTrailingStop OrderType = "TRAILING_STOP_MARKET"
)

// OrderSide represents the order side
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderStatus represents the order status
type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusExpired         OrderStatus = "EXPIRED"
	OrderStatusRejected        OrderStatus = "REJECTED"
)

// Order represents a trading order
type Order struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	AccountID     uint           `gorm:"index;not null" json:"account_id"`
	ClientOrderID string         `gorm:"size:50;index" json:"client_order_id"`
	Symbol        string         `gorm:"size:20;not null;index" json:"symbol"`
	Side          OrderSide      `gorm:"size:10;not null" json:"side"`
	PositionSide  PositionSide   `gorm:"size:10" json:"position_side"`
	Type          OrderType      `gorm:"size:20;not null" json:"type"`
	Quantity      float64        `gorm:"type:decimal(20,8);not null" json:"quantity"`
	Price         float64        `gorm:"type:decimal(20,8)" json:"price"`
	StopPrice     float64        `gorm:"type:decimal(20,8)" json:"stop_price"`
	FilledQty     float64        `gorm:"type:decimal(20,8);default:0" json:"filled_qty"`
	AvgPrice      float64        `gorm:"type:decimal(20,8)" json:"avg_price"`
	Status        OrderStatus    `gorm:"size:20;not null;default:'NEW'" json:"status"`
	ReduceOnly    bool           `gorm:"default:false" json:"reduce_only"`
	ClosePosition bool           `gorm:"default:false" json:"close_position"`
	TimeInForce   string         `gorm:"size:10;default:'GTC'" json:"time_in_force"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Account Account `gorm:"foreignKey:AccountID" json:"-"`
	Trades  []Trade `gorm:"foreignKey:OrderID" json:"trades,omitempty"`
}

// TableName specifies the table name for Order model
func (Order) TableName() string {
	return "orders"
}

// IsPending returns true if the order is still pending
func (o *Order) IsPending() bool {
	return o.Status == OrderStatusNew || o.Status == OrderStatusPartiallyFilled
}

// IsCompleted returns true if the order is completed (filled or canceled)
func (o *Order) IsCompleted() bool {
	return o.Status == OrderStatusFilled || o.Status == OrderStatusCanceled ||
		o.Status == OrderStatusExpired || o.Status == OrderStatusRejected
}
