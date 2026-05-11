package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestSearchByPhoto_MLClientReceivesDeadlineContext(t *testing.T) {
	client := &mockPhotoClient{}
	loader := &mockProductLoader{}
	uc := usecase.NewSearchByPhoto(client, loader)

	productID := uuid.New()

	client.On("SearchByPhoto", mock.MatchedBy(func(ctx context.Context) bool {
		deadline, ok := ctx.Deadline()
		if !ok {
			return false
		}
		until := time.Until(deadline)
		return until >= 4500*time.Millisecond && until <= 5500*time.Millisecond
	}), mock.Anything).Return(domain.SearchByPhotoResult{
		Candidates: []domain.Candidate{
			{ProductID: productID, Score: 0.77},
		},
	}, nil)
	loader.On("Execute", mock.Anything, productID).Return(&catalogdomain.ProductDetail{
		Product: catalogdomain.Product{ID: productID, Name: "Milk"},
	}, nil)

	result, err := uc.Execute(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "молоко 3.2",
		TopK:          1,
	})
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
}

func TestSearchByPhoto_MLUnavailableErrorPropagates(t *testing.T) {
	client := &mockPhotoClient{}
	loader := &mockProductLoader{}
	uc := usecase.NewSearchByPhoto(client, loader)

	client.On("SearchByPhoto", mock.Anything, mock.Anything).Return(domain.SearchByPhotoResult{}, sherrors.ErrUnavailable)

	_, err := uc.Execute(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "молоко 3.2",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrUnavailable))
}

func TestSearchByPhoto_SkipsInvalidProductIDCandidate(t *testing.T) {
	client := &mockPhotoClient{}
	loader := &mockProductLoader{}
	uc := usecase.NewSearchByPhoto(client, loader)

	validID := uuid.New()

	client.On("SearchByPhoto", mock.Anything, mock.Anything).Return(domain.SearchByPhotoResult{
		Candidates: []domain.Candidate{
			{ProductID: uuid.Nil, Score: 0.99},
			{ProductID: validID, Score: 0.77},
		},
	}, nil)
	loader.On("Execute", mock.Anything, validID).Return(&catalogdomain.ProductDetail{
		Product: catalogdomain.Product{ID: validID, Name: "Milk"},
	}, nil)

	result, err := uc.Execute(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "молоко 3.2",
	})
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	assert.Equal(t, validID, result.Candidates[0].Product.ID)
	loader.AssertNotCalled(t, "Execute", mock.Anything, uuid.Nil)
}

func TestSearchByPhoto_EmptyCandidatesReturnsEmptyResult(t *testing.T) {
	client := &mockPhotoClient{}
	loader := &mockProductLoader{}
	uc := usecase.NewSearchByPhoto(client, loader)

	client.On("SearchByPhoto", mock.Anything, mock.Anything).Return(domain.SearchByPhotoResult{
		MatchedName:  "молоко",
		MatchedBrand: "бренд",
		Candidates:   []domain.Candidate{},
	}, nil)

	result, err := uc.Execute(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "молоко 3.2",
	})
	require.NoError(t, err)
	require.Empty(t, result.Candidates)
	assert.Equal(t, "молоко", result.MatchedName)
	assert.Equal(t, "бренд", result.MatchedBrand)
}
