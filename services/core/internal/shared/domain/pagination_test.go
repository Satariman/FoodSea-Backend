package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/shared/domain"
)

func TestNewPagination_Defaults(t *testing.T) {
	p := domain.NewPagination(0, 0)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.PageSize)
}

func TestNewPagination_Clamp(t *testing.T) {
	p := domain.NewPagination(3, 150)
	assert.Equal(t, 3, p.Page)
	assert.Equal(t, 100, p.PageSize)
}

func TestNewPagination_NegativePage(t *testing.T) {
	p := domain.NewPagination(-5, 10)
	assert.Equal(t, 1, p.Page)
}

func TestPagination_Offset(t *testing.T) {
	p := domain.NewPagination(3, 20)
	assert.Equal(t, 40, p.Offset())
}

func TestPagination_Limit(t *testing.T) {
	p := domain.NewPagination(1, 50)
	assert.Equal(t, 50, p.Limit())
}

func TestPagination_FirstPage(t *testing.T) {
	p := domain.NewPagination(1, 20)
	assert.Equal(t, 0, p.Offset())
}
