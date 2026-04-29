package usecase_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/partners/domain"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// MockStoreRepository mocks domain.StoreRepository.
type MockStoreRepository struct{ mock.Mock }

func (m *MockStoreRepository) ListActive(ctx context.Context) ([]domain.Store, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.Store)
	return v, args.Error(1)
}

func (m *MockStoreRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Store, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.Store)
	return v, args.Error(1)
}

// MockOfferRepository mocks domain.OfferRepository.
type MockOfferRepository struct{ mock.Mock }

func (m *MockOfferRepository) ListByProduct(ctx context.Context, productID uuid.UUID) ([]domain.Offer, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.Offer)
	return v, args.Error(1)
}

func (m *MockOfferRepository) ListByProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]domain.Offer, error) {
	args := m.Called(ctx, productIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(map[uuid.UUID][]domain.Offer)
	return v, args.Error(1)
}

// MockDeliveryRepository mocks domain.DeliveryRepository.
type MockDeliveryRepository struct{ mock.Mock }

func (m *MockDeliveryRepository) ListByStores(ctx context.Context, storeIDs []uuid.UUID) (map[uuid.UUID]domain.DeliveryCondition, error) {
	args := m.Called(ctx, storeIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(map[uuid.UUID]domain.DeliveryCondition)
	return v, args.Error(1)
}

func (m *MockDeliveryRepository) GetByStore(ctx context.Context, storeID uuid.UUID) (*domain.DeliveryCondition, error) {
	args := m.Called(ctx, storeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.DeliveryCondition)
	return v, args.Error(1)
}

// MockOfferCache mocks domain.OfferCache.
type MockOfferCache struct{ mock.Mock }

func (m *MockOfferCache) GetOffersByProduct(ctx context.Context, productID uuid.UUID) ([]domain.Offer, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.Offer)
	return v, args.Error(1)
}

func (m *MockOfferCache) SetOffersByProduct(ctx context.Context, productID uuid.UUID, offers []domain.Offer) error {
	args := m.Called(ctx, productID, offers)
	return args.Error(0)
}

func (m *MockOfferCache) Invalidate(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

// helpers

func fakeStore() domain.Store {
	return domain.Store{ID: uuid.New(), Name: "TestStore", Slug: "test-store", IsActive: true}
}

func fakeOffer(productID, storeID uuid.UUID) domain.Offer {
	return domain.Offer{
		ID:           uuid.New(),
		ProductID:    productID,
		StoreID:      storeID,
		PriceKopecks: 10000,
		InStock:      true,
	}
}
