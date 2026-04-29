package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/images/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// --- additional mock for s3Deleter ---

type mockS3Deleter struct{ mock.Mock }

func (m *mockS3Deleter) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockS3Deleter) KeyFromURL(url string) string {
	args := m.Called(url)
	return args.String(0)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- tests ---

func TestDeleteImage_WithExistingImage(t *testing.T) {
	s3 := &mockS3Deleter{}
	repo := &mockImageRepo{}

	productID := uuid.New()
	storedURL := "http://localhost:9000/product-images/products/x/y.jpg"
	key := "products/x/y.jpg"

	repo.On("GetImageURL", mock.Anything, productID).Return(storedURL, nil)
	s3.On("KeyFromURL", storedURL).Return(key)
	s3.On("Delete", mock.Anything, key).Return(nil)
	repo.On("SetImageURL", mock.Anything, productID, "").Return(nil)

	uc := usecase.NewDeleteImage(s3, repo, newTestLogger())

	require.NoError(t, uc.Execute(context.Background(), productID))
	s3.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestDeleteImage_NoImage_SkipsS3(t *testing.T) {
	s3 := &mockS3Deleter{}
	repo := &mockImageRepo{}

	productID := uuid.New()
	repo.On("GetImageURL", mock.Anything, productID).Return("", nil)
	repo.On("SetImageURL", mock.Anything, productID, "").Return(nil)

	uc := usecase.NewDeleteImage(s3, repo, newTestLogger())

	require.NoError(t, uc.Execute(context.Background(), productID))
	s3.AssertNotCalled(t, "Delete")
	s3.AssertNotCalled(t, "KeyFromURL")
}

func TestDeleteImage_RepoGetFails(t *testing.T) {
	s3 := &mockS3Deleter{}
	repo := &mockImageRepo{}

	productID := uuid.New()
	repo.On("GetImageURL", mock.Anything, productID).Return("", sherrors.ErrNotFound)

	uc := usecase.NewDeleteImage(s3, repo, newTestLogger())

	err := uc.Execute(context.Background(), productID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrNotFound))
}

func TestDeleteImage_S3DeleteFails_StillClearsDB(t *testing.T) {
	s3 := &mockS3Deleter{}
	repo := &mockImageRepo{}

	productID := uuid.New()
	storedURL := "http://localhost:9000/product-images/products/x/y.jpg"
	key := "products/x/y.jpg"

	repo.On("GetImageURL", mock.Anything, productID).Return(storedURL, nil)
	s3.On("KeyFromURL", storedURL).Return(key)
	s3.On("Delete", mock.Anything, key).Return(errors.New("s3 unreachable"))
	repo.On("SetImageURL", mock.Anything, productID, "").Return(nil)

	uc := usecase.NewDeleteImage(s3, repo, newTestLogger())

	require.NoError(t, uc.Execute(context.Background(), productID))
	repo.AssertCalled(t, "SetImageURL", mock.Anything, productID, "")
}
