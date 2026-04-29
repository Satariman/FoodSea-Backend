package grpc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	pb "github.com/foodsea/proto/optimization"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type getResultExecutor interface {
	Execute(ctx context.Context, resultID uuid.UUID) (*domain.OptimizationResult, error)
}

type lockExecutor interface {
	Execute(ctx context.Context, resultID uuid.UUID) error
}

type unlockExecutor interface {
	Execute(ctx context.Context, resultID uuid.UUID) error
}

// OptimizationServer implements optimization.OptimizationService for ordering saga.
type OptimizationServer struct {
	pb.UnimplementedOptimizationServiceServer
	getResult getResultExecutor
	lock      lockExecutor
	unlock    unlockExecutor
	log       *slog.Logger
}

func NewOptimizationServer(
	getResult getResultExecutor,
	lock lockExecutor,
	unlock unlockExecutor,
	log *slog.Logger,
) *OptimizationServer {
	return &OptimizationServer{getResult: getResult, lock: lock, unlock: unlock, log: log}
}

func (s *OptimizationServer) GetResult(ctx context.Context, req *pb.GetResultRequest) (*pb.GetResultResponse, error) {
	id, err := uuid.Parse(req.GetResultId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid result_id: %v", err)
	}

	result, err := s.getResult.Execute(ctx, id)
	if err != nil {
		return nil, toStatusError(err)
	}

	return &pb.GetResultResponse{Result: toProtoResult(result)}, nil
}

func (s *OptimizationServer) LockResult(ctx context.Context, req *pb.LockResultRequest) (*pb.LockResultResponse, error) {
	id, err := uuid.Parse(req.GetResultId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid result_id: %v", err)
	}

	if err = s.lock.Execute(ctx, id); err != nil {
		return nil, toStatusError(err)
	}
	return &pb.LockResultResponse{Success: true}, nil
}

func (s *OptimizationServer) UnlockResult(ctx context.Context, req *pb.UnlockResultRequest) (*pb.UnlockResultResponse, error) {
	id, err := uuid.Parse(req.GetResultId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid result_id: %v", err)
	}

	if err = s.unlock.Execute(ctx, id); err != nil {
		return nil, toStatusError(err)
	}
	return &pb.UnlockResultResponse{Success: true}, nil
}

func (s *OptimizationServer) RunOptimization(context.Context, *pb.RunOptimizationRequest) (*pb.RunOptimizationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RunOptimization is available via HTTP POST /api/v1/optimize")
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, domain.ErrResultNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrResultNotActive), errors.Is(err, domain.ErrResultLocked), errors.Is(err, domain.ErrResultNotLocked):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrEmptyCart), errors.Is(err, domain.ErrNoOffers), errors.Is(err, domain.ErrNoFeasibleSolution):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func toProtoResult(result *domain.OptimizationResult) *pb.OptimizationResultProto {
	items := make([]*pb.OptimizationItemProto, len(result.Items))
	for i, item := range result.Items {
		items[i] = &pb.OptimizationItemProto{
			ProductId:    item.ProductID.String(),
			ProductName:  item.ProductName,
			StoreId:      item.StoreID.String(),
			StoreName:    item.StoreName,
			Quantity:     int32(item.Quantity),
			PriceKopecks: item.Price,
		}
	}

	subs := make([]*pb.SubstitutionProto, len(result.Substitutions))
	for i := range result.Substitutions {
		sub := &result.Substitutions[i]
		subs[i] = &pb.SubstitutionProto{
			OriginalProductId:    sub.OriginalID.String(),
			OriginalProductName:  sub.OriginalProductName,
			AnalogProductId:      sub.AnalogID.String(),
			AnalogProductName:    sub.AnalogProductName,
			OriginalStoreId:      sub.OriginalStoreID.String(),
			NewStoreId:           sub.NewStoreID.String(),
			NewStoreName:         sub.NewStoreName,
			OldPriceKopecks:      sub.OldPriceKopecks,
			NewPriceKopecks:      sub.NewPriceKopecks,
			PriceDeltaKopecks:    sub.PriceDeltaKopecks,
			DeliveryDeltaKopecks: sub.DeliveryDeltaKopecks,
			TotalSavingKopecks:   sub.TotalSavingKopecks,
			Score:                sub.Score,
			IsCrossStore:         sub.IsCrossStore,
		}
	}

	return &pb.OptimizationResultProto{
		Id:              result.ID.String(),
		UserId:          result.UserID.String(),
		TotalKopecks:    result.TotalKopecks,
		DeliveryKopecks: result.DeliveryKopecks,
		SavingsKopecks:  result.SavingsKopecks,
		Status:          result.Status,
		Items:           items,
		Substitutions:   subs,
		IsApproximate:   result.IsApproximate,
	}
}
