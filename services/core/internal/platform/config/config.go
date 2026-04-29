package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the full runtime configuration for core-service.
type Config struct {
	Env    string
	Server ServerConfig
	GRPC   GRPCConfig
	DB     DatabaseConfig
	Redis  RedisConfig
	Kafka  KafkaConfig
	JWT    JWTConfig
	S3     S3Config
}

type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
	PublicBaseURL   string
}

type ServerConfig struct {
	Port int
}

type GRPCConfig struct {
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
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	env := getEnv("ENV", "development")

	serverPort := getEnvInt("SERVER_PORT", 8081)
	grpcPort := getEnvInt("GRPC_PORT", 9091)

	if serverPort < 1 || serverPort > 65535 {
		return nil, fmt.Errorf("SERVER_PORT %d out of range [1, 65535]", serverPort)
	}
	if grpcPort < 1 || grpcPort > 65535 {
		return nil, fmt.Errorf("GRPC_PORT %d out of range [1, 65535]", grpcPort)
	}

	accessTTL := getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute)
	refreshTTL := getEnvDuration("JWT_REFRESH_TTL", 30*24*time.Hour)

	if accessTTL < time.Minute {
		return nil, fmt.Errorf("JWT_ACCESS_TTL must be >= 1m, got %s", accessTTL)
	}
	if refreshTTL < time.Minute {
		return nil, fmt.Errorf("JWT_REFRESH_TTL must be >= 1m, got %s", refreshTTL)
	}

	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-in-prod")
	if env == "production" && jwtSecret == "dev-secret-change-in-prod" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}
	if env == "production" && os.Getenv("JWT_SECRET") == "" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	cfg := &Config{
		Env: env,
		Server: ServerConfig{
			Port: serverPort,
		},
		GRPC: GRPCConfig{
			Port: grpcPort,
		},
		DB: DatabaseConfig{
			URL:             getEnv("DB_URL", "postgres://postgres:postgres@localhost:5433/core_db?sslmode=disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		Kafka: KafkaConfig{
			Brokers: getEnvStrings("KAFKA_BROKERS", []string{"localhost:9092"}),
		},
		JWT: JWTConfig{
			Secret:     jwtSecret,
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		S3: S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", "localhost:9000"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", "minioadmin"),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", "minioadmin"),
			BucketName:      getEnv("S3_BUCKET_NAME", "product-images"),
			UseSSL:          getEnvBool("S3_USE_SSL", false),
			PublicBaseURL:   getEnv("S3_PUBLIC_BASE_URL", "http://localhost:9000/product-images"),
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

func getEnvBool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
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

// getEnvBool and getEnvStrings are exported for testing via package-internal usage.
var _ = getEnvBool
