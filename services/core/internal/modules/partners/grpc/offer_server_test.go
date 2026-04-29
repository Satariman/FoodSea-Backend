package grpc_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/internal/modules/partners/domain"
	partgrpc "github.com/foodsea/core/internal/modules/partners/grpc"
	"github.com/foodsea/core/internal/modules/partners/usecase"
)

const bufSize = 1 << 20

// --- mocks ---

type mockOfferRepo struct{ mock.Mock }

func (m *mockOfferRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]domain.Offer, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.Offer)
	return v, args.Error(1)
}

func (m *mockOfferRepo) ListByProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]domain.Offer, error) {
	args := m.Called(ctx, productIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(map[uuid.UUID][]domain.Offer)
	return v, args.Error(1)
}

type mockStoreRepo struct{ mock.Mock }

func (m *mockStoreRepo) ListActive(ctx context.Context) ([]domain.Store, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.Store)
	return v, args.Error(1)
}

func (m *mockStoreRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Store, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.Store)
	return v, args.Error(1)
}

type mockDeliveryRepo struct{ mock.Mock }

func (m *mockDeliveryRepo) ListByStores(ctx context.Context, storeIDs []uuid.UUID) (map[uuid.UUID]domain.DeliveryCondition, error) {
	args := m.Called(ctx, storeIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(map[uuid.UUID]domain.DeliveryCondition)
	return v, args.Error(1)
}

func (m *mockDeliveryRepo) GetByStore(ctx context.Context, storeID uuid.UUID) (*domain.DeliveryCondition, error) {
	args := m.Called(ctx, storeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.DeliveryCondition)
	return v, args.Error(1)
}

// --- setup ---

func newServer(t *testing.T, offerRepo domain.OfferRepository, storeRepo domain.StoreRepository, deliveryRepo domain.DeliveryRepository) pb.OfferServiceClient {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	getOffersUC := usecase.NewGetOffersForProducts(offerRepo, storeRepo, log)
	getDeliveryUC := usecase.NewGetDeliveryConditions(deliveryRepo, log)
	srv := partgrpc.NewOfferServer(getOffersUC, getDeliveryUC, log)

	lis := bufconn.Listen(bufSize)
	grpcSrv := grpc.NewServer()
	pb.RegisterOfferServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewOfferServiceClient(conn)
}

// --- GetOffers tests ---

func TestGetOffers_ValidRequest_ReturnsOffers(t *testing.T) {
	offerRepo := &mockOfferRepo{}
	storeRepo := &mockStoreRepo{}
	deliveryRepo := &mockDeliveryRepo{}

	productID := uuid.New()
	storeID := uuid.New()
	store := domain.Store{ID: storeID, Name: "Пятёрочка", IsActive: true}
	offer := domain.Offer{
		ID:           uuid.New(),
		ProductID:    productID,
		StoreID:      storeID,
		PriceKopecks: 8990,
		InStock:      true,
	}

	offerRepo.On("ListByProducts", mock.Anything, mock.Anything).
		Return(map[uuid.UUID][]domain.Offer{productID: {offer}}, nil)
	storeRepo.On("GetByID", mock.Anything, storeID).Return(&store, nil)

	client := newServer(t, offerRepo, storeRepo, deliveryRepo)
	resp, err := client.GetOffers(context.Background(), &pb.GetOffersRequest{
		ProductIds: []string{productID.String()},
	})

	require.NoError(t, err)
	require.Len(t, resp.Offers, 1)
	assert.Equal(t, productID.String(), resp.Offers[0].ProductId)
	assert.Equal(t, int64(8990), resp.Offers[0].PriceKopecks)
	assert.Equal(t, "Пятёрочка", resp.Offers[0].StoreName)
}

func TestGetOffers_WithDiscount_ReturnsDiscountFields(t *testing.T) {
	offerRepo := &mockOfferRepo{}
	storeRepo := &mockStoreRepo{}
	deliveryRepo := &mockDeliveryRepo{}

	productID := uuid.New()
	storeID := uuid.New()
	store := domain.Store{ID: storeID, Name: "Лента", IsActive: true}
	originalPrice := int64(12000)
	offer := domain.Offer{
		ID:                   uuid.New(),
		ProductID:            productID,
		StoreID:              storeID,
		PriceKopecks:         9000,
		OriginalPriceKopecks: &originalPrice,
		DiscountPercent:      25,
		InStock:              true,
	}

	offerRepo.On("ListByProducts", mock.Anything, mock.Anything).
		Return(map[uuid.UUID][]domain.Offer{productID: {offer}}, nil)
	storeRepo.On("GetByID", mock.Anything, storeID).Return(&store, nil)

	client := newServer(t, offerRepo, storeRepo, deliveryRepo)
	resp, err := client.GetOffers(context.Background(), &pb.GetOffersRequest{
		ProductIds: []string{productID.String()},
	})

	require.NoError(t, err)
	require.Len(t, resp.Offers, 1)
	assert.Equal(t, int32(25), resp.Offers[0].DiscountPercent)
	require.NotNil(t, resp.Offers[0].OriginalPriceKopecks)
	assert.Equal(t, int64(12000), *resp.Offers[0].OriginalPriceKopecks)
}

func TestGetOffers_NoDiscount_OriginalPriceNil(t *testing.T) {
	offerRepo := &mockOfferRepo{}
	storeRepo := &mockStoreRepo{}
	deliveryRepo := &mockDeliveryRepo{}

	productID := uuid.New()
	storeID := uuid.New()
	store := domain.Store{ID: storeID, Name: "Дикси", IsActive: true}
	offer := domain.Offer{
		ID:              uuid.New(),
		ProductID:       productID,
		StoreID:         storeID,
		PriceKopecks:    9900,
		DiscountPercent: 0,
		InStock:         true,
	}

	offerRepo.On("ListByProducts", mock.Anything, mock.Anything).
		Return(map[uuid.UUID][]domain.Offer{productID: {offer}}, nil)
	storeRepo.On("GetByID", mock.Anything, storeID).Return(&store, nil)

	client := newServer(t, offerRepo, storeRepo, deliveryRepo)
	resp, err := client.GetOffers(context.Background(), &pb.GetOffersRequest{
		ProductIds: []string{productID.String()},
	})

	require.NoError(t, err)
	require.Len(t, resp.Offers, 1)
	assert.Equal(t, int32(0), resp.Offers[0].DiscountPercent)
	assert.Nil(t, resp.Offers[0].OriginalPriceKopecks)
}

func TestGetOffers_EmptyProductIDs_ReturnsInvalidArgument(t *testing.T) {
	client := newServer(t, &mockOfferRepo{}, &mockStoreRepo{}, &mockDeliveryRepo{})
	_, err := client.GetOffers(context.Background(), &pb.GetOffersRequest{ProductIds: []string{}})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetOffers_InvalidUUID_ReturnsInvalidArgument(t *testing.T) {
	client := newServer(t, &mockOfferRepo{}, &mockStoreRepo{}, &mockDeliveryRepo{})
	_, err := client.GetOffers(context.Background(), &pb.GetOffersRequest{
		ProductIds: []string{"not-a-uuid"},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetDeliveryConditions tests ---

func TestGetDeliveryConditions_ValidRequest_ReturnsConditions(t *testing.T) {
	offerRepo := &mockOfferRepo{}
	storeRepo := &mockStoreRepo{}
	deliveryRepo := &mockDeliveryRepo{}

	storeID := uuid.New()
	dc := domain.DeliveryCondition{
		StoreID:             storeID,
		MinOrderKopecks:     30000,
		DeliveryCostKopecks: 9900,
	}

	deliveryRepo.On("ListByStores", mock.Anything, mock.Anything).
		Return(map[uuid.UUID]domain.DeliveryCondition{storeID: dc}, nil)

	client := newServer(t, offerRepo, storeRepo, deliveryRepo)
	resp, err := client.GetDeliveryConditions(context.Background(), &pb.GetDeliveryConditionsRequest{
		StoreIds: []string{storeID.String()},
	})

	require.NoError(t, err)
	require.Len(t, resp.Conditions, 1)
	assert.Equal(t, storeID.String(), resp.Conditions[0].StoreId)
	assert.Equal(t, int64(30000), resp.Conditions[0].MinOrderKopecks)
}

func TestGetDeliveryConditions_EmptyStoreIDs_ReturnsInvalidArgument(t *testing.T) {
	client := newServer(t, &mockOfferRepo{}, &mockStoreRepo{}, &mockDeliveryRepo{})
	_, err := client.GetDeliveryConditions(context.Background(), &pb.GetDeliveryConditionsRequest{StoreIds: []string{}})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetDeliveryConditions_InvalidUUID_ReturnsInvalidArgument(t *testing.T) {
	client := newServer(t, &mockOfferRepo{}, &mockStoreRepo{}, &mockDeliveryRepo{})
	_, err := client.GetDeliveryConditions(context.Background(), &pb.GetDeliveryConditionsRequest{
		StoreIds: []string{"bad-uuid"},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
