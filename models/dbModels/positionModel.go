package dbModels

import (
	"time"

	"github.com/shopspring/decimal"
)

type PositionStatus int

const (
	PositionStatus_None   PositionStatus = iota
	PositionStatus_Open                  // 開倉
	PositionStatus_Closed                // 關倉
)

type ProcessState int

const (
	ProcessState_None           ProcessState = iota
	ProcessState_Open                        // 開倉
	ProcessState_PendingToClose              // 等待關倉
	ProcessState_Closed                      // 關倉
)

type ProductType int

const (
	ProductType_None ProductType = iota
	ProductType_Stock
	ProductType_Crypto
	ProductType_Forex
	ProductType_Futures
)

type TradeType int

const (
	TradeType_None TradeType = iota
	TradeType_Buy
	TradeType_Sell
)

type PositionModel struct {
	ID             uint64          `gorm:"column:id; primary_key"`
	MemberID       uint64          `gorm:"column:member_id"`
	ProductType    ProductType     `gorm:"column:product_type"`
	ExchangeCode   string          `gorm:"column:exchange_code"`
	ProductCode    string          `gorm:"column:product_code"`
	TradeType      TradeType       `gorm:"column:trade_type"`
	PositionStatus PositionStatus  `gorm:"column:position_status"`
	ProcessState   ProcessState    `gorm:"column:process_state"`
	Amount         decimal.Decimal `gorm:"column:amount"`
	UnitPrice      decimal.Decimal `gorm:"column:unit_price"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
	UpdatedAt      time.Time       `gorm:"column:updated_at"`
}
