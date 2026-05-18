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
	Env           string
	Server        ServerConfig
	GRPC          GRPCConfig
	ML            MLConfig
	PhotoSearch   PhotoSearchConfig
	DB            DatabaseConfig
	Redis         RedisConfig
	Kafka         KafkaConfig
	Notifications NotificationsConfig
	JWT           JWTConfig
	S3            S3Config
	OAuth         OAuthConfig
	APNS          APNSConfig
}

type MLConfig struct {
	GRPCAddr string
}

type PhotoSearchConfig struct {
	MaxImageBytes int64
	Timeout       time.Duration
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

type NotificationsConfig struct {
	Kafka NotificationsKafkaConfig
}

type NotificationsKafkaConfig struct {
	Topic   string
	GroupID string
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type OAuthConfig struct {
	StateTTL                  time.Duration
	AllowedRedirectURIs       []string
	NativeAllowedRedirectURIs []string
	LegacyEnabled             bool
	NativeEnabled             bool
	Google                    OAuthProviderConfig
	GoogleNative              OAuthProviderConfig
	AppleNative               OAuthAppleConfig
	Yandex                    OAuthProviderConfig
	YandexNativeSDKEnabled    bool
}

type OAuthProviderConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
}

type OAuthAppleConfig struct {
	Enabled      bool
	ClientID     string
	JWKSURL      string
	JWKSCacheTTL time.Duration
	Issuer       string
}

type APNSConfig struct {
	Environment string
	BundleID    string
	TeamID      string
	KeyID       string
	PrivateKey  string
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
	photoSearchMaxBytes := int64(getEnvInt("PHOTO_SEARCH_MAX_IMAGE_BYTES", 8*1024*1024))
	if photoSearchMaxBytes <= 0 {
		return nil, fmt.Errorf("PHOTO_SEARCH_MAX_IMAGE_BYTES must be > 0")
	}
	photoSearchTimeout := getEnvDuration("PHOTO_SEARCH_TIMEOUT", 10*time.Second)
	if photoSearchTimeout <= 0 {
		return nil, fmt.Errorf("PHOTO_SEARCH_TIMEOUT must be > 0")
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
	apnsEnv := strings.ToLower(getEnv("APNS_ENV", ""))
	if apnsEnv == "" {
		if env == "production" {
			apnsEnv = "production"
		} else {
			apnsEnv = "sandbox"
		}
	}
	if apnsEnv != "sandbox" && apnsEnv != "production" {
		return nil, fmt.Errorf("APNS_ENV must be one of: sandbox, production")
	}

	cfg := &Config{
		Env: env,
		Server: ServerConfig{
			Port: serverPort,
		},
		GRPC: GRPCConfig{
			Port: grpcPort,
		},
		ML: MLConfig{
			GRPCAddr: getEnv("ML_GRPC_ADDR", "ml-service:50051"),
		},
		PhotoSearch: PhotoSearchConfig{
			MaxImageBytes: photoSearchMaxBytes,
			Timeout:       photoSearchTimeout,
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
		Notifications: NotificationsConfig{
			Kafka: NotificationsKafkaConfig{
				Topic:   getEnv("NOTIFICATIONS_KAFKA_TOPIC", "order.events"),
				GroupID: getEnv("NOTIFICATIONS_KAFKA_GROUP_ID", "core-notifications"),
			},
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
		OAuth: OAuthConfig{
			StateTTL:                  getEnvDuration("OAUTH_STATE_TTL", 10*time.Minute),
			AllowedRedirectURIs:       getEnvStrings("OAUTH_ALLOWED_REDIRECT_URIS", []string{}),
			NativeAllowedRedirectURIs: getEnvStrings("OAUTH_NATIVE_ALLOWED_REDIRECT_URIS", []string{}),
			LegacyEnabled:             getEnvBool("OAUTH_LEGACY_ENABLED", true),
			NativeEnabled:             getEnvBool("OAUTH_NATIVE_ENABLED", false),
			Google: oauthProviderConfig("GOOGLE", OAuthProviderConfig{
				AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:    "https://oauth2.googleapis.com/token",
				UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
				Scopes:      []string{"openid", "email", "profile"},
			}),
			GoogleNative: oauthPublicProviderConfig("GOOGLE_NATIVE", OAuthProviderConfig{
				AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:    "https://oauth2.googleapis.com/token",
				UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
				Scopes:      []string{"openid", "email", "profile"},
			}),
			AppleNative: oauthAppleConfig(OAuthAppleConfig{
				JWKSURL:      "https://appleid.apple.com/auth/keys",
				JWKSCacheTTL: time.Hour,
				Issuer:       "https://appleid.apple.com",
			}),
			Yandex: oauthProviderConfig("YANDEX", OAuthProviderConfig{
				AuthURL:     "https://oauth.yandex.ru/authorize",
				TokenURL:    "https://oauth.yandex.ru/token",
				UserInfoURL: "https://login.yandex.ru/info",
				Scopes:      []string{"login:email", "login:avatar"},
			}),
			YandexNativeSDKEnabled: getEnvBool("OAUTH_YANDEX_NATIVE_SDK_ENABLED", false),
		},
		APNS: APNSConfig{
			Environment: apnsEnv,
			BundleID:    getEnv("APNS_BUNDLE_ID", ""),
			TeamID:      getEnv("APNS_TEAM_ID", ""),
			KeyID:       getEnv("APNS_KEY_ID", ""),
			PrivateKey:  getEnv("APNS_PRIVATE_KEY", ""),
		},
	}

	if err := validateOAuthProvider("GOOGLE", cfg.OAuth.Google); err != nil {
		return nil, err
	}
	if err := validateOAuthProvider("YANDEX", cfg.OAuth.Yandex); err != nil {
		return nil, err
	}
	if err := validatePublicOAuthProvider("GOOGLE_NATIVE", cfg.OAuth.GoogleNative); err != nil {
		return nil, err
	}
	if err := validateAppleOAuthProvider(cfg.OAuth.AppleNative); err != nil {
		return nil, err
	}
	if env == "production" && cfg.OAuth.LegacyEnabled && (cfg.OAuth.Google.Enabled || cfg.OAuth.Yandex.Enabled) && len(cfg.OAuth.AllowedRedirectURIs) == 0 {
		return nil, fmt.Errorf("OAUTH_ALLOWED_REDIRECT_URIS must be set in production when OAuth is enabled")
	}
	if env == "production" && cfg.OAuth.NativeEnabled && (cfg.OAuth.GoogleNative.Enabled || cfg.OAuth.AppleNative.Enabled || cfg.OAuth.YandexNativeSDKEnabled) && len(cfg.OAuth.NativeAllowedRedirectURIs) == 0 {
		return nil, fmt.Errorf("OAUTH_NATIVE_ALLOWED_REDIRECT_URIS must be set in production when native OAuth is enabled")
	}

	return cfg, nil
}

func oauthProviderConfig(provider string, defaults OAuthProviderConfig) OAuthProviderConfig {
	clientID := getEnv("OAUTH_"+provider+"_CLIENT_ID", "")
	clientSecret := getEnv("OAUTH_"+provider+"_CLIENT_SECRET", "")

	cfg := defaults
	cfg.ClientID = clientID
	cfg.ClientSecret = clientSecret
	cfg.Enabled = clientID != "" && clientSecret != ""
	cfg.AuthURL = getEnv("OAUTH_"+provider+"_AUTH_URL", defaults.AuthURL)
	cfg.TokenURL = getEnv("OAUTH_"+provider+"_TOKEN_URL", defaults.TokenURL)
	cfg.UserInfoURL = getEnv("OAUTH_"+provider+"_USER_INFO_URL", defaults.UserInfoURL)
	cfg.Scopes = getEnvStrings("OAUTH_"+provider+"_SCOPES", defaults.Scopes)

	return cfg
}

func oauthPublicProviderConfig(provider string, defaults OAuthProviderConfig) OAuthProviderConfig {
	clientID := getEnv("OAUTH_"+provider+"_CLIENT_ID", "")
	clientSecret := getEnv("OAUTH_"+provider+"_CLIENT_SECRET", "")

	cfg := defaults
	cfg.ClientID = clientID
	cfg.ClientSecret = clientSecret
	cfg.Enabled = clientID != ""
	cfg.AuthURL = getEnv("OAUTH_"+provider+"_AUTH_URL", defaults.AuthURL)
	cfg.TokenURL = getEnv("OAUTH_"+provider+"_TOKEN_URL", defaults.TokenURL)
	cfg.UserInfoURL = getEnv("OAUTH_"+provider+"_USER_INFO_URL", defaults.UserInfoURL)
	cfg.Scopes = getEnvStrings("OAUTH_"+provider+"_SCOPES", defaults.Scopes)

	return cfg
}

func oauthAppleConfig(defaults OAuthAppleConfig) OAuthAppleConfig {
	clientID := getEnv("APPLE_CLIENT_ID", getEnv("OAUTH_APPLE_CLIENT_ID", ""))

	cfg := defaults
	cfg.ClientID = clientID
	cfg.JWKSURL = getEnv("APPLE_JWKS_URL", getEnv("OAUTH_APPLE_JWKS_URL", defaults.JWKSURL))
	cfg.JWKSCacheTTL = getEnvDuration("APPLE_JWKS_CACHE_TTL", getEnvDuration("OAUTH_APPLE_JWKS_CACHE_TTL", defaults.JWKSCacheTTL))
	cfg.Issuer = getEnv("APPLE_ISSUER", getEnv("OAUTH_APPLE_ISSUER", defaults.Issuer))

	if rawEnabled, ok := os.LookupEnv("APPLE_ENABLED"); ok {
		if enabled, err := strconv.ParseBool(rawEnabled); err == nil {
			cfg.Enabled = enabled
		} else {
			cfg.Enabled = false
		}
	} else {
		cfg.Enabled = clientID != ""
	}

	return cfg
}

func validateOAuthProvider(provider string, cfg OAuthProviderConfig) error {
	clientIDKey := "OAUTH_" + provider + "_CLIENT_ID"
	clientSecretKey := "OAUTH_" + provider + "_CLIENT_SECRET"

	hasClientID := cfg.ClientID != ""
	hasClientSecret := cfg.ClientSecret != ""
	if hasClientID != hasClientSecret {
		return fmt.Errorf("%s and %s must be set together", clientIDKey, clientSecretKey)
	}

	return nil
}

func validatePublicOAuthProvider(provider string, cfg OAuthProviderConfig) error {
	clientIDKey := "OAUTH_" + provider + "_CLIENT_ID"
	clientSecretKey := "OAUTH_" + provider + "_CLIENT_SECRET"

	hasClientID := cfg.ClientID != ""
	hasClientSecret := cfg.ClientSecret != ""
	if !hasClientID && hasClientSecret {
		return fmt.Errorf("%s must be set when %s is set", clientIDKey, clientSecretKey)
	}

	return nil
}

func validateAppleOAuthProvider(cfg OAuthAppleConfig) error {
	if cfg.Enabled && strings.TrimSpace(cfg.ClientID) == "" {
		return fmt.Errorf("APPLE_CLIENT_ID or OAUTH_APPLE_CLIENT_ID must be set when APPLE_ENABLED=true")
	}
	return nil
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
