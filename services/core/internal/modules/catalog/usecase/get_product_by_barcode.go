package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// GetProductByBarcode looks up a product by EAN-8 or EAN-13 barcode with Cache-Aside.
type GetProductByBarcode struct {
	products domain.ProductRepository
	cache    domain.ProductCache
	log      *slog.Logger
}

func NewGetProductByBarcode(products domain.ProductRepository, cache domain.ProductCache, log *slog.Logger) *GetProductByBarcode {
	return &GetProductByBarcode{products: products, cache: cache, log: log}
}

func (uc *GetProductByBarcode) Execute(ctx context.Context, code string) (*domain.ProductDetail, error) {
	if err := validateBarcode(code); err != nil {
		return nil, err
	}

	product, err := uc.products.GetByBarcode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("catalog.GetProductByBarcode: %w", err)
	}

	if cacheErr := uc.cache.SetProduct(ctx, product); cacheErr != nil {
		uc.log.WarnContext(ctx, "catalog: barcode product cache set error", "error", cacheErr)
	}

	return product, nil
}

// ByBarcode implements the ProductGetter interface used by the barcode module.
func (uc *GetProductByBarcode) ByBarcode(ctx context.Context, code string) (*domain.ProductDetail, error) {
	return uc.Execute(ctx, code)
}

// validateBarcode checks that code is a valid EAN-8 or EAN-13 with correct check digit.
func validateBarcode(code string) error {
	invalid := func() error {
		return fmt.Errorf("%w: %s", sherrors.ErrInvalidInput,
			(&sherrors.ValidationError{Field: "barcode", Message: "must be a valid EAN-8 or EAN-13"}).Error())
	}

	n := len(code)
	if n != 8 && n != 13 {
		return invalid()
	}

	digits := make([]int, n)
	for i, ch := range code {
		if ch < '0' || ch > '9' {
			return invalid()
		}
		digits[i] = int(ch - '0')
	}

	var sum int
	for i := 0; i < n-1; i++ {
		if (n-1-i)%2 == 1 {
			// even position from the right (0-indexed): weight 3
			sum += digits[i] * 3
		} else {
			sum += digits[i]
		}
	}
	check := (10 - sum%10) % 10
	if check != digits[n-1] {
		return invalid()
	}

	return nil
}
