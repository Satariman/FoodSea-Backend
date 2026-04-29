package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/images/domain"
)

type s3Deleter interface {
	Delete(ctx context.Context, key string) error
	KeyFromURL(url string) string
}

// DeleteImage fetches the current image URL, removes the object from S3 and clears the DB field.
type DeleteImage struct {
	s3   s3Deleter
	repo domain.ProductImageRepo
	log  *slog.Logger
}

func NewDeleteImage(s3 s3Deleter, repo domain.ProductImageRepo, log *slog.Logger) *DeleteImage {
	return &DeleteImage{s3: s3, repo: repo, log: log}
}

func (uc *DeleteImage) Execute(ctx context.Context, productID uuid.UUID) error {
	imageURL, err := uc.repo.GetImageURL(ctx, productID)
	if err != nil {
		return fmt.Errorf("images.DeleteImage: get url: %w", err)
	}

	if imageURL != "" {
		key := uc.s3.KeyFromURL(imageURL)
		if err := uc.s3.Delete(ctx, key); err != nil {
			uc.log.WarnContext(ctx, "images: s3 delete failed, clearing db record anyway", "error", err, "key", key)
		}
	}

	if err := uc.repo.SetImageURL(ctx, productID, ""); err != nil {
		return fmt.Errorf("images.DeleteImage: clear url: %w", err)
	}

	return nil
}
