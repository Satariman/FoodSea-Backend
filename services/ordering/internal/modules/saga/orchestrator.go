package saga

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

const (
	defaultStepTimeout      = 30 * time.Second
	defaultMaxStepAttempts  = 3
	defaultMaxCompAttempts  = 5
	compensationCtxTimeout  = 5 * time.Minute
)

// Orchestrator executes the 4-step place-order saga with gRPC participants and Kafka audit trail.
type Orchestrator struct {
	repo            domain.SagaRepository
	carts           domain.CartParticipant
	optimization    domain.OptimizationParticipant
	orders          domain.OrdersParticipant
	audit           domain.SagaAuditPublisher
	log             *slog.Logger
	stepTimeout     time.Duration
	maxStepAttempts int
	maxCompAttempts int
}

// OrchestratorConfig allows optional tuning; zero values use defaults.
type OrchestratorConfig struct {
	StepTimeout     time.Duration
	MaxStepAttempts int
	MaxCompAttempts int
}

// NewOrchestrator constructs an Orchestrator with the given participants.
func NewOrchestrator(
	repo domain.SagaRepository,
	carts domain.CartParticipant,
	optimization domain.OptimizationParticipant,
	orders domain.OrdersParticipant,
	audit domain.SagaAuditPublisher,
	log *slog.Logger,
	cfg OrchestratorConfig,
) *Orchestrator {
	if cfg.StepTimeout == 0 {
		cfg.StepTimeout = defaultStepTimeout
	}
	if cfg.MaxStepAttempts == 0 {
		cfg.MaxStepAttempts = defaultMaxStepAttempts
	}
	if cfg.MaxCompAttempts == 0 {
		cfg.MaxCompAttempts = defaultMaxCompAttempts
	}
	return &Orchestrator{
		repo:            repo,
		carts:           carts,
		optimization:    optimization,
		orders:          orders,
		audit:           audit,
		log:             log,
		stepTimeout:     cfg.StepTimeout,
		maxStepAttempts: cfg.MaxStepAttempts,
		maxCompAttempts: cfg.MaxCompAttempts,
	}
}

// ─── Public API ───────────────────────────────────────────────────────────────

// Start launches a new saga for the given user and optimization result.
// It blocks until all 4 steps complete or compensations finish.
// Returns the created order ID on success.
func (o *Orchestrator) Start(ctx context.Context, input domain.StartInput) (uuid.UUID, error) {
	optResult, err := o.optimization.GetResult(ctx, input.OptimizationResultID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get optimization result: %w", err)
	}
	if optResult.UserID != input.UserID {
		return uuid.Nil, fmt.Errorf("%w: optimization result belongs to a different user", sherrors.ErrUnauthorized)
	}
	if optResult.Status != "active" {
		return uuid.Nil, fmt.Errorf("%w: optimization result is not active (status=%s)", sherrors.ErrConflict, optResult.Status)
	}

	s := &domain.SagaState{
		UserID: input.UserID,
		Status: domain.SagaStatusPending,
		Payload: domain.SagaPayload{
			OptimizationResultID: input.OptimizationResultID,
			Items:                optResult.Items,
			TotalKopecks:         optResult.TotalKopecks,
			DeliveryKopecks:      optResult.DeliveryKopecks,
		},
	}
	if err = o.repo.Create(ctx, s); err != nil {
		return uuid.Nil, fmt.Errorf("create saga state: %w", err)
	}

	return o.execute(ctx, s, input)
}

// Resume continues a saga loaded from the DB (used by RecoverPending).
// For pending sagas it continues forward; for compensating sagas it finishes compensations
// and always returns an error (the saga already failed — recovery just completes the cleanup).
func (o *Orchestrator) Resume(ctx context.Context, s *domain.SagaState) (uuid.UUID, error) {
	input := domain.StartInput{
		UserID:               s.UserID,
		OptimizationResultID: s.Payload.OptimizationResultID,
	}
	if s.Status == domain.SagaStatusCompensating {
		compErr := o.compensate(s.ID, s.CurrentStep, input, &s.Payload)
		_ = o.repo.UpdateState(context.Background(), s.ID, 0, domain.SagaStatusFailed, nil)
		if compErr != nil {
			return uuid.Nil, fmt.Errorf("%w: compensation failed during recovery: %v (original: %s)",
				domain.ErrManualIntervention, compErr, s.Payload.FailureReason)
		}
		return uuid.Nil, fmt.Errorf("saga %s recovered and compensated: %s", s.ID, s.Payload.FailureReason)
	}
	return o.execute(ctx, s, input)
}

// ─── Internal execution ───────────────────────────────────────────────────────

// execute runs forward steps starting from s.CurrentStep+1.
func (o *Orchestrator) execute(ctx context.Context, s *domain.SagaState, input domain.StartInput) (uuid.UUID, error) {
	completed := s.CurrentStep // last successfully persisted step

	// Step 1 — LockResult(optimization)
	if completed < 1 {
		o.publishCmd(ctx, s.ID, 1, "LockResult", nil)
		if err := o.runStep(ctx, func(c context.Context) error {
			return o.optimization.LockResult(c, input.OptimizationResultID)
		}); err != nil {
			o.publishReply(ctx, s.ID, 1, "failed", err)
			return uuid.Nil, o.fail(ctx, s.ID, completed, "step 1 LockResult", err, input, &s.Payload)
		}
		o.publishReply(ctx, s.ID, 1, "ok", nil)
		completed = 1
		_ = o.repo.UpdateState(ctx, s.ID, 1, domain.SagaStatusPending, nil)
	}

	// Step 2 — CreatePending(ordering)
	if completed < 2 {
		o.publishCmd(ctx, s.ID, 2, "CreatePending", nil)
		var orderID uuid.UUID
		if err := o.runStep(ctx, func(c context.Context) error {
			// Idempotency: if orderID already in payload from a previous attempt, skip creation.
			if s.Payload.OrderID != nil {
				orderID = *s.Payload.OrderID
				return nil
			}
			resID := input.OptimizationResultID
			order, err := o.orders.CreatePending(c, domain.CreatePendingInput{
				UserID:               input.UserID,
				OptimizationResultID: &resID,
				Items:                s.Payload.Items,
				TotalKopecks:         s.Payload.TotalKopecks,
				DeliveryKopecks:      s.Payload.DeliveryKopecks,
			})
			if err != nil {
				return err
			}
			orderID = order.ID
			return nil
		}); err != nil {
			o.publishReply(ctx, s.ID, 2, "failed", err)
			return uuid.Nil, o.fail(ctx, s.ID, completed, "step 2 CreatePending", err, input, &s.Payload)
		}
		o.publishReply(ctx, s.ID, 2, "ok", nil)
		completed = 2
		_ = o.repo.UpdateState(ctx, s.ID, 2, domain.SagaStatusPending, func(p *domain.SagaPayload) {
			p.OrderID = &orderID
		})
		s.Payload.OrderID = &orderID // keep in-memory in sync
	}

	if s.Payload.OrderID == nil {
		return uuid.Nil, fmt.Errorf("invariant violated: saga %s has no OrderID after step 2", s.ID)
	}
	orderID := *s.Payload.OrderID

	// Step 3 — ClearCart(core)
	if completed < 3 {
		o.publishCmd(ctx, s.ID, 3, "ClearCart", nil)
		if err := o.runStep(ctx, func(c context.Context) error {
			return o.carts.ClearCart(c, input.UserID)
		}); err != nil {
			o.publishReply(ctx, s.ID, 3, "failed", err)
			return uuid.Nil, o.fail(ctx, s.ID, completed, "step 3 ClearCart", err, input, &s.Payload)
		}
		o.publishReply(ctx, s.ID, 3, "ok", nil)
		completed = 3
		_ = o.repo.UpdateState(ctx, s.ID, 3, domain.SagaStatusPending, nil)
	}

	// Step 4 — Confirm(ordering)
	if completed < 4 {
		o.publishCmd(ctx, s.ID, 4, "Confirm", nil)
		if err := o.runStep(ctx, func(c context.Context) error {
			err := o.orders.Confirm(c, orderID)
			// ErrConflict means already confirmed — idempotent success.
			if errors.Is(err, sherrors.ErrConflict) {
				return nil
			}
			return err
		}); err != nil {
			o.publishReply(ctx, s.ID, 4, "failed", err)
			return uuid.Nil, o.fail(ctx, s.ID, completed, "step 4 Confirm", err, input, &s.Payload)
		}
		o.publishReply(ctx, s.ID, 4, "ok", nil)
		_ = o.repo.UpdateState(ctx, s.ID, 4, domain.SagaStatusCompleted, nil)
	}

	return orderID, nil
}

// fail records the failure reason, starts compensations, and returns the final error.
func (o *Orchestrator) fail(
	ctx context.Context,
	sagaID uuid.UUID,
	completedStep int8,
	failedAt string,
	stepErr error,
	input domain.StartInput,
	payload *domain.SagaPayload,
) error {
	reason := fmt.Sprintf("%s: %v", failedAt, stepErr)
	_ = o.repo.UpdateState(ctx, sagaID, completedStep, domain.SagaStatusCompensating, func(p *domain.SagaPayload) {
		p.FailureReason = reason
	})

	compErr := o.compensate(sagaID, completedStep, input, payload)
	_ = o.repo.UpdateState(context.Background(), sagaID, 0, domain.SagaStatusFailed, nil)

	if compErr != nil {
		return fmt.Errorf("%w: saga failed at %s and compensation failed: %v (original: %v)",
			domain.ErrManualIntervention, failedAt, compErr, stepErr)
	}
	return fmt.Errorf("saga failed at %s (compensated): %w", failedAt, stepErr)
}

// compensate runs compensations for completed steps in reverse order.
func (o *Orchestrator) compensate(
	sagaID uuid.UUID,
	completedStep int8,
	input domain.StartInput,
	payload *domain.SagaPayload,
) error {
	type comp struct {
		step int8
		name string
		fn   func(context.Context) error
	}

	var orderID uuid.UUID
	if payload.OrderID != nil {
		orderID = *payload.OrderID
	}

	// Full compensation table, in reverse execution order (step 3 → 2 → 1).
	allComps := []comp{
		{3, "RestoreCart", func(ctx context.Context) error {
			return o.carts.RestoreCart(ctx, input.UserID, payload.Items)
		}},
		{2, "Cancel", func(ctx context.Context) error {
			if orderID == (uuid.UUID{}) {
				return nil
			}
			err := o.orders.Cancel(ctx, orderID, "saga compensation")
			if errors.Is(err, sherrors.ErrNotFound) {
				return nil // already gone — idempotent
			}
			return err
		}},
		{1, "UnlockResult", func(ctx context.Context) error {
			return o.optimization.UnlockResult(ctx, input.OptimizationResultID)
		}},
	}

	for _, c := range allComps {
		if c.step > completedStep {
			continue // skip steps that were never executed
		}
		if err := o.runCompensation(sagaID, c.step, c.name, c.fn); err != nil {
			return err
		}
	}
	return nil
}

// ─── Step helpers ─────────────────────────────────────────────────────────────

// runStep executes fn within the step timeout, retrying on ErrTransient.
func (o *Orchestrator) runStep(ctx context.Context, fn func(context.Context) error) error {
	stepCtx, cancel := context.WithTimeout(ctx, o.stepTimeout)
	defer cancel()

	backoff := 100 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < o.maxStepAttempts; attempt++ {
		lastErr = fn(stepCtx)
		if lastErr == nil {
			return nil
		}
		if !errors.Is(lastErr, domain.ErrTransient) {
			return lastErr // fail-fast for non-transient errors
		}
		if attempt < o.maxStepAttempts-1 {
			select {
			case <-stepCtx.Done():
				return stepCtx.Err()
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, 5*time.Second)
		}
	}
	return lastErr
}

// runCompensation executes fn with its own background context and retry loop.
// NotFound is treated as success (idempotent). Permanent failure returns an error.
func (o *Orchestrator) runCompensation(sagaID uuid.UUID, step int8, name string, fn func(context.Context) error) error {
	compCtx, cancel := context.WithTimeout(context.Background(), compensationCtxTimeout)
	defer cancel()

	backoff := 100 * time.Millisecond
	for attempt := 0; attempt < o.maxCompAttempts; attempt++ {
		err := fn(compCtx)
		if err == nil {
			o.log.Info("saga compensation succeeded",
				"saga_id", sagaID, "step", step, "name", name)
			return nil
		}
		if errors.Is(err, domain.ErrNotFound) {
			return nil // already compensated — idempotent
		}
		o.log.Warn("saga compensation attempt failed",
			"saga_id", sagaID, "step", step, "name", name,
			"attempt", attempt+1, "error", err)
		if attempt < o.maxCompAttempts-1 {
			select {
			case <-compCtx.Done():
				return fmt.Errorf("compensation %s context expired", name)
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, 30*time.Second)
		}
	}
	o.log.Error("CRITICAL: saga compensation permanently failed — manual intervention required",
		"saga_id", sagaID, "step", step, "name", name)
	return fmt.Errorf("compensation %s for saga %s exceeded %d attempts", name, sagaID, o.maxCompAttempts)
}

// ─── Audit helpers (best-effort; errors are logged and swallowed) ─────────────

func (o *Orchestrator) publishCmd(ctx context.Context, sagaID uuid.UUID, step int8, name string, payload any) {
	if err := o.audit.PublishCommand(ctx, sagaID, step, name, payload); err != nil {
		o.log.WarnContext(ctx, "audit command publish failed",
			"saga_id", sagaID, "step", step, "name", name, "error", err)
	}
}

func (o *Orchestrator) publishReply(ctx context.Context, sagaID uuid.UUID, step int8, status string, errVal error) {
	var payload any
	if errVal != nil {
		payload = map[string]any{"error": errVal.Error()}
	}
	if err := o.audit.PublishReply(ctx, sagaID, step, status, payload); err != nil {
		o.log.WarnContext(ctx, "audit reply publish failed",
			"saga_id", sagaID, "step", step, "status", status, "error", err)
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
