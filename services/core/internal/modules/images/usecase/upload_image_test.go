package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/images/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// --- mocks ---

type mockS3Uploader struct{ mock.Mock }

func (m *mockS3Uploader) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	args := m.Called(ctx, key, reader, contentType)
	return args.String(0), args.Error(1)
}

type mockImageRepo struct{ mock.Mock }

func (m *mockImageRepo) SetImageURL(ctx context.Context, productID uuid.UUID, url string) error {
	args := m.Called(ctx, productID, url)
	return args.Error(0)
}

func (m *mockImageRepo) GetImageURL(ctx context.Context, productID uuid.UUID) (string, error) {
	args := m.Called(ctx, productID)
	return args.String(0), args.Error(1)
}

// --- tests ---

func TestUploadImage_ValidJPEG(t *testing.T) {
	s3 := &mockS3Uploader{}
	repo := &mockImageRepo{}

	productID := uuid.New()
	wantURL := "http://localhost:9000/product-images/products/" + productID.String() + "/abc.jpg"

	s3.On("Upload", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0
	}), mock.Anything, "image/jpeg").Return(wantURL, nil)
	repo.On("SetImageURL", mock.Anything, productID, wantURL).Return(nil)

	uc := usecase.NewUploadImage(s3, repo, newTestLogger())

	url, err := uc.Execute(context.Background(), productID, "photo.jpg", bytes.NewReader([]byte("data")), "image/jpeg")

	require.NoError(t, err)
	assert.Equal(t, wantURL, url)
	s3.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestUploadImage_InvalidContentType(t *testing.T) {
	uc := usecase.NewUploadImage(&mockS3Uploader{}, &mockImageRepo{}, newTestLogger())

	_, err := uc.Execute(context.Background(), uuid.New(), "file.gif", bytes.NewReader([]byte("data")), "image/gif")

	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
}

func TestUploadImage_S3Failure(t *testing.T) {
	s3 := &mockS3Uploader{}
	repo := &mockImageRepo{}

	s3.On("Upload", mock.Anything, mock.Anything, mock.Anything, "image/png").Return("", errors.New("s3 down"))

	uc := usecase.NewUploadImage(s3, repo, newTestLogger())

	_, err := uc.Execute(context.Background(), uuid.New(), "img.png", bytes.NewReader([]byte{}), "image/png")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3 upload")
	repo.AssertNotCalled(t, "SetImageURL")
}

func TestUploadImage_RepoPersistFailure(t *testing.T) {
	s3 := &mockS3Uploader{}
	repo := &mockImageRepo{}

	productID := uuid.New()
	s3.On("Upload", mock.Anything, mock.Anything, mock.Anything, "image/webp").Return("http://x.com/k", nil)
	repo.On("SetImageURL", mock.Anything, productID, "http://x.com/k").Return(errors.New("db error"))

	uc := usecase.NewUploadImage(s3, repo, newTestLogger())

	_, err := uc.Execute(context.Background(), productID, "img.webp", bytes.NewReader([]byte{}), "image/webp")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "persist url")
}
