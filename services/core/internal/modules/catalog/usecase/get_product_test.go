package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestGetProduct_CacheHit(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	detail := fakeProductDetail()

	c.On("GetProduct", mock.Anything, detail.ID).Return(detail, nil)

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Equal(t, detail, result)
	repo.AssertNotCalled(t, "GetByIDWithDetails")
}

func TestGetProduct_CacheMiss_QueriesDB_AndSetsCache(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	detail := fakeProductDetail()

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	c.On("SetProduct", mock.Anything, detail).Return(nil)

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Equal(t, detail, result)
	c.AssertCalled(t, "SetProduct", mock.Anything, detail)
}

func TestGetProduct_NotFound(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	id := uuid.New()

	c.On("GetProduct", mock.Anything, id).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, id).Return(nil, sherrors.ErrNotFound)

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	_, err := uc.Execute(context.Background(), id)

	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}

func TestGetProduct_CacheError_FallsBackToDB(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	detail := fakeProductDetail()

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, errors.New("redis down"))
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	c.On("SetProduct", mock.Anything, detail).Return(nil)

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Equal(t, detail, result)
}

func TestGetProduct_CacheSetError_ReturnsResult(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	detail := fakeProductDetail()

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	c.On("SetProduct", mock.Anything, detail).Return(errors.New("redis down"))

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Equal(t, detail, result)
}

func TestGetProduct_OptionalFieldsMapping(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}

	desc := "A great product"
	comp := "Water, sugar"
	weight := "500 мл"
	barcode := "4607025390015"
	imgURL := "https://example.com/img.jpg"

	detail := &domain.ProductDetail{
		Product: domain.Product{
			ID:          uuid.New(),
			Name:        "Full Product",
			Description: &desc,
			Composition: &comp,
			Weight:      &weight,
			Barcode:     &barcode,
			ImageURL:    &imgURL,
			InStock:     true,
			CategoryID:  uuid.New(),
		},
		Category: fakeCategory("Cat"),
		Nutrition: &domain.Nutrition{Calories: 46, Protein: 0, Fat: 0, Carbohydrates: 11.5},
	}

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	c.On("SetProduct", mock.Anything, detail).Return(nil)

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Equal(t, &desc, result.Description)
	assert.Equal(t, &comp, result.Composition)
	assert.Equal(t, &weight, result.Weight)
	assert.Equal(t, &barcode, result.Barcode)
	assert.Equal(t, &imgURL, result.ImageURL)
	require.NotNil(t, result.Nutrition)
	assert.Equal(t, 46.0, result.Nutrition.Calories)
}

func TestGetProduct_BestOfferProvider_EnrichesBestOffer(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	bestOfferProvider := &MockBestOfferProvider{}
	detail := fakeProductDetail()

	bestOffer := &domain.BestOffer{
		StoreName:    "Пятёрочка",
		StoreSlug:    "pyaterochka",
		PriceKopecks: 8500,
	}

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	bestOfferProvider.On("GetBestOffer", mock.Anything, detail.ID).Return(bestOffer, nil)
	c.On("SetProduct", mock.Anything, mock.Anything).Return(nil)

	uc := usecase.NewGetProduct(repo, c, bestOfferProvider, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	require.NotNil(t, result.BestOffer)
	assert.Equal(t, "Пятёрочка", result.BestOffer.StoreName)
	assert.Equal(t, int64(8500), result.BestOffer.PriceKopecks)
}

func TestGetProduct_BestOfferProvider_ErrorLogsWarnButReturnsProduct(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	bestOfferProvider := &MockBestOfferProvider{}
	detail := fakeProductDetail()

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	bestOfferProvider.On("GetBestOffer", mock.Anything, detail.ID).Return(nil, assert.AnError)
	c.On("SetProduct", mock.Anything, mock.Anything).Return(nil)

	uc := usecase.NewGetProduct(repo, c, bestOfferProvider, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Nil(t, result.BestOffer)
}

func TestGetProduct_NilBestOfferProvider_SkipsBestOffer(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}
	detail := fakeProductDetail()

	c.On("GetProduct", mock.Anything, detail.ID).Return(nil, nil)
	repo.On("GetByIDWithDetails", mock.Anything, detail.ID).Return(detail, nil)
	c.On("SetProduct", mock.Anything, mock.Anything).Return(nil)

	uc := usecase.NewGetProduct(repo, c, nil, silentLogger())
	result, err := uc.Execute(context.Background(), detail.ID)

	require.NoError(t, err)
	assert.Nil(t, result.BestOffer)
}
