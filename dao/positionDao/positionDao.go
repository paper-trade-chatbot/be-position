package positionDao

import (
	"errors"
	"time"

	"github.com/paper-trade-chatbot/be-common/pagination"
	"github.com/paper-trade-chatbot/be-position/models/dbModels"
	"github.com/paper-trade-chatbot/be-proto/general"
	"github.com/shopspring/decimal"

	"gorm.io/gorm"
)

const table = "position"

type OrderColumn int

const (
	OrderColumn_None OrderColumn = iota
	OrderColumn_ProductCode
	OrderColumn_CreatedAt
)

type OrderDirection int

const (
	OrderDirection_None = 0
	OrderDirection_ASC  = 1
	OrderDirection_DESC = -1
)

type Order struct {
	Column    OrderColumn
	Direction OrderDirection
}

// QueryModel set query condition, used by queryChain()
type QueryModel struct {
	ID             []uint64
	MemberID       []uint64
	ProductType    *dbModels.ProductType
	ExchangeCode   *string
	ProductCode    *string
	TradeType      *dbModels.TradeType
	PositionStatus *dbModels.PositionStatus
	ProcessState   *dbModels.ProcessState
	Amount         *decimal.Decimal
	UnitPrice      *decimal.Decimal
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	OrderBy        []*Order
}

type UpdateModel struct {
	PositionStatus *dbModels.PositionStatus
	ProcessState   *dbModels.ProcessState
	Amount         *decimal.Decimal
}

// New a row
func New(db *gorm.DB, model *dbModels.PositionModel) (int, error) {

	err := db.Table(table).
		Create(model).Error

	if err != nil {
		return 0, err
	}
	return 1, nil
}

// New rows
func News(db *gorm.DB, m []*dbModels.PositionModel) (int, error) {

	err := db.Transaction(func(tx *gorm.DB) error {

		err := tx.Table(table).
			CreateInBatches(m, 3000).Error

		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return len(m), nil
}

// Get return a record as raw-data-form
func Get(tx *gorm.DB, query *QueryModel) (*dbModels.PositionModel, error) {

	result := &dbModels.PositionModel{}
	err := tx.Table(table).
		Scopes(queryChain(query)).
		Scan(result).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Gets return records as raw-data-form
func Gets(tx *gorm.DB, query *QueryModel) ([]dbModels.PositionModel, error) {
	result := make([]dbModels.PositionModel, 0)
	err := tx.Table(table).
		Scopes(queryChain(query)).
		Scan(&result).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []dbModels.PositionModel{}, nil
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetsWithPagination(tx *gorm.DB, query *QueryModel, paginate *general.Pagination) ([]dbModels.PositionModel, *general.PaginationInfo, error) {

	var rows []dbModels.PositionModel
	var count int64 = 0
	err := tx.Table(table).
		Scopes(queryChain(query)).
		Count(&count).
		Scopes(paginateChain(paginate)).
		Scan(&rows).Error

	offset, _ := pagination.GetOffsetAndLimit(paginate)
	paginationInfo := pagination.SetPaginationDto(paginate.Page, paginate.PageSize, int32(count), int32(offset))

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []dbModels.PositionModel{}, paginationInfo, nil
	}

	if err != nil {
		return []dbModels.PositionModel{}, nil, err
	}

	return rows, paginationInfo, nil
}

// Gets return records as raw-data-form
func Modify(tx *gorm.DB, model *dbModels.PositionModel, lock *QueryModel, update *UpdateModel) error {
	attrs := map[string]interface{}{}
	if update.PositionStatus != nil {
		attrs["position_status"] = *update.PositionStatus
	}
	if update.Amount != nil {
		attrs["amount"] = *update.Amount
	}
	if update.ProcessState != nil {
		attrs["process_state"] = *update.ProcessState
	}

	if lock == nil {
		lock = &QueryModel{}
	}

	err := tx.Table(table).
		Model(dbModels.PositionModel{}).
		Where(table+".id = ?", model.ID).
		Scopes(queryChain(lock)).
		Updates(attrs).Error
	return err
}

func queryChain(query *QueryModel) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.
			Scopes(idInScope(query.ID)).
			Scopes(memberIDInScope(query.MemberID)).
			Scopes(productTypeEqualScope(query.ProductType)).
			Scopes(exchangeCodeEqualScope(query.ExchangeCode)).
			Scopes(productCodeEqualScope(query.ProductCode)).
			Scopes(tradeTypeEqualScope(query.TradeType)).
			Scopes(positionStatusEqualScope(query.PositionStatus)).
			Scopes(processStateEqualScope(query.ProcessState)).
			Scopes(amountEqualScope(query.Amount)).
			Scopes(unitPriceEqualScope(query.UnitPrice)).
			Scopes(createdBetweenScope(query.CreatedFrom, query.CreatedTo)).
			Scopes(orderByScope(query.OrderBy))
	}
}

func paginateChain(paginate *general.Pagination) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset, limit := pagination.GetOffsetAndLimit(paginate)
		return db.
			Scopes(offsetScope(offset)).
			Scopes(limitScope(limit))

	}
}

func idInScope(id []uint64) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(id) > 0 {
			return db.Where(table+".id IN ?", id)
		}
		return db
	}
}

func memberIDInScope(memberID []uint64) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(memberID) > 0 {
			return db.Where(table+".member_id IN ?", memberID)
		}
		return db
	}
}

func productTypeEqualScope(productType *dbModels.ProductType) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if productType != nil {
			return db.Where(table+".product_type = ?", *productType)
		}
		return db
	}
}

func exchangeCodeEqualScope(exchangeCode *string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if exchangeCode != nil {
			return db.Where(table+".exchange_code = ?", *exchangeCode)
		}
		return db
	}
}

func productCodeEqualScope(productCode *string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if productCode != nil {
			return db.Where(table+".product_code = ?", *productCode)
		}
		return db
	}
}

func tradeTypeEqualScope(tradeType *dbModels.TradeType) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if tradeType != nil {
			return db.Where(table+".trade_type = ?", *tradeType)
		}
		return db
	}
}

func positionStatusEqualScope(positionStatus *dbModels.PositionStatus) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if positionStatus != nil {
			return db.Where(table+".position_status = ?", *positionStatus)
		}
		return db.Where(table + ".position_status IN (1,2)")
	}
}

func processStateEqualScope(processState *dbModels.ProcessState) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if processState != nil {
			return db.Where(table+".process_state = ?", *processState)
		}
		return db
	}
}

func amountEqualScope(amount *decimal.Decimal) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if amount != nil {
			return db.Where(table+".amount = ?", *amount)
		}
		return db
	}
}

func unitPriceEqualScope(unitPrice *decimal.Decimal) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if unitPrice != nil {
			return db.Where(table+".unit_price = ?", *unitPrice)
		}
		return db
	}
}

func createdBetweenScope(createdFrom, createdTo *time.Time) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if createdFrom != nil && createdTo != nil {
			return db.Where(table+".created_at BETWEEN ? AND ?", createdFrom, createdTo)
		}
		return db
	}
}

func orderByScope(order []*Order) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(order) > 0 {
			for _, o := range order {
				orderClause := ""
				switch o.Column {
				case OrderColumn_ProductCode:
					orderClause += "product_code"
				case OrderColumn_CreatedAt:
					orderClause += "created_at"
				default:
					continue
				}

				switch o.Direction {
				case OrderDirection_ASC:
					orderClause += " ASC"
				case OrderDirection_DESC:
					orderClause += " DESC"
				}

				db = db.Order(orderClause)
			}
			return db
		}
		return db
	}
}

func limitScope(limit int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if limit > 0 {
			return db.Limit(limit)
		}
		return db
	}
}

func offsetScope(offset int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if offset > 0 {
			return db.Limit(offset)
		}
		return db
	}
}
