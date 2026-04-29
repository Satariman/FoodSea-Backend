package usecase

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

func TestGetResultExecute_NotFound(t *testing.T) {
	repo := &mockRepo{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	id := uuid.New()

	repo.On("GetByID", mock.Anything, id).Return((*domain.OptimizationResult)(nil), domain.ErrResultNotFound).Once()

	uc := NewGetResult(repo, log)
	_, err := uc.Execute(context.Background(), id)
	require.ErrorIs(t, err, domain.ErrResultNotFound)
	repo.AssertExpectations(t)
}

func TestLockUnlockExecute(t *testing.T) {
	repo := &mockRepo{}
	events := &mockEvents{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	id := uuid.New()

	repo.On("Lock", mock.Anything, id).Return(nil).Once()
	events.On("ResultLocked", mock.Anything, id).Return(nil).Once()

	lockUC := NewLockResult(repo, events, log)
	require.NoError(t, lockUC.Execute(context.Background(), id))

	repo.On("Unlock", mock.Anything, id).Return(nil).Once()
	events.On("ResultUnlocked", mock.Anything, id).Return(nil).Once()

	unlockUC := NewUnlockResult(repo, events, log)
	require.NoError(t, unlockUC.Execute(context.Background(), id))

	repo.AssertExpectations(t)
	events.AssertExpectations(t)
}
