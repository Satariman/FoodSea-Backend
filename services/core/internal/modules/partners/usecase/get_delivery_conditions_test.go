package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/modules/partners/usecase"
)

func TestGetDeliveryConditions_ReturnsMappedConditions(t *testing.T) {
	repo := &MockDeliveryRepository{}

	storeID := uuid.New()
	dc := domain.DeliveryCondition{
		StoreID:             storeID,
		MinOrderKopecks:     50000,
		DeliveryCostKopecks: 15000,
	}
	repo.On("ListByStores", context.Background(), []uuid.UUID{storeID}).
		Return(map[uuid.UUID]domain.DeliveryCondition{storeID: dc}, nil)

	uc := usecase.NewGetDeliveryConditions(repo, silentLogger())
	result, err := uc.Execute(context.Background(), []uuid.UUID{storeID})

	require.NoError(t, err)
	assert.Contains(t, result, storeID)
	assert.Equal(t, int64(50000), result[storeID].MinOrderKopecks)
}
