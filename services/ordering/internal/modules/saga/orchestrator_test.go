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
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

// ─── test helpers ─────────────────────────────────────────────────────────────

func newTestOrchestrator(
	repo *mockSagaRepo,
	carts *mockCart,
	opt *mockOpt,
	orders *mockOrders,
	audit *mockAudit,
) *saga.Orchestrator {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return saga.NewOrchestrator(repo, carts, opt, orders, audit, log, saga.OrchestratorConfig{
		MaxStepAttempts: 3,
		MaxCompAttempts: 3,
	})
}

func fixedOptResult(userID, resultID uuid.UUID) *domain.OptimizationResult {
	return &domain.OptimizationResult{
		ID:              resultID,
		UserID:          userID,
		TotalKopecks:    1000,
		DeliveryKopecks: 100,
		Status:          "active",
		Items: []domain.OrderItemSnapshot{
			{ProductID: uuid.New(), StoreID: uuid.New(), StoreName: "Store A", Quantity: 2, PriceKopecks: 500},
		},
	}
}

// allowAuditAny stubs audit calls to always return nil (not under test in most scenarios).
func allowAuditAny(audit *mockAudit) {
	audit.On("PublishCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	audit.On("PublishReply", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

// allowUpdateStateAny stubs UpdateState to always succeed.
func allowUpdateStateAny(repo *mockSagaRepo) {
	repo.On("UpdateState", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

// ─── Happy path ───────────────────────────────────────────────────────────────

func TestOrchestrator_HappyPath(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.AnythingOfType("domain.CreatePendingInput")).
		Return(&domain.Order{ID: orderID}, nil)
	carts.On("ClearCart", mock.Anything, userID).Return(nil)
	ords.On("Confirm", mock.Anything, orderID).Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*domain.SagaState")).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	gotOrderID, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.NoError(t, err)
	assert.Equal(t, orderID, gotOrderID)
	opt.AssertCalled(t, "LockResult", mock.Anything, resultID)
	ords.AssertCalled(t, "CreatePending", mock.Anything, mock.Anything)
	carts.AssertCalled(t, "ClearCart", mock.Anything, userID)
	ords.AssertCalled(t, "Confirm", mock.Anything, orderID)
}

// ─── Fail step 1 ──────────────────────────────────────────────────────────────

func TestOrchestrator_FailStep1_NoCompensation(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()
	stepErr := errors.New("lock failed")

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(stepErr)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	opt.AssertNotCalled(t, "UnlockResult")
	ords.AssertNotCalled(t, "Cancel")
	carts.AssertNotCalled(t, "RestoreCart")
}

// ─── Fail step 2 ──────────────────────────────────────────────────────────────

func TestOrchestrator_FailStep2_CompensateStep1(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	opt.On("UnlockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	opt.AssertCalled(t, "UnlockResult", mock.Anything, resultID)
	ords.AssertNotCalled(t, "Cancel")
	carts.AssertNotCalled(t, "RestoreCart")
}

// ─── Fail step 3 ──────────────────────────────────────────────────────────────

func TestOrchestrator_FailStep3_CompensateStep2AndStep1(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	opt.On("UnlockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(&domain.Order{ID: orderID}, nil)
	carts.On("ClearCart", mock.Anything, userID).Return(errors.New("clear failed"))
	ords.On("Cancel", mock.Anything, orderID, mock.AnythingOfType("string")).Return(nil)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	ords.AssertCalled(t, "Cancel", mock.Anything, orderID, mock.Anything)
	opt.AssertCalled(t, "UnlockResult", mock.Anything, resultID)
	carts.AssertNotCalled(t, "RestoreCart")
}

// ─── Fail step 4 ──────────────────────────────────────────────────────────────

func TestOrchestrator_FailStep4_CompensateStep3_2_1(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	opt.On("UnlockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(&domain.Order{ID: orderID}, nil)
	carts.On("ClearCart", mock.Anything, userID).Return(nil)
	ords.On("Confirm", mock.Anything, orderID).Return(errors.New("confirm failed"))
	carts.On("RestoreCart", mock.Anything, userID, mock.Anything).Return(nil)
	ords.On("Cancel", mock.Anything, orderID, mock.AnythingOfType("string")).Return(nil)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	carts.AssertCalled(t, "RestoreCart", mock.Anything, userID, mock.Anything)
	ords.AssertCalled(t, "Cancel", mock.Anything, orderID, mock.Anything)
	opt.AssertCalled(t, "UnlockResult", mock.Anything, resultID)
}

// ─── Transient error retry ────────────────────────────────────────────────────

func TestOrchestrator_TransientErrorStep1_RetrySuccess(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	// LockResult fails twice with transient, succeeds on third attempt
	opt.On("LockResult", mock.Anything, resultID).Return(domain.ErrTransient).Once()
	opt.On("LockResult", mock.Anything, resultID).Return(domain.ErrTransient).Once()
	opt.On("LockResult", mock.Anything, resultID).Return(nil).Once()
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(&domain.Order{ID: orderID}, nil)
	carts.On("ClearCart", mock.Anything, userID).Return(nil)
	ords.On("Confirm", mock.Anything, orderID).Return(nil)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	gotID, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.NoError(t, err)
	assert.Equal(t, orderID, gotID)
	opt.AssertNumberOfCalls(t, "LockResult", 3)
}

// ─── Compensation retry ───────────────────────────────────────────────────────

func TestOrchestrator_CompensationRetry_EventualSuccess(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(nil, errors.New("create failed"))
	// UnlockResult fails twice then succeeds
	opt.On("UnlockResult", mock.Anything, resultID).Return(domain.ErrTransient).Once()
	opt.On("UnlockResult", mock.Anything, resultID).Return(domain.ErrTransient).Once()
	opt.On("UnlockResult", mock.Anything, resultID).Return(nil).Once()
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	// Saga failed (step 2), but compensations succeeded after retries.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "manual intervention")
	opt.AssertNumberOfCalls(t, "UnlockResult", 3)
}

func TestOrchestrator_CompensationPermanentFail_ManualIntervention(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(nil, errors.New("create failed"))
	// UnlockResult always fails — exceeds MaxCompAttempts (3)
	opt.On("UnlockResult", mock.Anything, resultID).Return(errors.New("permanent failure"))
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrManualIntervention)
}

// ─── Compensation: NotFound is success ───────────────────────────────────────

func TestOrchestrator_UnlockResult_NotFoundIsSuccess(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
	// UnlockResult returns NotFound → idempotent success
	opt.On("UnlockResult", mock.Anything, resultID).Return(domain.ErrNotFound)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrManualIntervention)
}

// ─── Idempotency: Confirm ErrConflict is success ─────────────────────────────

func TestOrchestrator_ConfirmErrConflict_TreatedAsSuccess(t *testing.T) {
	userID, resultID, orderID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	opt.On("GetResult", ctx, resultID).Return(fixedOptResult(userID, resultID), nil)
	opt.On("LockResult", mock.Anything, resultID).Return(nil)
	ords.On("CreatePending", mock.Anything, mock.Anything).Return(&domain.Order{ID: orderID}, nil)
	carts.On("ClearCart", mock.Anything, userID).Return(nil)
	ords.On("Confirm", mock.Anything, orderID).Return(sherrors.ErrConflict)
	repo.On("Create", ctx, mock.Anything).Return(nil)
	allowUpdateStateAny(repo)
	allowAuditAny(audit)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	gotID, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.NoError(t, err)
	assert.Equal(t, orderID, gotID)
}

// ─── GetResult user mismatch ─────────────────────────────────────────────────

func TestOrchestrator_UserMismatch_ReturnsUnauthorized(t *testing.T) {
	userID, resultID := uuid.New(), uuid.New()
	ctx := context.Background()

	repo, carts, opt, ords, audit := &mockSagaRepo{}, &mockCart{}, &mockOpt{}, &mockOrders{}, &mockAudit{}

	optResult := fixedOptResult(uuid.New(), resultID) // different userID
	opt.On("GetResult", ctx, resultID).Return(optResult, nil)

	orch := newTestOrchestrator(repo, carts, opt, ords, audit)
	_, err := orch.Start(ctx, domain.StartInput{UserID: userID, OptimizationResultID: resultID})

	require.Error(t, err)
	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}
