package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the full runtime configuration for ordering-service.
type Config struct {
	Env         string
	Server      ServerConfig
	GRPC        GRPCServerConfig
	DB          DatabaseConfig
	Redis       RedisConfig
	Kafka       KafkaConfig
	JWT         JWTConfig
	GRPCClients GRPCClientConfig
	Saga        SagaConfig
}

type ServerConfig struct {
	Port int
}

type GRPCServerConfig struct {
	Port int
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL string
}

type KafkaConfig struct {
	Brokers []string
}

type JWTConfig struct {
	Secret string
}

// GRPCClientConfig holds addresses of upstream gRPC services.
type GRPCClientConfig struct {
	CoreAddr         string
	OptimizationAddr string
}

// SagaConfig holds saga orchestration tuning parameters.
type SagaConfig struct {
	StepTimeout             time.Duration
	MaxCompensationAttempts int
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	env := getEnv("ENV", "development")

	serverPort := getEnvInt("SERVER_PORT", 8083)
	grpcPort := getEnvInt("GRPC_PORT", 9093)

	if serverPort < 1 || serverPort > 65535 {
		return nil, fmt.Errorf("SERVER_PORT %d out of range [1, 65535]", serverPort)
	}
	if grpcPort < 1 || grpcPort > 65535 {
		return nil, fmt.Errorf("GRPC_PORT %d out of range [1, 65535]", grpcPort)
	}

	dbURL := getEnv("DB_URL", "postgres://postgres:postgres@localhost:5432/ordering_db?sslmode=disable")

	kafkaBrokers := getEnvStrings("KAFKA_BROKERS", []string{"localhost:9092"})
	if len(kafkaBrokers) == 0 {
		return nil, fmt.Errorf("KAFKA_BROKERS must not be empty")
	}

	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-in-prod")
	if env == "production" && (jwtSecret == "dev-secret-change-in-prod" || os.Getenv("JWT_SECRET") == "") {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	cfg := &Config{
		Env: env,
		Server: ServerConfig{
			Port: serverPort,
		},
		GRPC: GRPCServerConfig{
			Port: grpcPort,
		},
		DB: DatabaseConfig{
			URL:             dbURL,
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		Kafka: KafkaConfig{
			Brokers: kafkaBrokers,
		},
		JWT: JWTConfig{
			Secret: jwtSecret,
		},
		GRPCClients: GRPCClientConfig{
			CoreAddr:         getEnv("CORE_GRPC_ADDR", "core-service:9091"),
			OptimizationAddr: getEnv("OPTIMIZATION_GRPC_ADDR", "optimization-service:9092"),
		},
		Saga: SagaConfig{
			StepTimeout:             getEnvDuration("SAGA_STEP_TIMEOUT", 30*time.Second),
			MaxCompensationAttempts: getEnvInt("SAGA_MAX_COMPENSATION_ATTEMPTS", 5),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvStrings(key string, defaultValue []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}
