package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"golang.org/x/sync/errgroup"

	"github.com/foodsea/core/internal/modules/notifications/apns"
	notifinternal "github.com/foodsea/core/internal/modules/notifications/internal"
	"github.com/foodsea/core/internal/modules/notifications/repository"
	kafkaplatform "github.com/foodsea/core/internal/platform/kafka"
)

const (
	liveActivityEndDismissAfter = 30 * time.Second
)

var defaultBackoffSchedule = []time.Duration{time.Second, 4 * time.Second, 16 * time.Second}

type pushClient interface {
	Push(env string, notification *apns2.Notification) (apns.Outcome, error)
}

type store interface {
	GetUserDevice(ctx context.Context, userID uuid.UUID) (*repository.UserDevice, error)
	GetOrderLiveActivity(ctx context.Context, orderID uuid.UUID) (*repository.OrderLiveActivity, error)
	DeleteDevices(ctx context.Context, userID uuid.UUID) error
	DeleteLiveActivity(ctx context.Context, userID, orderID uuid.UUID) error
}

type orderEventPayload struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	Status     string    `json:"status"`
	OccurredAt time.Time `json:"occurred_at"`
}

type parsedOrderEvent struct {
	Envelope   kafkaplatform.Event
	OrderID    uuid.UUID
	UserID     uuid.UUID
	Status     notifinternal.OrderStatus
	OccurredAt time.Time
}

type Handler struct {
	log *slog.Logger

	store store
	apns  pushClient

	sleep           func(context.Context, time.Duration) error
	backoffSchedule []time.Duration
	maxAttempts     int
	redeliveryLimit int

	mu              sync.Mutex
	redeliveryCount map[string]int
}

func NewHandler(log *slog.Logger, store store, apns pushClient) *Handler {
	return &Handler{
		log:             log,
		store:           store,
		apns:            apns,
		sleep:           sleepContext,
		backoffSchedule: append([]time.Duration(nil), defaultBackoffSchedule...),
		maxAttempts:     3,
		redeliveryLimit: 3,
		redeliveryCount: make(map[string]int),
	}
}

func (h *Handler) Handle(ctx context.Context, event kafkaplatform.Event) error {
	parsed, err := parseOrderEvent(event)
	if err != nil {
		h.log.WarnContext(ctx, "notifications: skip invalid order.events payload",
			"error", err,
			"event_id", event.EventID,
			"event_type", event.EventType,
		)
		return nil
	}

	device, err := h.store.GetUserDevice(ctx, parsed.UserID)
	if err != nil {
		return fmt.Errorf("loading user device: %w", err)
	}
	if device == nil {
		return nil
	}

	liveActivity, err := h.store.GetOrderLiveActivity(ctx, parsed.OrderID)
	if err != nil {
		return fmt.Errorf("loading order live activity: %w", err)
	}

	if err = h.dispatchPushes(ctx, parsed, device, liveActivity); err != nil {
		if shouldCommitAsPoison := h.registerFailure(parsed.Envelope.EventID); shouldCommitAsPoison {
			h.log.ErrorContext(ctx, "notifications: poison pill reached redelivery limit, committing",
				"event_id", parsed.Envelope.EventID,
				"event_type", parsed.Envelope.EventType,
				"order_id", parsed.OrderID,
				"user_id", parsed.UserID,
				"error", err,
			)
			h.clearFailure(parsed.Envelope.EventID)
			return nil
		}
		return err
	}

	h.clearFailure(parsed.Envelope.EventID)
	return nil
}

func (h *Handler) dispatchPushes(
	ctx context.Context,
	event parsedOrderEvent,
	device *repository.UserDevice,
	liveActivity *repository.OrderLiveActivity,
) error {
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		alertPayload, err := notifinternal.BuildAlertPayload(event.OrderID.String(), event.Status)
		if err != nil {
			return fmt.Errorf("build alert payload: %w", err)
		}

		notification := apns.NewAlertNotification(device.APNSToken, device.BundleID, alertPayload)
		notification.ApnsID = deterministicAPNSID(event.Envelope.EventID, device.APNSToken)

		outcome, sendErr := h.sendWithRetry(groupCtx, device.Environment, notification)
		if sendErr != nil {
			return fmt.Errorf("send alert push: %w", sendErr)
		}

		if outcome.Class == apns.OutcomeClassInvalidToken {
			if err = h.store.DeleteDevices(groupCtx, device.UserID); err != nil {
				h.log.WarnContext(groupCtx, "notifications: failed to delete invalid alert token",
					"user_id", device.UserID,
					"error", err,
				)
			}
		}

		return nil
	})

	if liveActivity != nil {
		group.Go(func() error {
			payload, shouldDeleteAfterSuccess, err := buildLiveActivityPayload(event)
			if err != nil {
				return fmt.Errorf("build live activity payload: %w", err)
			}

			notification := apns.NewLiveActivityNotification(liveActivity.PushToken, liveActivity.BundleID, payload)
			notification.ApnsID = deterministicAPNSID(event.Envelope.EventID, liveActivity.PushToken)

			outcome, sendErr := h.sendWithRetry(groupCtx, liveActivity.Environment, notification)
			if sendErr != nil {
				return fmt.Errorf("send liveactivity push: %w", sendErr)
			}

			if outcome.Class == apns.OutcomeClassInvalidToken || shouldDeleteAfterSuccess {
				if err = h.store.DeleteLiveActivity(groupCtx, liveActivity.UserID, liveActivity.OrderID); err != nil {
					h.log.WarnContext(groupCtx, "notifications: failed to delete live activity binding",
						"order_id", liveActivity.OrderID,
						"user_id", liveActivity.UserID,
						"error", err,
					)
				}
			}

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	return nil
}

func buildLiveActivityPayload(event parsedOrderEvent) ([]byte, bool, error) {
	if isTerminalStatus(event.Status) {
		dismissAt := event.OccurredAt.Add(liveActivityEndDismissAfter)
		payload, err := notifinternal.BuildLiveActivityEndPayload(
			event.OrderID.String(),
			event.Status,
			event.OccurredAt,
			dismissAt,
		)
		if err != nil {
			return nil, false, err
		}
		return payload, true, nil
	}

	staleAt := event.OccurredAt.Add(2 * time.Hour)
	payload, err := notifinternal.BuildLiveActivityUpdatePayload(
		event.OrderID.String(),
		event.Status,
		event.OccurredAt,
		staleAt,
	)
	if err != nil {
		return nil, false, err
	}

	return payload, false, nil
}

func (h *Handler) sendWithRetry(ctx context.Context, env string, notification *apns2.Notification) (apns.Outcome, error) {
	var lastOutcome apns.Outcome
	var lastErr error

	for attempt := 1; attempt <= h.maxAttempts; attempt++ {
		outcome, err := h.apns.Push(env, notification)
		lastOutcome = outcome
		lastErr = err

		if err == nil && outcome.Class != apns.OutcomeClassTransient {
			return outcome, nil
		}

		if err == nil && outcome.Class == apns.OutcomeClassTransient {
			lastErr = fmt.Errorf("apns transient response: status=%d reason=%s", outcome.StatusCode, outcome.Reason)
		}

		if attempt == h.maxAttempts {
			break
		}

		delay := backoffAt(h.backoffSchedule, attempt-1)
		if sleepErr := h.sleep(ctx, delay); sleepErr != nil {
			return apns.Outcome{}, sleepErr
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("push failed after retries")
	}

	return lastOutcome, lastErr
}

func parseOrderEvent(event kafkaplatform.Event) (parsedOrderEvent, error) {
	var payload orderEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return parsedOrderEvent{}, fmt.Errorf("unmarshal payload: %w", err)
	}

	orderID, err := uuid.Parse(payload.OrderID)
	if err != nil {
		return parsedOrderEvent{}, fmt.Errorf("invalid order_id: %w", err)
	}
	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return parsedOrderEvent{}, fmt.Errorf("invalid user_id: %w", err)
	}

	status, err := parseStatus(payload.Status)
	if err != nil {
		return parsedOrderEvent{}, err
	}

	occurredAt := payload.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = event.Timestamp
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	return parsedOrderEvent{
		Envelope:   event,
		OrderID:    orderID,
		UserID:     userID,
		Status:     status,
		OccurredAt: occurredAt.UTC(),
	}, nil
}

func parseStatus(status string) (notifinternal.OrderStatus, error) {
	switch notifinternal.OrderStatus(strings.ToLower(strings.TrimSpace(status))) {
	case notifinternal.StatusCreated:
		return notifinternal.StatusCreated, nil
	case notifinternal.StatusConfirmed:
		return notifinternal.StatusConfirmed, nil
	case notifinternal.StatusInDelivery:
		return notifinternal.StatusInDelivery, nil
	case notifinternal.StatusDelivered:
		return notifinternal.StatusDelivered, nil
	case notifinternal.StatusCancelled:
		return notifinternal.StatusCancelled, nil
	default:
		return "", fmt.Errorf("unknown status %q", status)
	}
}

func deterministicAPNSID(eventID, token string) string {
	sum := sha256.Sum256([]byte(eventID + ":" + token))
	buf := sum[:16]
	buf[6] = (buf[6] & 0x0f) | 0x50
	buf[8] = (buf[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16],
	)
}

func backoffAt(schedule []time.Duration, idx int) time.Duration {
	if idx < len(schedule) {
		return schedule[idx]
	}
	if len(schedule) == 0 {
		return 0
	}
	return schedule[len(schedule)-1]
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isTerminalStatus(status notifinternal.OrderStatus) bool {
	return status == notifinternal.StatusDelivered || status == notifinternal.StatusCancelled
}

func (h *Handler) registerFailure(eventID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.redeliveryCount[eventID]++
	return h.redeliveryCount[eventID] > h.redeliveryLimit
}

func (h *Handler) clearFailure(eventID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.redeliveryCount, eventID)
}

// Runner bridges the domain handler with the shared Kafka consumer.
type Runner struct {
	consumer *kafkaplatform.Consumer
	handler  *Handler
}

func NewRunner(consumer *kafkaplatform.Consumer, handler *Handler) *Runner {
	return &Runner{consumer: consumer, handler: handler}
}

func (r *Runner) Run(ctx context.Context) error {
	return r.consumer.Run(ctx, r.handler.Handle)
}

func (r *Runner) Close() error {
	return r.consumer.Close()
}
