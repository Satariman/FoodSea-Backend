package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/images/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type s3Uploader interface {
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error)
}

var allowedContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// UploadImage validates the content-type, generates a unique S3 key, uploads the file
// and persists the public URL on the product.
type UploadImage struct {
	s3   s3Uploader
	repo domain.ProductImageRepo
	log  *slog.Logger
}

func NewUploadImage(s3 s3Uploader, repo domain.ProductImageRepo, log *slog.Logger) *UploadImage {
	return &UploadImage{s3: s3, repo: repo, log: log}
}

func (uc *UploadImage) Execute(ctx context.Context, productID uuid.UUID, filename string, reader io.Reader, contentType string) (string, error) {
	ext, ok := allowedContentTypes[contentType]
	if !ok {
		return "", fmt.Errorf("%w: content-type %q is not allowed", sherrors.ErrInvalidInput, contentType)
	}

	_ = filename
	key := fmt.Sprintf("products/%s/%s.%s", productID, uuid.New(), ext)

	url, err := uc.s3.Upload(ctx, key, reader, contentType)
	if err != nil {
		return "", fmt.Errorf("images.UploadImage: s3 upload: %w", err)
	}

	if err := uc.repo.SetImageURL(ctx, productID, url); err != nil {
		uc.log.WarnContext(ctx, "images: s3 upload succeeded but url persist failed", "error", err, "url", url)
		return "", fmt.Errorf("images.UploadImage: persist url: %w", err)
	}

	return url, nil
}
