package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/shared/domain"
)

func TestMoney_Add(t *testing.T) {
	a := domain.NewMoney(1000)
	b := domain.NewMoney(500)
	assert.Equal(t, int64(1500), a.Add(b).Kopecks())
}

func TestMoney_Sub(t *testing.T) {
	a := domain.NewMoney(1000)
	b := domain.NewMoney(300)
	assert.Equal(t, int64(700), a.Sub(b).Kopecks())
}

func TestMoney_Mul(t *testing.T) {
	m := domain.NewMoney(500)
	assert.Equal(t, int64(1500), m.Mul(3).Kopecks())
}

func TestMoney_IsZero(t *testing.T) {
	assert.True(t, domain.NewMoney(0).IsZero())
	assert.False(t, domain.NewMoney(1).IsZero())
}

func TestMoney_String(t *testing.T) {
	assert.Equal(t, "12.34 ₽", domain.NewMoney(1234).String())
	assert.Equal(t, "0.01 ₽", domain.NewMoney(1).String())
	assert.Equal(t, "1000.00 ₽", domain.NewMoney(100000).String())
}
