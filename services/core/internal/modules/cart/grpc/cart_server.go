package grpc

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/internal/modules/cart/domain"
	"github.com/foodsea/core/internal/modules/cart/usecase"
)

// CartServer implements pb.CartServiceServer.
type CartServer struct {
	pb.UnimplementedCartServiceServer
	getCart     *usecase.GetCart
	clearCart   *usecase.ClearCart
	restoreCart *usecase.RestoreCart
	log         *slog.Logger
}

func NewCartServer(
	getCart *usecase.GetCart,
	clearCart *usecase.ClearCart,
	restoreCart *usecase.RestoreCart,
	log *slog.Logger,
) *CartServer {
	return &CartServer{
		getCart:     getCart,
		clearCart:   clearCart,
		restoreCart: restoreCart,
		log:         log,
	}
}

func (s *CartServer) GetCartItems(ctx context.Context, req *pb.GetCartItemsRequest) (*pb.GetCartItemsResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}

	cart, err := s.getCart.Execute(ctx, userID)
	if err != nil {
		s.log.ErrorContext(ctx, "GetCartItems usecase failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	items := make([]*pb.CartItemProto, len(cart.Items))
	for i, item := range cart.Items {
		items[i] = &pb.CartItemProto{
			ProductId:   item.ProductID.String(),
			ProductName: item.ProductName,
			Quantity:    int32(item.Quantity),
		}
	}
	return &pb.GetCartItemsResponse{Items: items}, nil
}

func (s *CartServer) ClearCart(ctx context.Context, req *pb.ClearCartRequest) (*pb.ClearCartResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}

	if err = s.clearCart.Execute(ctx, userID); err != nil {
		s.log.ErrorContext(ctx, "ClearCart usecase failed", "error", err)
		return &pb.ClearCartResponse{Success: false}, status.Error(codes.Internal, "internal error")
	}

	return &pb.ClearCartResponse{Success: true}, nil
}

func (s *CartServer) RestoreCart(ctx context.Context, req *pb.RestoreCartRequest) (*pb.RestoreCartResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}

	items := make([]domain.CartItem, 0, len(req.Items))
	for _, proto := range req.Items {
		productID, parseErr := uuid.Parse(proto.ProductId)
		if parseErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid product_id %q: %v", proto.ProductId, parseErr)
		}
		items = append(items, domain.CartItem{
			ProductID: productID,
			Quantity:  int16(proto.Quantity),
		})
	}

	if err = s.restoreCart.Execute(ctx, userID, items); err != nil {
		s.log.ErrorContext(ctx, "RestoreCart usecase failed", "error", err)
		return &pb.RestoreCartResponse{Success: false}, status.Error(codes.Internal, "internal error")
	}

	return &pb.RestoreCartResponse{Success: true}, nil
}

func parseUserID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid user_id %q: %v", raw, err)
	}
	return id, nil
}
