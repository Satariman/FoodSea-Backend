package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entproduct "github.com/foodsea/core/ent/product"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// ProductImageRepo implements domain.ProductImageRepo using Ent.
type ProductImageRepo struct {
	client *ent.Client
}

func NewProductImageRepo(client *ent.Client) *ProductImageRepo {
	return &ProductImageRepo{client: client}
}

func (r *ProductImageRepo) SetImageURL(ctx context.Context, productID uuid.UUID, url string) error {
	var ptr *string
	if url != "" {
		ptr = &url
	}

	n, err := r.client.Product.Update().
		Where(entproduct.ID(productID)).
		SetNillableImageURL(ptr).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("images: set image_url: %w", err)
	}
	if n == 0 {
		return sherrors.ErrNotFound
	}
	return nil
}

func (r *ProductImageRepo) GetImageURL(ctx context.Context, productID uuid.UUID) (string, error) {
	p, err := r.client.Product.Query().
		Where(entproduct.ID(productID)).
		Select(entproduct.FieldImageURL).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", sherrors.ErrNotFound
		}
		return "", fmt.Errorf("images: get image_url: %w", err)
	}
	if p.ImageURL == nil {
		return "", nil
	}
	return *p.ImageURL, nil
}
