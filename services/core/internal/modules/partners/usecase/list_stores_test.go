package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/modules/partners/usecase"
)

func TestListStores_ReturnsActiveStores(t *testing.T) {
	repo := &MockStoreRepository{}
	stores := []domain.Store{fakeStore(), fakeStore()}
	repo.On("ListActive", context.Background()).Return(stores, nil)

	uc := usecase.NewListStores(repo, silentLogger())
	result, err := uc.Execute(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestListStores_PropagatesRepoError(t *testing.T) {
	repo := &MockStoreRepository{}
	repo.On("ListActive", context.Background()).Return(nil, errors.New("db error"))

	uc := usecase.NewListStores(repo, silentLogger())
	_, err := uc.Execute(context.Background())

	require.Error(t, err)
}
