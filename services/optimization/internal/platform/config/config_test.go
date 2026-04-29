package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "development", cfg.Env)
	assert.Equal(t, 8082, cfg.Server.Port)
	assert.Equal(t, 9092, cfg.GRPC.Port)
	assert.Equal(t, "core-service:9091", cfg.GRPCClients.CoreAddr)
	assert.Equal(t, "ml-service:50051", cfg.GRPCClients.MLAddr)
	assert.Equal(t, 30*time.Second, cfg.OptimizationTimeout)
	assert.Equal(t, 30*time.Minute, cfg.ResultTTL)
}

func TestLoad_DBURLRequired(t *testing.T) {
	t.Setenv("DB_URL", "")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_CoreGRPCAddrOverride(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("CORE_GRPC_ADDR", "localhost:9091")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "localhost:9091", cfg.GRPCClients.CoreAddr)
}

func TestLoad_MLGRPCAddrOverride(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("ML_GRPC_ADDR", "localhost:50051")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "localhost:50051", cfg.GRPCClients.MLAddr)
}

func TestLoad_OptimizationTimeoutOverride(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("OPTIMIZATION_TIMEOUT", "45s")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 45*time.Second, cfg.OptimizationTimeout)
}

func TestLoad_ResultTTLOverride(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("RESULT_TTL", "1h")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, time.Hour, cfg.ResultTTL)
}

func TestLoad_ProductionRequiresJWTSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("JWT_SECRET", "")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_InvalidServerPort(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("SERVER_PORT", "99999")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_InvalidGRPCPort(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/optimization_db?sslmode=disable")
	t.Setenv("GRPC_PORT", "0")
	_, err := Load()
	assert.Error(t, err)
}
