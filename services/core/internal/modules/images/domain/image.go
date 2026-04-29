package domain

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// ImageUploader is the service interface consumed by other modules that need to upload product images.
type ImageUploader interface {
	UploadProductImage(ctx context.Context, productID uuid.UUID, filename string, reader io.Reader, contentType string) (imageURL string, err error)
	DeleteProductImage(ctx context.Context, productID uuid.UUID) error
}

// ProductImageRepo is the storage port for reading and updating a product's image URL.
type ProductImageRepo interface {
	SetImageURL(ctx context.Context, productID uuid.UUID, url string) error
	GetImageURL(ctx context.Context, productID uuid.UUID) (string, error)
}
