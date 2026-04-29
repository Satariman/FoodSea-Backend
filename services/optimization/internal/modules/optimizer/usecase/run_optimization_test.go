package usecase

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	analogsdomain "github.com/foodsea/optimization/internal/modules/analogs/domain"
	"github.com/foodsea/optimization/internal/modules/optimizer/algorithm"
	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	pbcore "github.com/foodsea/proto/core"
)

type mockCartClient struct{ mock.Mock }

func (m *mockCartClient) GetCartItems(ctx context.Context, in *pbcore.GetCartItemsRequest, opts ...grpc.CallOption) (*pbcore.GetCartItemsResponse, error) {
	args := m.Called(ctx, in)
	resp, _ := args.Get(0).(*pbcore.GetCartItemsResponse)
	return resp, args.Error(1)
}

func (m *mockCartClient) ClearCart(ctx context.Context, in *pbcore.ClearCartRequest, opts ...grpc.CallOption) (*pbcore.ClearCartResponse, error) {
	args := m.Called(ctx, in)
	resp, _ := args.Get(0).(*pbcore.ClearCartResponse)
	return resp, args.Error(1)
}

func (m *mockCartClient) RestoreCart(ctx context.Context, in *pbcore.RestoreCartRequest, opts ...grpc.CallOption) (*pbcore.RestoreCartResponse, error) {
	args := m.Called(ctx, in)
	resp, _ := args.Get(0).(*pbcore.RestoreCartResponse)
	return resp, args.Error(1)
}

type mockOfferClient struct{ mock.Mock }

func (m *mockOfferClient) GetOffers(ctx context.Context, in *pbcore.GetOffersRequest, opts ...grpc.CallOption) (*pbcore.GetOffersResponse, error) {
	args := m.Called(ctx, in)
	resp, _ := args.Get(0).(*pbcore.GetOffersResponse)
	return resp, args.Error(1)
}

func (m *mockOfferClient) GetDeliveryConditions(ctx context.Context, in *pbcore.GetDeliveryConditionsRequest, opts ...grpc.CallOption) (*pbcore.GetDeliveryConditionsResponse, error) {
	args := m.Called(ctx, in)
	resp, _ := args.Get(0).(*pbcore.GetDeliveryConditionsResponse)
	return resp, args.Error(1)
}

type mockAnalogProvider struct{ mock.Mock }

func (m *mockAnalogProvider) GetAnalogs(ctx context.Context, productID uuid.UUID, topK int) ([]analogsdomain.Analog, error) {
	args := m.Called(ctx, productID, topK)
	resp, _ := args.Get(0).([]analogsdomain.Analog)
	return resp, args.Error(1)
}

func (m *mockAnalogProvider) GetBatchAnalogsForStores(ctx context.Context, productIDs []uuid.UUID, topK int, storeIDs []uuid.UUID) (map[uuid.UUID][]analogsdomain.Analog, error) {
	args := m.Called(ctx, productIDs, topK, storeIDs)
	resp, _ := args.Get(0).(map[uuid.UUID][]analogsdomain.Analog)
	return resp, args.Error(1)
}

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Save(ctx context.Context, result *domain.OptimizationResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.OptimizationResult, error) {
	args := m.Called(ctx, id)
	resp, _ := args.Get(0).(*domain.OptimizationResult)
	return resp, args.Error(1)
}

func (m *mockRepo) FindByCartHash(ctx context.Context, cartHash string) (*domain.OptimizationResult, error) {
	args := m.Called(ctx, cartHash)
	resp, _ := args.Get(0).(*domain.OptimizationResult)
	return resp, args.Error(1)
}

func (m *mockRepo) Lock(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) Unlock(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) ExpireOld(ctx context.Context, olderThan time.Time) (int, error) {
	args := m.Called(ctx, olderThan)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) DeleteByUserCartHash(ctx context.Context, userID uuid.UUID, cartHash string) error {
	args := m.Called(ctx, userID, cartHash)
	return args.Error(0)
}

type mockEvents struct{ mock.Mock }

func (m *mockEvents) ResultCreated(ctx context.Context, result *domain.OptimizationResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *mockEvents) ResultLocked(ctx context.Context, resultID uuid.UUID) error {
	args := m.Called(ctx, resultID)
	return args.Error(0)
}

func (m *mockEvents) ResultUnlocked(ctx context.Context, resultID uuid.UUID) error {
	args := m.Called(ctx, resultID)
	return args.Error(0)
}

func TestRunOptimizationExecute_FullFlow(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cart := &mockCartClient{}
	offer := &mockOfferClient{}
	provider := &mockAnalogProvider{}
	repo := &mockRepo{}
	events := &mockEvents{}

	userID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()

	cart.On("GetCartItems", mock.Anything, &pbcore.GetCartItemsRequest{UserId: userID.String()}).Return(&pbcore.GetCartItemsResponse{
		Items: []*pbcore.CartItemProto{{ProductId: productID.String(), ProductName: "Milk", Quantity: 2}},
	}, nil).Once()
	offer.On("GetOffers", mock.Anything, mock.Anything).Return(&pbcore.GetOffersResponse{
		Offers: []*pbcore.OfferProto{{ProductId: productID.String(), StoreId: storeID.String(), StoreName: "Store", PriceKopecks: 120, InStock: true}},
	}, nil).Once()
	offer.On("GetDeliveryConditions", mock.Anything, mock.Anything).Return(&pbcore.GetDeliveryConditionsResponse{
		Conditions: []*pbcore.DeliveryConditionProto{{StoreId: storeID.String(), MinOrderKopecks: 0, DeliveryCostKopecks: 100}},
	}, nil).Once()
	repo.On("FindByCartHash", mock.Anything, mock.AnythingOfType("string")).Return((*domain.OptimizationResult)(nil), nil).Once()
	provider.On("GetBatchAnalogsForStores", mock.Anything, mock.Anything, 5, mock.Anything).Return(map[uuid.UUID][]analogsdomain.Analog{}, nil).Once()
	repo.On("Save", mock.Anything, mock.MatchedBy(func(result *domain.OptimizationResult) bool {
		return result.UserID == userID && result.Status == "active" && len(result.Items) == 1
	})).Return(nil).Once()
	events.On("ResultCreated", mock.Anything, mock.AnythingOfType("*domain.OptimizationResult")).Return(nil).Once()

	uc := NewRunOptimization(cart, offer, provider, algorithm.New(), repo, events, nil, 5*time.Second, log)
	result, err := uc.Execute(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, userID, result.UserID)
	require.Len(t, result.Items, 1)

	cart.AssertExpectations(t)
	offer.AssertExpectations(t)
	provider.AssertExpectations(t)
	repo.AssertExpectations(t)
	events.AssertExpectations(t)
}

func TestRunOptimizationExecute_CacheHit(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cart := &mockCartClient{}
	offer := &mockOfferClient{}
	provider := &mockAnalogProvider{}
	repo := &mockRepo{}
	events := &mockEvents{}

	userID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()
	cached := &domain.OptimizationResult{ID: uuid.New(), UserID: userID, Status: "active"}

	cart.On("GetCartItems", mock.Anything, mock.Anything).Return(&pbcore.GetCartItemsResponse{
		Items: []*pbcore.CartItemProto{{ProductId: productID.String(), ProductName: "Milk", Quantity: 1}},
	}, nil).Once()
	offer.On("GetOffers", mock.Anything, mock.Anything).Return(&pbcore.GetOffersResponse{
		Offers: []*pbcore.OfferProto{{ProductId: productID.String(), StoreId: storeID.String(), StoreName: "Store", PriceKopecks: 100, InStock: true}},
	}, nil).Once()
	offer.On("GetDeliveryConditions", mock.Anything, mock.Anything).Return(&pbcore.GetDeliveryConditionsResponse{
		Conditions: []*pbcore.DeliveryConditionProto{{StoreId: storeID.String(), MinOrderKopecks: 0, DeliveryCostKopecks: 100}},
	}, nil).Once()
	repo.On("FindByCartHash", mock.Anything, mock.AnythingOfType("string")).Return(cached, nil).Once()

	uc := NewRunOptimization(cart, offer, provider, algorithm.New(), repo, events, nil, 5*time.Second, log)
	result, err := uc.Execute(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, cached.ID, result.ID)

	provider.AssertNotCalled(t, "GetBatchAnalogsForStores", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "ResultCreated", mock.Anything, mock.Anything)
}

func TestRunOptimizationExecute_FetchesOffersForAnalogs(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cart := &mockCartClient{}
	offer := &mockOfferClient{}
	provider := &mockAnalogProvider{}
	repo := &mockRepo{}
	events := &mockEvents{}

	userID := uuid.New()
	productID := uuid.New()
	analogID := uuid.New()
	storeID := uuid.New()

	cart.On("GetCartItems", mock.Anything, mock.Anything).Return(&pbcore.GetCartItemsResponse{
		Items: []*pbcore.CartItemProto{{ProductId: productID.String(), ProductName: "Milk", Quantity: 1}},
	}, nil).Once()
	offer.On("GetOffers", mock.Anything, mock.MatchedBy(func(req *pbcore.GetOffersRequest) bool {
		return sameIDs(req.GetProductIds(), []string{productID.String()})
	})).Return(&pbcore.GetOffersResponse{
		Offers: []*pbcore.OfferProto{{ProductId: productID.String(), StoreId: storeID.String(), StoreName: "Store", PriceKopecks: 120, InStock: true}},
	}, nil).Once()
	offer.On("GetDeliveryConditions", mock.Anything, mock.Anything).Return(&pbcore.GetDeliveryConditionsResponse{
		Conditions: []*pbcore.DeliveryConditionProto{{StoreId: storeID.String(), MinOrderKopecks: 0, DeliveryCostKopecks: 100}},
	}, nil).Once()
	repo.On("FindByCartHash", mock.Anything, mock.AnythingOfType("string")).Return((*domain.OptimizationResult)(nil), domain.ErrResultNotFound).Once()
	provider.On("GetBatchAnalogsForStores", mock.Anything, mock.Anything, 5, mock.Anything).Return(map[uuid.UUID][]analogsdomain.Analog{
		productID: {{
			ProductID:       analogID,
			ProductName:     "Analog Milk",
			Score:           0.9,
			MinPriceKopecks: 90,
		}},
	}, nil).Once()
	offer.On("GetOffers", mock.Anything, mock.MatchedBy(func(req *pbcore.GetOffersRequest) bool {
		return sameIDs(req.GetProductIds(), []string{analogID.String()})
	})).Return(&pbcore.GetOffersResponse{
		Offers: []*pbcore.OfferProto{{ProductId: analogID.String(), StoreId: storeID.String(), StoreName: "Store", PriceKopecks: 90, InStock: true}},
	}, nil).Once()
	repo.On("Save", mock.Anything, mock.MatchedBy(func(result *domain.OptimizationResult) bool {
		return len(result.Substitutions) == 1 && result.Substitutions[0].AnalogID == analogID
	})).Return(nil).Once()
	events.On("ResultCreated", mock.Anything, mock.AnythingOfType("*domain.OptimizationResult")).Return(nil).Once()

	uc := NewRunOptimization(cart, offer, provider, algorithm.New(), repo, events, nil, 5*time.Second, log)
	result, err := uc.Execute(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, result.Substitutions, 1)
	require.Equal(t, analogID, result.Substitutions[0].AnalogID)
}

func TestRunOptimizationExecute_ErrorsOnIncompleteDeliveryConditions(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cart := &mockCartClient{}
	offer := &mockOfferClient{}
	provider := &mockAnalogProvider{}
	repo := &mockRepo{}
	events := &mockEvents{}

	userID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()

	cart.On("GetCartItems", mock.Anything, mock.Anything).Return(&pbcore.GetCartItemsResponse{
		Items: []*pbcore.CartItemProto{{ProductId: productID.String(), ProductName: "Milk", Quantity: 1}},
	}, nil).Once()
	offer.On("GetOffers", mock.Anything, mock.Anything).Return(&pbcore.GetOffersResponse{
		Offers: []*pbcore.OfferProto{{ProductId: productID.String(), StoreId: storeID.String(), StoreName: "Store", PriceKopecks: 120, InStock: true}},
	}, nil).Once()
	offer.On("GetDeliveryConditions", mock.Anything, mock.Anything).Return(&pbcore.GetDeliveryConditionsResponse{
		Conditions: []*pbcore.DeliveryConditionProto{},
	}, nil).Once()

	uc := NewRunOptimization(cart, offer, provider, algorithm.New(), repo, events, nil, 5*time.Second, log)
	_, err := uc.Execute(context.Background(), userID)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrDeliveryIncomplete)
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	joinedGot := strings.Join(append([]string(nil), got...), ",")
	joinedWant := strings.Join(append([]string(nil), want...), ",")
	return joinedGot == joinedWant
}
