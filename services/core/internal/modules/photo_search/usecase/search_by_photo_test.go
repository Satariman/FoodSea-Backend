package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
	"github.com/foodsea/core/internal/modules/photo_search/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type mockPhotoClient struct{ mock.Mock }

func (m *mockPhotoClient) SearchByPhoto(ctx context.Context, req domain.SearchByPhotoRequest) (domain.SearchByPhotoResult, error) {
	args := m.Called(ctx, req)
	result, _ := args.Get(0).(domain.SearchByPhotoResult)
	return result, args.Error(1)
}

type mockProductLoader struct{ mock.Mock }

func (m *mockProductLoader) Execute(ctx context.Context, id uuid.UUID) (*catalogdomain.ProductDetail, error) {
	args := m.Called(ctx, id)
	product, _ := args.Get(0).(*catalogdomain.ProductDetail)
	return product, args.Error(1)
}

func TestSearchByPhoto_SkipsStaleProductsAndPreservesScoreOrder(t *testing.T) {
	client := &mockPhotoClient{}
	loader := &mockProductLoader{}
	uc := usecase.NewSearchByPhoto(client, loader)

	staleID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()

	client.On("SearchByPhoto", mock.Anything, mock.MatchedBy(func(req domain.SearchByPhotoRequest) bool {
		return req.TopK == 5 && req.ImageMimeType == "image/jpeg"
	})).Return(domain.SearchByPhotoResult{
		MatchedName:  "молоко",
		MatchedBrand: "простоквашино",
		Candidates: []domain.Candidate{
			{ProductID: staleID, Score: 0.99},
			{ProductID: firstID, Score: 0.91},
			{ProductID: secondID, Score: 0.62},
		},
	}, nil)

	loader.On("Execute", mock.Anything, staleID).Return((*catalogdomain.ProductDetail)(nil), sherrors.ErrNotFound)
	loader.On("Execute", mock.Anything, firstID).Return(&catalogdomain.ProductDetail{Product: catalogdomain.Product{ID: firstID, Name: "A"}}, nil)
	loader.On("Execute", mock.Anything, secondID).Return(&catalogdomain.ProductDetail{Product: catalogdomain.Product{ID: secondID, Name: "B"}}, nil)

	result, err := uc.Execute(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "  молоко 3.2% ",
	})
	require.NoError(t, err)
	require.Len(t, result.Candidates, 2)
	assert.Equal(t, firstID, result.Candidates[0].Product.ID)
	assert.Equal(t, 0.91, result.Candidates[0].Score)
	assert.Equal(t, secondID, result.Candidates[1].Product.ID)
	assert.Equal(t, 0.62, result.Candidates[1].Score)
	assert.Equal(t, "image_ocr", result.Candidates[0].Source)
	assert.Equal(t, "молоко", result.MatchedName)
	assert.Equal(t, "простоквашино", result.MatchedBrand)
}

func TestSearchByPhoto_Validation(t *testing.T) {
	uc := usecase.NewSearchByPhoto(&mockPhotoClient{}, &mockProductLoader{})

	_, err := uc.Execute(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "ok",
	})
	require.Error(t, err)
	var ve *sherrors.ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "ocr_text", ve.Field)
}
