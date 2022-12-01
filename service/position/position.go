package position

import (
	"context"
	"errors"

	common "github.com/paper-trade-chatbot/be-common"
	"github.com/paper-trade-chatbot/be-position/dao/positionDao"
	"github.com/paper-trade-chatbot/be-position/database"
	"github.com/paper-trade-chatbot/be-position/logging"
	"github.com/paper-trade-chatbot/be-position/models/dbModels"
	"github.com/paper-trade-chatbot/be-proto/position"
	"github.com/paper-trade-chatbot/be-proto/product"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type PositionIntf interface {
	OpenPosition(ctx context.Context, in *position.OpenPositionReq) (*position.OpenPositionRes, error)
	PendingToClosePosition(ctx context.Context, in *position.PendingToClosePositionReq) (*position.PendingToClosePositionRes, error)
	ClosePosition(ctx context.Context, in *position.ClosePositionReq) (*position.ClosePositionRes, error)
	GetPositions(ctx context.Context, in *position.GetPositionsReq) (*position.GetPositionsRes, error)
	ModifyPosition(ctx context.Context, in *position.ModifyPositionReq) (*position.ModifyPositionRes, error)
}

type PositionImpl struct {
	PositionClient position.PositionServiceClient
}

func New() PositionIntf {
	return &PositionImpl{}
}

func (impl *PositionImpl) OpenPosition(ctx context.Context, in *position.OpenPositionReq) (*position.OpenPositionRes, error) {
	logging.Info(ctx, "[OpenPosition] in: %#v", in)
	db := database.GetDB()

	amount, err := decimal.NewFromString(in.Amount)
	if err != nil {
		logging.Error(ctx, "[OpenPosition] amount failed: %v", err)
		return nil, err
	}
	unitPrice, err := decimal.NewFromString(in.UnitPrice)
	if err != nil {
		logging.Error(ctx, "[OpenPosition] unitPrice failed: %v", err)
		return nil, err
	}

	model := &dbModels.PositionModel{
		MemberID:       in.MemberID,
		ProductType:    dbModels.ProductType(in.Type),
		ExchangeCode:   in.ExchangeCode,
		ProductCode:    in.ProductCode,
		TradeType:      dbModels.TradeType(in.TradeType),
		PositionStatus: dbModels.PositionStatus_Open,
		ProcessState:   dbModels.ProcessState_Open,
		Amount:         amount,
		UnitPrice:      unitPrice,
	}

	_, err = positionDao.New(db, model)
	if err != nil {
		logging.Error(ctx, "[OpenPosition] New failed: %v", err)
		return nil, err
	}

	return &position.OpenPositionRes{
		Id: model.ID,
	}, nil
}

func (impl *PositionImpl) PendingToClosePosition(ctx context.Context, in *position.PendingToClosePositionReq) (*position.PendingToClosePositionRes, error) {
	logging.Info(ctx, "[PendingToClosePosition] in: %#v", in)
	db := database.GetDB()

	closeAmount, err := decimal.NewFromString(in.CloseAmount)
	if err != nil {
		logging.Error(ctx, "[PendingToClosePosition] closeAmount failed: %v", err)
		return nil, err
	}

	positionStatusOpen := dbModels.PositionStatus_Open
	model, err := positionDao.Get(db, &positionDao.QueryModel{
		ID:             []uint64{in.Id},
		PositionStatus: &positionStatusOpen,
	})
	if err != nil {
		logging.Error(ctx, "[PendingToClosePosition] Get failed: %v", err)
		return nil, err
	}
	if model == nil {
		logging.Error(ctx, "[PendingToClosePosition] Get failed: %v", common.ErrNoSuchPosition)
		return nil, common.ErrNoSuchPosition
	}

	if model.Amount.LessThan(closeAmount) || closeAmount.LessThanOrEqual(decimal.Zero) {
		logging.Error(ctx, "[PendingToClosePosition] failed: %v", common.ErrInvalidCloseAmount)
		return nil, common.ErrInvalidCloseAmount
	}

	if model.ProcessState != dbModels.ProcessState_Open {
		logging.Error(ctx, "[PendingToClosePosition] failed: %v", common.ErrProcessStateNotOpen)
		return nil, common.ErrProcessStateNotOpen
	}

	open := dbModels.ProcessState_Open
	pending := dbModels.ProcessState_PendingToClose
	update := &positionDao.UpdateModel{
		ProcessState: &pending,
	}

	lock := &positionDao.QueryModel{
		ProcessState: &open,
	}

	err = positionDao.Modify(db, model, lock, update)
	if err != nil {
		logging.Error(ctx, "[PendingToClosePosition] Modify failed: %v", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &position.PendingToClosePositionRes{
				PreemptSuccess: false,
			}, nil
		}
		return nil, err
	}

	return &position.PendingToClosePositionRes{
		PreemptSuccess: true,
	}, nil
}

func (impl *PositionImpl) ClosePosition(ctx context.Context, in *position.ClosePositionReq) (*position.ClosePositionRes, error) {
	logging.Info(ctx, "[ClosePosition] in: %#v", in)
	db := database.GetDB()
	deal := false
	retryCount := 0

	closeAmount, err := decimal.NewFromString(in.CloseAmount)
	if err != nil {
		logging.Error(ctx, "[ClosePosition] closeAmount failed: %v", err)
		return nil, err
	}

	for !deal && retryCount <= 10 {

		retryCount++

		positionStatus := dbModels.PositionStatus_Open

		model, err := positionDao.Get(db, &positionDao.QueryModel{
			ID:             []uint64{in.Id},
			PositionStatus: &positionStatus,
		})
		if err != nil {
			logging.Error(ctx, "[ClosePosition] Get failed: %v", err)
			return nil, err
		}
		if model == nil {
			logging.Error(ctx, "[ClosePosition] Get failed: %v", common.ErrNoSuchPosition)
			return nil, common.ErrNoSuchPosition
		}

		if model.Amount.LessThan(closeAmount) || closeAmount.LessThanOrEqual(decimal.Zero) {
			logging.Error(ctx, "[ClosePosition] position amount [%s], close amount [%s] failed: %v", model.Amount, closeAmount, common.ErrInvalidCloseAmount)
			logging.Error(ctx, "[ClosePosition] position %#v", model)
			return nil, common.ErrInvalidCloseAmount
		}

		if model.ProcessState != dbModels.ProcessState_PendingToClose {
			logging.Error(ctx, "[ClosePosition] failed: %v", common.ErrProcessStateNotPending)
			return nil, common.ErrProcessStateNotPending
		}

		res := &position.ClosePositionRes{}

		amountLeft := model.Amount.Sub(closeAmount)
		open := dbModels.ProcessState_Open
		update := &positionDao.UpdateModel{
			Amount:       &amountLeft,
			ProcessState: &open,
		}
		if amountLeft.Equal(decimal.Zero) {
			positionStatus := dbModels.PositionStatus_Closed
			update.PositionStatus = &positionStatus
			close := dbModels.ProcessState_Closed
			update.ProcessState = &close
			res.Closed = true
		}

		pending := dbModels.ProcessState_PendingToClose
		lock := &positionDao.QueryModel{
			Amount:       &model.Amount,
			ProcessState: &pending,
		}

		err = positionDao.Modify(db, model, lock, update)
		if err != nil {
			logging.Warn(ctx, "[ClosePosition] Modify failed: %v", err)
			continue
		}

		res.AmountLeft = amountLeft.String()
		return res, nil

	}
	logging.Error(ctx, "[ClosePosition] failed: %v", common.ErrExceedRetryTimes)
	return nil, common.ErrExceedRetryTimes
}

func (impl *PositionImpl) GetPositions(ctx context.Context, in *position.GetPositionsReq) (*position.GetPositionsRes, error) {
	logging.Info(ctx, "[GetPositions] in: %#v", in)
	db := database.GetDB()

	query := &positionDao.QueryModel{
		ID:           in.Id,
		MemberID:     in.MemberID,
		ExchangeCode: in.ExchangeCode,
		ProductCode:  in.ProductCode,
	}
	if in.Type != nil {
		t := dbModels.ProductType(*in.Type)
		query.ProductType = &t
	}
	if in.TradeType != nil {
		tradeType := dbModels.TradeType(*in.TradeType)
		query.TradeType = &tradeType
	}
	if in.Status != nil {
		status := dbModels.PositionStatus(*in.Status)
		query.PositionStatus = &status
	}

	orders := []*positionDao.Order{}
	for _, o := range in.Order {
		order := &positionDao.Order{}
		if o.OrderBy == position.GetPositionsReq_OrderBy_CreatedAt {
			order.Column = positionDao.OrderColumn_CreatedAt
		} else if o.OrderBy == position.GetPositionsReq_OrderBy_ProductCode {
			order.Column = positionDao.OrderColumn_ProductCode
		}

		if o.OrderDirection == position.GetPositionsReq_OrderDirection_ASC {
			order.Direction = positionDao.OrderDirection_ASC
		} else if o.OrderDirection == position.GetPositionsReq_OrderDirection_DESC {
			order.Direction = positionDao.OrderDirection_DESC
		}

		orders = append(orders, order)
	}
	query.OrderBy = orders

	models, paginationInfo, err := positionDao.GetsWithPagination(db, query, in.Pagination)
	if err != nil {
		logging.Error(ctx, "[GetPositions] GetsWithPagination failed: %v", err)
		return nil, err
	}

	positions := []*position.Position{}
	for _, m := range models {
		p := &position.Position{
			Id:           m.ID,
			MemberID:     m.MemberID,
			Type:         product.ProductType(m.ProductType),
			ExchangeCode: m.ExchangeCode,
			ProductCode:  m.ProductCode,
			TradeType:    position.TradeType(m.TradeType),
			Status:       position.PositionStatus(m.PositionStatus),
			ProcessState: position.ProcessState(m.ProcessState),
			Amount:       m.Amount.String(),
			UnitPrice:    m.UnitPrice.String(),
			CreatedAt:    m.CreatedAt.Unix(),
			UpdatedAt:    m.UpdatedAt.Unix(),
		}
		positions = append(positions, p)
	}

	return &position.GetPositionsRes{
		Positions:      positions,
		PaginationInfo: paginationInfo,
	}, nil

}

func (impl *PositionImpl) ModifyPosition(ctx context.Context, in *position.ModifyPositionReq) (*position.ModifyPositionRes, error) {
	return nil, common.ErrNotImplemented
}
