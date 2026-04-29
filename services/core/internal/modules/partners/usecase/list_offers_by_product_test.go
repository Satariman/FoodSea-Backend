package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/modules/partners/usecase"
)

func TestListOffersByProduct_CacheMiss_FetchesAndSets(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}
	cache := &MockOfferCache{}

	productID := uuid.New()
	store := fakeStore()
	offer := fakeOffer(productID, store.ID)

	cache.On("GetOffersByProduct", mock.Anything, productID).Return(nil, nil)
	offerRepo.On("ListByProduct", mock.Anything, productID).Return([]domain.Offer{offer}, nil)
	cache.On("SetOffersByProduct", mock.Anything, productID, mock.Anything).Return(nil)
	storeRepo.On("GetByID", mock.Anything, store.ID).Return(&store, nil)

	uc := usecase.NewListOffersByProduct(offerRepo, storeRepo, cache, silentLogger())
	result, err := uc.Execute(context.Background(), productID, false)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, store.ID, result[0].Store.ID)
	cache.AssertCalled(t, "SetOffersByProduct", mock.Anything, productID, mock.Anything)
}

func TestListOffersByProduct_CacheHit_SkipsRepo(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}
	cache := &MockOfferCache{}

	productID := uuid.New()
	store := fakeStore()
	offer := fakeOffer(productID, store.ID)

	cache.On("GetOffersByProduct", mock.Anything, productID).Return([]domain.Offer{offer}, nil)
	storeRepo.On("GetByID", mock.Anything, store.ID).Return(&store, nil)

	uc := usecase.NewListOffersByProduct(offerRepo, storeRepo, cache, silentLogger())
	result, err := uc.Execute(context.Background(), productID, false)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	offerRepo.AssertNotCalled(t, "ListByProduct", mock.Anything, mock.Anything)
}

func TestListOffersByProduct_SortedByPrice(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}
	cache := &MockOfferCache{}

	productID := uuid.New()
	store1 := fakeStore()
	store2 := fakeStore()

	cheap := fakeOffer(productID, store1.ID)
	cheap.PriceKopecks = 5000

	expensive := fakeOffer(productID, store2.ID)
	expensive.PriceKopecks = 15000

	cache.On("GetOffersByProduct", mock.Anything, productID).Return([]domain.Offer{expensive, cheap}, nil)
	storeRepo.On("GetByID", mock.Anything, store1.ID).Return(&store1, nil)
	storeRepo.On("GetByID", mock.Anything, store2.ID).Return(&store2, nil)

	uc := usecase.NewListOffersByProduct(offerRepo, storeRepo, cache, silentLogger())
	result, err := uc.Execute(context.Background(), productID, false)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Less(t, result[0].PriceKopecks, result[1].PriceKopecks)
}

func TestListOffersByProduct_HasDiscountOnly_FiltersNonDiscounted(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}
	cache := &MockOfferCache{}

	productID := uuid.New()
	store1 := fakeStore()
	store2 := fakeStore()

	originalPrice := int64(12000)
	discounted := fakeOffer(productID, store1.ID)
	discounted.PriceKopecks = 9000
	discounted.OriginalPriceKopecks = &originalPrice
	discounted.DiscountPercent = 25

	regular := fakeOffer(productID, store2.ID)
	regular.PriceKopecks = 10000

	cache.On("GetOffersByProduct", mock.Anything, productID).Return([]domain.Offer{discounted, regular}, nil)
	storeRepo.On("GetByID", mock.Anything, store1.ID).Return(&store1, nil)

	uc := usecase.NewListOffersByProduct(offerRepo, storeRepo, cache, silentLogger())
	result, err := uc.Execute(context.Background(), productID, true)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, store1.ID, result[0].Store.ID)
	storeRepo.AssertNotCalled(t, "GetByID", mock.Anything, store2.ID)
}
