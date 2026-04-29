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

func TestGetBestOffer_NoOffers_ReturnsNil(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}

	productID := uuid.New()
	offerRepo.On("ListByProduct", mock.Anything, productID).Return([]domain.Offer{}, nil)

	uc := usecase.NewGetBestOffer(offerRepo, storeRepo)
	result, err := uc.GetBestOffer(context.Background(), productID)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetBestOffer_OnlyOutOfStock_ReturnsNil(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}

	productID := uuid.New()
	offer := fakeOffer(productID, uuid.New())
	offer.InStock = false

	offerRepo.On("ListByProduct", mock.Anything, productID).Return([]domain.Offer{offer}, nil)

	uc := usecase.NewGetBestOffer(offerRepo, storeRepo)
	result, err := uc.GetBestOffer(context.Background(), productID)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetBestOffer_InStock_ReturnsBestOffer(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}

	productID := uuid.New()
	store := fakeStore()
	offer := fakeOffer(productID, store.ID)
	offer.PriceKopecks = 8500

	offerRepo.On("ListByProduct", mock.Anything, productID).Return([]domain.Offer{offer}, nil)
	storeRepo.On("GetByID", mock.Anything, store.ID).Return(&store, nil)

	uc := usecase.NewGetBestOffer(offerRepo, storeRepo)
	result, err := uc.GetBestOffer(context.Background(), productID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, store.Name, result.StoreName)
	assert.Equal(t, int64(8500), result.PriceKopecks)
	assert.Nil(t, result.OriginalPriceKopecks)
}

func TestGetBestOffer_WithDiscount_SetsDiscountFields(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}

	productID := uuid.New()
	store := fakeStore()
	originalPrice := int64(10000)
	offer := fakeOffer(productID, store.ID)
	offer.PriceKopecks = 8000
	offer.OriginalPriceKopecks = &originalPrice
	offer.DiscountPercent = 20

	offerRepo.On("ListByProduct", mock.Anything, productID).Return([]domain.Offer{offer}, nil)
	storeRepo.On("GetByID", mock.Anything, store.ID).Return(&store, nil)

	uc := usecase.NewGetBestOffer(offerRepo, storeRepo)
	result, err := uc.GetBestOffer(context.Background(), productID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(8000), result.PriceKopecks)
	require.NotNil(t, result.OriginalPriceKopecks)
	assert.Equal(t, int64(10000), *result.OriginalPriceKopecks)
	assert.Equal(t, int8(20), result.DiscountPercent)
}

func TestGetBestOffer_PicksFirstInStock(t *testing.T) {
	offerRepo := &MockOfferRepository{}
	storeRepo := &MockStoreRepository{}

	productID := uuid.New()
	store1 := fakeStore()
	store2 := fakeStore()

	outOfStock := fakeOffer(productID, store1.ID)
	outOfStock.PriceKopecks = 5000
	outOfStock.InStock = false

	inStock := fakeOffer(productID, store2.ID)
	inStock.PriceKopecks = 8000

	offerRepo.On("ListByProduct", mock.Anything, productID).Return([]domain.Offer{outOfStock, inStock}, nil)
	storeRepo.On("GetByID", mock.Anything, store2.ID).Return(&store2, nil)

	uc := usecase.NewGetBestOffer(offerRepo, storeRepo)
	result, err := uc.GetBestOffer(context.Background(), productID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, store2.Name, result.StoreName)
	assert.Equal(t, int64(8000), result.PriceKopecks)
}
