package saga_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/modules/saga"
	"github.com/foodsea/ordering/internal/modules/saga/domain"
)

// TestOrchestrator_RecoverPending_ResumeFromStep3 simulates a saga that was
// interrupted after step 2 (CreatePending completed, order_id in payload).
// Recovery should continue from step 3 without re-calling CreatePending.
func TestOrchestrator_RecoverPending_ResumeFromStep3(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	existingState := &domain.SagaState{
		ID:          uuid.New(),
		UserID:      userID,
		OrderID:     orderID,
		CurrentStep: 2,
		Status:      domain.SagaStatusPending,
		Payload: domain.SagaPayload{
			OptimizationResultID: resultID,
			OrderID:              &orderID,
			Items: []domain.OrderItemSnapshot{
				{ProductID: uuid.New(), StoreID: uuid.New(), Quantity: 1, PriceKopecks: 500},
			},
			TotalKopecks:    500,
			DeliveryKopecks: 50,
		},
	}

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	carts.On("ClearCart", mock.Anything, userID).Return(nil)
	ords.On("Confirm", mock.Anything, orderID).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := saga.NewOrchestrator(repo, carts, opt, ords, audit, log, saga.OrchestratorConfig{
		MaxStepAttempts: 3,
		MaxCompAttempts: 3,
	})

	gotOrderID, err := orch.Resume(ctx, existingState)

	require.NoError(t, err)
	assert.Equal(t, orderID, gotOrderID)
	// CreatePending must NOT be called — order already exists
	ords.AssertNotCalled(t, "CreatePending")
	// LockResult must NOT be called — already done
	opt.AssertNotCalled(t, "LockResult")
	carts.AssertCalled(t, "ClearCart", mock.Anything, userID)
	ords.AssertCalled(t, "Confirm", mock.Anything, orderID)
}

// TestOrchestrator_RecoverCompensating resumes a saga that crashed during compensations.
func TestOrchestrator_RecoverCompensating_RunsCompensations(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	existingState := &domain.SagaState{
		ID:          uuid.New(),
		UserID:      userID,
		OrderID:     orderID,
		CurrentStep: 2, // LockResult + CreatePending completed
		Status:      domain.SagaStatusCompensating,
		Payload: domain.SagaPayload{
			OptimizationResultID: resultID,
			OrderID:              &orderID,
			Items:                []domain.OrderItemSnapshot{},
			FailureReason:        "step 3 failed",
		},
	}

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	ords.On("Cancel", mock.Anything, orderID, mock.Anything).Return(nil)
	opt.On("UnlockResult", mock.Anything, resultID).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := saga.NewOrchestrator(repo, carts, opt, ords, audit, log, saga.OrchestratorConfig{
		MaxStepAttempts: 3,
		MaxCompAttempts: 3,
	})

	_, err := orch.Resume(ctx, existingState)

	// Resume returns error for compensating saga (it's already failed)
	require.Error(t, err)
	ords.AssertCalled(t, "Cancel", mock.Anything, orderID, mock.Anything)
	opt.AssertCalled(t, "UnlockResult", mock.Anything, resultID)
	carts.AssertNotCalled(t, "RestoreCart") // step 3 never ran
}

// TestOrchestrator_RecoverPending_AlreadyAtStep4 simulates a saga stuck at step 4.
func TestOrchestrator_RecoverPending_ResumeStep4Only(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	existingState := &domain.SagaState{
		ID:          uuid.New(),
		UserID:      userID,
		OrderID:     orderID,
		CurrentStep: 3,
		Status:      domain.SagaStatusPending,
		Payload: domain.SagaPayload{
			OptimizationResultID: resultID,
			OrderID:              &orderID,
			Items:                []domain.OrderItemSnapshot{},
			TotalKopecks:         100,
		},
	}

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	ords.On("Confirm", mock.Anything, orderID).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := saga.NewOrchestrator(repo, carts, opt, ords, audit, log, saga.OrchestratorConfig{})

	gotID, err := orch.Resume(ctx, existingState)

	require.NoError(t, err)
	assert.Equal(t, orderID, gotID)
	ords.AssertCalled(t, "Confirm", mock.Anything, orderID)
	carts.AssertNotCalled(t, "ClearCart")
	opt.AssertNotCalled(t, "LockResult")

	// Idempotency: none of the already-done steps called
	ords.AssertNotCalled(t, "CreatePending")
}

// TestOrchestrator_NonTransientError_NoRetry ensures non-transient errors fail fast.
func TestOrchestrator_NonTransientError_NoRetry(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	// Non-transient error on first attempt — must not retry
	permanentErr := errors.New("invalid argument")
	opt.On("LockResult", mock.Anything, resultID).Return(permanentErr)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := saga.NewOrchestrator(repo, carts, opt, ords, audit, log, saga.OrchestratorConfig{
		MaxStepAttempts: 3,
	})

	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	// LockResult called exactly once — no retry for non-transient error
	opt.AssertNumberOfCalls(t, "LockResult", 1)
}
