package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("ENV", "development")
	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "development", cfg.Env)
	assert.Equal(t, 8083, cfg.Server.Port)
	assert.Equal(t, 9093, cfg.GRPC.Port)
	assert.Equal(t, "core-service:9091", cfg.GRPCClients.CoreAddr)
	assert.Equal(t, "optimization-service:9092", cfg.GRPCClients.OptimizationAddr)
	assert.Equal(t, 30*time.Second, cfg.Saga.StepTimeout)
	assert.Equal(t, 5, cfg.Saga.MaxCompensationAttempts)
}

func TestLoad_CoreGRPCAddrOverride(t *testing.T) {
	t.Setenv("CORE_GRPC_ADDR", "localhost:9091")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "localhost:9091", cfg.GRPCClients.CoreAddr)
}

func TestLoad_OptimizationGRPCAddrOverride(t *testing.T) {
	t.Setenv("OPTIMIZATION_GRPC_ADDR", "localhost:9092")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "localhost:9092", cfg.GRPCClients.OptimizationAddr)
}

func TestLoad_SagaStepTimeoutOverride(t *testing.T) {
	t.Setenv("SAGA_STEP_TIMEOUT", "60s")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, cfg.Saga.StepTimeout)
}

func TestLoad_ProductionRequiresJWTSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_ProductionWithJWTSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "super-secret-key-for-prod")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "super-secret-key-for-prod", cfg.JWT.Secret)
}

func TestLoad_InvalidServerPort(t *testing.T) {
	t.Setenv("SERVER_PORT", "99999")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_InvalidGRPCPort(t *testing.T) {
	t.Setenv("GRPC_PORT", "0")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_KafkaBrokersOverride(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "broker1:9092,broker2:9092")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, cfg.Kafka.Brokers)
}
