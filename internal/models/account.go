package models

import (
	"time"

	"gorm.io/gorm"
)

// ExchangeType represents supported exchange types
type ExchangeType string

const (
	ExchangeBinance     ExchangeType = "binance"
	ExchangeOKX         ExchangeType = "okx"
	ExchangeBybit       ExchangeType = "bybit"
	ExchangeBitget      ExchangeType = "bitget"
	ExchangeHyperliquid ExchangeType = "hyperliquid"
)

// MarginMode represents the margin mode
type MarginMode string

const (
	MarginModeCross    MarginMode = "cross"
	MarginModeIsolated MarginMode = "isolated"
)

// Account represents a simulated exchange account
type Account struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	UserID               uint           `gorm:"index;not null" json:"user_id"`
	ExchangeType         ExchangeType   `gorm:"size:20;not null" json:"exchange_type"`
	APIKey               string         `gorm:"uniqueIndex;size:100;not null" json:"api_key"`
	APISecretEncrypted   string         `gorm:"size:255;not null" json:"-"`
	PassphraseEncrypted  string         `gorm:"size:255" json:"-"` // Only for OKX
	BalanceUSDT          float64        `gorm:"type:decimal(20,8);default:0" json:"balance_usdt"`
	InitialBalance       float64        `gorm:"type:decimal(20,8);default:0" json:"initial_balance"`
	MarginMode           MarginMode     `gorm:"size:20;default:'cross'" json:"margin_mode"`
	HedgeMode            bool           `gorm:"default:false" json:"hedge_mode"`
	DefaultLeverage      int            `gorm:"default:20" json:"default_leverage"`
	MakerFeeRate         float64        `gorm:"type:decimal(10,6);default:0.0002" json:"maker_fee_rate"`
	TakerFeeRate         float64        `gorm:"type:decimal(10,6);default:0.0004" json:"taker_fee_rate"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	User      User       `gorm:"foreignKey:UserID" json:"-"`
	Positions []Position `gorm:"foreignKey:AccountID" json:"positions,omitempty"`
	Orders    []Order    `gorm:"foreignKey:AccountID" json:"orders,omitempty"`
}

// TableName specifies the table name for Account model
func (Account) TableName() string {
	return "accounts"
}

// AccountResponse is the response structure for account (with decrypted secret)
type AccountResponse struct {
	ID              uint         `json:"id"`
	ExchangeType    ExchangeType `json:"exchange_type"`
	APIKey          string       `json:"api_key"`
	APISecret       string       `json:"api_secret,omitempty"`
	Passphrase      string       `json:"passphrase,omitempty"`
	BalanceUSDT     float64      `json:"balance_usdt"`
	InitialBalance  float64      `json:"initial_balance"`
	MarginMode      MarginMode   `json:"margin_mode"`
	HedgeMode       bool         `json:"hedge_mode"`
	DefaultLeverage int          `json:"default_leverage"`
	MakerFeeRate    float64      `json:"maker_fee_rate"`
	TakerFeeRate    float64      `json:"taker_fee_rate"`
	EndpointURL     string       `json:"endpoint_url"`
	CreatedAt       time.Time    `json:"created_at"`
}
