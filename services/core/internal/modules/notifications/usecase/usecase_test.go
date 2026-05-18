package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/notifications/usecase"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) UpsertDevice(ctx context.Context, in usecase.DeviceRegistration) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *mockRepo) DeleteDevices(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockRepo) UpsertLiveActivity(ctx context.Context, in usecase.LiveActivityRegistration) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *mockRepo) DeleteLiveActivity(ctx context.Context, userID, orderID uuid.UUID) error {
	args := m.Called(ctx, userID, orderID)
	return args.Error(0)
}

func TestRegisterDevice_Execute(t *testing.T) {
	repo := &mockRepo{}
	uc := usecase.NewRegisterDevice(repo)
	in := usecase.DeviceRegistration{
		UserID:      uuid.New(),
		APNSToken:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		BundleID:    "com.foodsea.app",
		Environment: "sandbox",
	}

	repo.On("UpsertDevice", mock.Anything, in).Return(nil).Once()

	err := uc.Execute(context.Background(), in)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRemoveDevices_Execute_Error(t *testing.T) {
	repo := &mockRepo{}
	uc := usecase.NewRemoveDevices(repo)
	userID := uuid.New()
	expectedErr := errors.New("db down")

	repo.On("DeleteDevices", mock.Anything, userID).Return(expectedErr).Once()

	err := uc.Execute(context.Background(), userID)
	assert.ErrorIs(t, err, expectedErr)
	repo.AssertExpectations(t)
}

func TestRegisterLiveActivity_Execute(t *testing.T) {
	repo := &mockRepo{}
	uc := usecase.NewRegisterLiveActivity(repo)
	in := usecase.LiveActivityRegistration{
		UserID:      uuid.New(),
		OrderID:     uuid.New(),
		PushToken:   "deadbeef",
		BundleID:    "com.foodsea.app",
		Environment: "production",
	}

	repo.On("UpsertLiveActivity", mock.Anything, in).Return(nil).Once()

	err := uc.Execute(context.Background(), in)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRemoveLiveActivity_Execute(t *testing.T) {
	repo := &mockRepo{}
	uc := usecase.NewRemoveLiveActivity(repo)
	userID := uuid.New()
	orderID := uuid.New()

	repo.On("DeleteLiveActivity", mock.Anything, userID, orderID).Return(nil).Once()

	err := uc.Execute(context.Background(), userID, orderID)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
