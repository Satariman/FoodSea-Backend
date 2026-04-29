package grpc

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/internal/modules/catalog/domain"
)

// CatalogServer implements pb.CatalogServiceServer.
type CatalogServer struct {
	pb.UnimplementedCatalogServiceServer
	repo domain.ProductRepository
	log  *slog.Logger
}

func NewCatalogServer(repo domain.ProductRepository, log *slog.Logger) *CatalogServer {
	return &CatalogServer{repo: repo, log: log}
}

func (s *CatalogServer) ListProductsForML(ctx context.Context, _ *pb.ListProductsForMLRequest) (*pb.ListProductsForMLResponse, error) {
	products, err := s.repo.ListAllForML(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "ListProductsForML failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	result := make([]*pb.ProductFeaturesProto, 0, len(products))
	for _, p := range products {
		protoProduct := &pb.ProductFeaturesProto{
			ProductId:     p.ID.String(),
			Name:          p.Name,
			Description:   strOrEmpty(p.Description),
			Composition:   strOrEmpty(p.Composition),
			CategoryId:    p.CategoryID.String(),
			SubcategoryId: uuidPtrToString(p.SubcategoryID),
			BrandId:       uuidPtrToString(p.BrandID),
			Weight:        strOrEmpty(p.Weight),
		}

		if p.Nutrition != nil {
			protoProduct.Calories = p.Nutrition.Calories
			protoProduct.Protein = p.Nutrition.Protein
			protoProduct.Fat = p.Nutrition.Fat
			protoProduct.Carbohydrates = p.Nutrition.Carbohydrates
		}

		protoProduct.Offers = make([]*pb.ProductOfferBrief, 0, len(p.Offers))
		for _, o := range p.Offers {
			protoProduct.Offers = append(protoProduct.Offers, &pb.ProductOfferBrief{
				StoreId:      o.StoreID.String(),
				PriceKopecks: o.PriceKopecks,
			})
		}

		result = append(result, protoProduct)
	}

	return &pb.ListProductsForMLResponse{Products: result}, nil
}

func strOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func uuidPtrToString(v *uuid.UUID) string {
	if v == nil {
		return ""
	}
	return v.String()
}
