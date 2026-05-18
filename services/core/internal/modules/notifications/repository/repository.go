package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/ent/orderliveactivity"
	"github.com/foodsea/core/ent/userdevice"
	"github.com/foodsea/core/internal/modules/notifications/usecase"
)

// Repository stores notifications data using ent client.
type Repository struct {
	client *ent.Client
}

type UserDevice struct {
	UserID      uuid.UUID
	APNSToken   string
	BundleID    string
	Environment string
}

type OrderLiveActivity struct {
	OrderID     uuid.UUID
	UserID      uuid.UUID
	PushToken   string
	BundleID    string
	Environment string
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) UpsertDevice(ctx context.Context, in usecase.DeviceRegistration) error {
	env := userdevice.Environment(in.Environment)

	err := r.client.UserDevice.UpdateOneID(in.UserID).
		SetApnsToken(in.APNSToken).
		SetBundleID(in.BundleID).
		SetEnvironment(env).
		SetNillableAppVersion(in.AppVersion).
		Exec(ctx)
	if err == nil {
		return nil
	}
	if !ent.IsNotFound(err) {
		return fmt.Errorf("updating notification device: %w", err)
	}

	err = r.client.UserDevice.Create().
		SetID(in.UserID).
		SetUserID(in.UserID).
		SetApnsToken(in.APNSToken).
		SetBundleID(in.BundleID).
		SetEnvironment(env).
		SetNillableAppVersion(in.AppVersion).
		Exec(ctx)
	if err == nil {
		return nil
	}

	if ent.IsConstraintError(err) {
		if updErr := r.client.UserDevice.UpdateOneID(in.UserID).
			SetApnsToken(in.APNSToken).
			SetBundleID(in.BundleID).
			SetEnvironment(env).
			SetNillableAppVersion(in.AppVersion).
			Exec(ctx); updErr == nil {
			return nil
		} else {
			return fmt.Errorf("upserting notification device after conflict: %w", updErr)
		}
	}

	return fmt.Errorf("creating notification device: %w", err)
}

func (r *Repository) DeleteDevices(ctx context.Context, userID uuid.UUID) error {
	err := r.client.UserDevice.DeleteOneID(userID).Exec(ctx)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deleting notification device: %w", err)
	}
	return nil
}

func (r *Repository) UpsertLiveActivity(ctx context.Context, in usecase.LiveActivityRegistration) error {
	env := orderliveactivity.Environment(in.Environment)

	err := r.client.OrderLiveActivity.UpdateOneID(in.OrderID).
		Where(orderliveactivity.UserID(in.UserID)).
		SetPushToken(in.PushToken).
		SetBundleID(in.BundleID).
		SetEnvironment(env).
		Exec(ctx)
	if err == nil {
		return nil
	}
	if !ent.IsNotFound(err) {
		return fmt.Errorf("updating live activity token: %w", err)
	}

	err = r.client.OrderLiveActivity.Create().
		SetID(in.OrderID).
		SetUserID(in.UserID).
		SetPushToken(in.PushToken).
		SetBundleID(in.BundleID).
		SetEnvironment(env).
		Exec(ctx)
	if err == nil {
		return nil
	}

	if ent.IsConstraintError(err) {
		if updErr := r.client.OrderLiveActivity.UpdateOneID(in.OrderID).
			Where(orderliveactivity.UserID(in.UserID)).
			SetPushToken(in.PushToken).
			SetBundleID(in.BundleID).
			SetEnvironment(env).
			Exec(ctx); updErr == nil {
			return nil
		} else {
			return fmt.Errorf("upserting live activity token after conflict: %w", updErr)
		}
	}

	return fmt.Errorf("creating live activity token: %w", err)
}

func (r *Repository) DeleteLiveActivity(ctx context.Context, userID, orderID uuid.UUID) error {
	err := r.client.OrderLiveActivity.DeleteOneID(orderID).
		Where(orderliveactivity.UserID(userID)).
		Exec(ctx)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deleting live activity token: %w", err)
	}
	return nil
}

func (r *Repository) GetUserDevice(ctx context.Context, userID uuid.UUID) (*UserDevice, error) {
	row, err := r.client.UserDevice.Get(ctx, userID)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying notification device: %w", err)
	}

	return &UserDevice{
		UserID:      row.ID,
		APNSToken:   row.ApnsToken,
		BundleID:    row.BundleID,
		Environment: row.Environment.String(),
	}, nil
}

func (r *Repository) GetOrderLiveActivity(ctx context.Context, orderID uuid.UUID) (*OrderLiveActivity, error) {
	row, err := r.client.OrderLiveActivity.Get(ctx, orderID)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying order live activity: %w", err)
	}

	return &OrderLiveActivity{
		OrderID:     row.ID,
		UserID:      row.UserID,
		PushToken:   row.PushToken,
		BundleID:    row.BundleID,
		Environment: row.Environment.String(),
	}, nil
}
