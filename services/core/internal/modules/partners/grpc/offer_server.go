package grpc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/modules/partners/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// OfferServer implements pb.OfferServiceServer.
type OfferServer struct {
	pb.UnimplementedOfferServiceServer
	getOffers    *usecase.GetOffersForProducts
	getDelivery  *usecase.GetDeliveryConditions
	log          *slog.Logger
}

func NewOfferServer(
	getOffers *usecase.GetOffersForProducts,
	getDelivery *usecase.GetDeliveryConditions,
	log *slog.Logger,
) *OfferServer {
	return &OfferServer{
		getOffers:   getOffers,
		getDelivery: getDelivery,
		log:         log,
	}
}

func (s *OfferServer) GetOffers(ctx context.Context, req *pb.GetOffersRequest) (*pb.GetOffersResponse, error) {
	if len(req.ProductIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "product_ids must not be empty")
	}

	productIDs := make([]uuid.UUID, 0, len(req.ProductIds))
	for _, raw := range req.ProductIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid product_id %q: %v", raw, err)
		}
		productIDs = append(productIDs, id)
	}

	offerMap, storeMap, err := s.getOffers.Execute(ctx, productIDs)
	if err != nil {
		s.log.ErrorContext(ctx, "GetOffers usecase failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	var protos []*pb.OfferProto
	for productID, offers := range offerMap {
		for _, o := range offers {
			proto := toOfferProto(productID, o, storeMap)
			protos = append(protos, proto)
		}
	}

	return &pb.GetOffersResponse{Offers: protos}, nil
}

func (s *OfferServer) GetDeliveryConditions(ctx context.Context, req *pb.GetDeliveryConditionsRequest) (*pb.GetDeliveryConditionsResponse, error) {
	if len(req.StoreIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "store_ids must not be empty")
	}

	storeIDs := make([]uuid.UUID, 0, len(req.StoreIds))
	for _, raw := range req.StoreIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid store_id %q: %v", raw, err)
		}
		storeIDs = append(storeIDs, id)
	}

	dcMap, err := s.getDelivery.Execute(ctx, storeIDs)
	if err != nil {
		if errors.Is(err, sherrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "delivery condition not found")
		}
		s.log.ErrorContext(ctx, "GetDeliveryConditions usecase failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	protos := make([]*pb.DeliveryConditionProto, 0, len(dcMap))
	for _, dc := range dcMap {
		protos = append(protos, toDeliveryProto(dc))
	}

	return &pb.GetDeliveryConditionsResponse{Conditions: protos}, nil
}

func toOfferProto(productID uuid.UUID, o domain.Offer, storeMap map[uuid.UUID]domain.Store) *pb.OfferProto {
	proto := &pb.OfferProto{
		ProductId:            productID.String(),
		StoreId:              o.StoreID.String(),
		PriceKopecks:         o.PriceKopecks,
		DiscountPercent:      int32(o.DiscountPercent),
		InStock:              o.InStock,
		OriginalPriceKopecks: o.OriginalPriceKopecks,
	}
	if s, ok := storeMap[o.StoreID]; ok {
		proto.StoreName = s.Name
	}
	return proto
}

func toDeliveryProto(dc domain.DeliveryCondition) *pb.DeliveryConditionProto {
	proto := &pb.DeliveryConditionProto{
		StoreId:             dc.StoreID.String(),
		MinOrderKopecks:     dc.MinOrderKopecks,
		DeliveryCostKopecks: dc.DeliveryCostKopecks,
	}
	if dc.FreeFromKopecks != nil {
		proto.FreeFromKopecks = dc.FreeFromKopecks
	}
	if dc.EstimatedMinutes != nil {
		proto.EstimatedMinutes = dc.EstimatedMinutes
	}
	return proto
}
