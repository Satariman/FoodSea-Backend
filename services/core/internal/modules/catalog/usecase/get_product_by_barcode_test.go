package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestGetProductByBarcode_ValidEAN13_Found(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}

	barcode := "4607025390015"
	detail := fakeProductDetail()
	detail.Barcode = &barcode

	repo.On("GetByBarcode", mock.Anything, barcode).Return(detail, nil)
	c.On("SetProduct", mock.Anything, detail).Return(nil)

	uc := usecase.NewGetProductByBarcode(repo, c, silentLogger())
	result, err := uc.Execute(context.Background(), barcode)

	require.NoError(t, err)
	assert.Equal(t, detail, result)
}

func TestGetProductByBarcode_InvalidFormat(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}

	tests := []string{"abc", "123456", "12345678901234", "460702539001X"}
	uc := usecase.NewGetProductByBarcode(repo, c, silentLogger())

	for _, code := range tests {
		_, err := uc.Execute(context.Background(), code)
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput, "code=%q should be invalid", code)
	}

	repo.AssertNotCalled(t, "GetByBarcode")
}

func TestGetProductByBarcode_ValidFormat_NotFound(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}

	barcode := "4607025390015"
	repo.On("GetByBarcode", mock.Anything, barcode).Return(nil, sherrors.ErrNotFound)

	uc := usecase.NewGetProductByBarcode(repo, c, silentLogger())
	_, err := uc.Execute(context.Background(), barcode)

	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}

func TestGetProductByBarcode_ValidEAN8(t *testing.T) {
	repo := &MockProductRepository{}
	c := &MockProductCache{}

	// EAN-8: 12345670 — check digit: (1+3+5+7)*1 + (2+4+6)*3 = 16 + 36 = 52 → (10 - 52%10)%10 = (10-2)%10 = 8 → not right
	// Let me compute a valid EAN-8. Use 01234565:
	// digits: 0,1,2,3,4,5,6,5
	// sum for positions 1-7:
	//   i=0 (pos from right = 6): even from right → weight 1 → 0
	//   i=1 (pos from right = 5): odd → weight 3 → 3
	//   i=2 (pos from right = 4): even → weight 1 → 2
	//   i=3 (pos from right = 3): odd → weight 3 → 9
	//   i=4 (pos from right = 2): even → weight 1 → 4
	//   i=5 (pos from right = 1): odd → weight 3 → 15
	//   i=6 (pos from right = 0): even → weight 1 → 6
	// sum = 0+3+2+9+4+15+6 = 39 → check = (10 - 39%10)%10 = (10-9)%10 = 1
	// Wait, n=8, so (n-1-i)%2:
	// i=0: (7-0)%2 = 1 → weight 3 → 0*3=0
	// i=1: (7-1)%2 = 0 → weight 1 → 1
	// i=2: (7-2)%2 = 1 → weight 3 → 6
	// i=3: (7-3)%2 = 0 → weight 1 → 3
	// i=4: (7-4)%2 = 1 → weight 3 → 12
	// i=5: (7-5)%2 = 0 → weight 1 → 5
	// i=6: (7-6)%2 = 1 → weight 3 → 18
	// sum = 0+1+6+3+12+5+18 = 45 → check = (10-5)%10 = 5
	// So 01234565 is a valid EAN-8.
	barcode := "01234565"

	detail := &domain.ProductDetail{
		Product:  fakeProduct(),
		Category: fakeCategory("Food"),
	}
	detail.Barcode = &barcode

	repo.On("GetByBarcode", mock.Anything, barcode).Return(detail, nil)
	c.On("SetProduct", mock.Anything, detail).Return(nil)

	uc := usecase.NewGetProductByBarcode(repo, c, silentLogger())
	result, err := uc.Execute(context.Background(), barcode)

	require.NoError(t, err)
	assert.Equal(t, detail, result)
}
