package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/platform/config"
)

func setenv(t *testing.T, key, value string) {
	t.Helper()
	prev := os.Getenv(key)
	os.Setenv(key, value)
	t.Cleanup(func() { os.Setenv(key, prev) })
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	prev := os.Getenv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if prev != "" {
			os.Setenv(key, prev)
		}
	})
}

func TestLoad_DevDefaults(t *testing.T) {
	unsetenv(t, "ENV")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "development", cfg.Env)
	assert.Equal(t, 8081, cfg.Server.Port)
	assert.Equal(t, 9091, cfg.GRPC.Port)
	assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.JWT.RefreshTTL)
}

func TestLoad_ProdWithoutJWTSecret(t *testing.T) {
	setenv(t, "ENV", "production")
	unsetenv(t, "JWT_SECRET")
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_ProdWithJWTSecret(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.Env)
	assert.Equal(t, "supersecret", cfg.JWT.Secret)
}

func TestLoad_CustomPort(t *testing.T) {
	setenv(t, "SERVER_PORT", "9000")
	setenv(t, "GRPC_PORT", "9001")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, 9001, cfg.GRPC.Port)
}

func TestLoad_InvalidPort(t *testing.T) {
	setenv(t, "SERVER_PORT", "0")
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_PORT")
}

func TestLoad_LargePort(t *testing.T) {
	setenv(t, "SERVER_PORT", "99999")
	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_CustomTTLs(t *testing.T) {
	setenv(t, "JWT_ACCESS_TTL", "30m")
	setenv(t, "JWT_REFRESH_TTL", "720h")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 720*time.Hour, cfg.JWT.RefreshTTL)
}

func TestLoad_KafkaBrokers(t *testing.T) {
	setenv(t, "KAFKA_BROKERS", "broker1:9092,broker2:9092")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, cfg.Kafka.Brokers)
}

func TestLoad_NotificationsKafkaConfig(t *testing.T) {
	unsetenv(t, "NOTIFICATIONS_KAFKA_TOPIC")
	unsetenv(t, "NOTIFICATIONS_KAFKA_GROUP_ID")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "order.events", cfg.Notifications.Kafka.Topic)
	assert.Equal(t, "core-notifications", cfg.Notifications.Kafka.GroupID)

	setenv(t, "NOTIFICATIONS_KAFKA_TOPIC", "custom.order.events")
	setenv(t, "NOTIFICATIONS_KAFKA_GROUP_ID", "custom-core-notifications")
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, "custom.order.events", cfg.Notifications.Kafka.Topic)
	assert.Equal(t, "custom-core-notifications", cfg.Notifications.Kafka.GroupID)
}

func TestLoad_OAuthDefaults(t *testing.T) {
	unsetenv(t, "ENV")
	unsetenv(t, "OAUTH_STATE_TTL")
	unsetenv(t, "OAUTH_ALLOWED_REDIRECT_URIS")
	unsetenv(t, "OAUTH_GOOGLE_CLIENT_ID")
	unsetenv(t, "OAUTH_GOOGLE_CLIENT_SECRET")
	unsetenv(t, "OAUTH_GOOGLE_AUTH_URL")
	unsetenv(t, "OAUTH_GOOGLE_TOKEN_URL")
	unsetenv(t, "OAUTH_GOOGLE_USER_INFO_URL")
	unsetenv(t, "OAUTH_GOOGLE_SCOPES")
	unsetenv(t, "OAUTH_YANDEX_CLIENT_ID")
	unsetenv(t, "OAUTH_YANDEX_CLIENT_SECRET")
	unsetenv(t, "OAUTH_YANDEX_AUTH_URL")
	unsetenv(t, "OAUTH_YANDEX_TOKEN_URL")
	unsetenv(t, "OAUTH_YANDEX_USER_INFO_URL")
	unsetenv(t, "OAUTH_YANDEX_SCOPES")
	unsetenv(t, "OAUTH_APPLE_CLIENT_ID")
	unsetenv(t, "OAUTH_APPLE_JWKS_URL")
	unsetenv(t, "OAUTH_APPLE_JWKS_CACHE_TTL")
	unsetenv(t, "OAUTH_APPLE_ISSUER")
	unsetenv(t, "APPLE_ENABLED")
	unsetenv(t, "APPLE_CLIENT_ID")
	unsetenv(t, "APPLE_JWKS_URL")
	unsetenv(t, "APPLE_JWKS_CACHE_TTL")
	unsetenv(t, "APPLE_ISSUER")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 10*time.Minute, cfg.OAuth.StateTTL)
	assert.Empty(t, cfg.OAuth.AllowedRedirectURIs)
	assert.Equal(t, "https://accounts.google.com/o/oauth2/v2/auth", cfg.OAuth.Google.AuthURL)
	assert.Equal(t, "https://oauth2.googleapis.com/token", cfg.OAuth.Google.TokenURL)
	assert.Equal(t, "https://openidconnect.googleapis.com/v1/userinfo", cfg.OAuth.Google.UserInfoURL)
	assert.Equal(t, []string{"openid", "email", "profile"}, cfg.OAuth.Google.Scopes)
	assert.False(t, cfg.OAuth.Google.Enabled)
	assert.False(t, cfg.OAuth.Yandex.Enabled)
	assert.False(t, cfg.OAuth.AppleNative.Enabled)
	assert.Equal(t, "https://appleid.apple.com/auth/keys", cfg.OAuth.AppleNative.JWKSURL)
	assert.Equal(t, time.Hour, cfg.OAuth.AppleNative.JWKSCacheTTL)
	assert.Equal(t, "https://appleid.apple.com", cfg.OAuth.AppleNative.Issuer)
}

func TestLoad_OAuthCustomValues(t *testing.T) {
	setenv(t, "OAUTH_STATE_TTL", "20m")
	setenv(t, "OAUTH_ALLOWED_REDIRECT_URIS", "https://app.foodsea.ru/oauth/callback, foodsea://oauth/callback")
	setenv(t, "OAUTH_NATIVE_ALLOWED_REDIRECT_URIS", "foodsea-dev://oauth/callback,foodsea://oauth/callback")
	setenv(t, "OAUTH_LEGACY_ENABLED", "false")
	setenv(t, "OAUTH_NATIVE_ENABLED", "true")
	setenv(t, "OAUTH_GOOGLE_CLIENT_ID", "google-client-id")
	setenv(t, "OAUTH_GOOGLE_CLIENT_SECRET", "google-client-secret")
	setenv(t, "OAUTH_GOOGLE_NATIVE_CLIENT_ID", "google-native-client-id")
	setenv(t, "OAUTH_YANDEX_CLIENT_ID", "yandex-client-id")
	setenv(t, "OAUTH_YANDEX_CLIENT_SECRET", "yandex-client-secret")
	setenv(t, "OAUTH_YANDEX_NATIVE_SDK_ENABLED", "true")
	setenv(t, "OAUTH_APPLE_CLIENT_ID", "me.foodSea")
	setenv(t, "OAUTH_APPLE_JWKS_URL", "https://apple.example.test/keys")
	setenv(t, "OAUTH_APPLE_JWKS_CACHE_TTL", "30m")
	setenv(t, "OAUTH_APPLE_ISSUER", "https://apple.example.test")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 20*time.Minute, cfg.OAuth.StateTTL)
	assert.Equal(t, []string{"https://app.foodsea.ru/oauth/callback", "foodsea://oauth/callback"}, cfg.OAuth.AllowedRedirectURIs)
	assert.Equal(t, []string{"foodsea-dev://oauth/callback", "foodsea://oauth/callback"}, cfg.OAuth.NativeAllowedRedirectURIs)
	assert.False(t, cfg.OAuth.LegacyEnabled)
	assert.True(t, cfg.OAuth.NativeEnabled)
	assert.True(t, cfg.OAuth.Google.Enabled)
	assert.Equal(t, "google-client-id", cfg.OAuth.Google.ClientID)
	assert.Equal(t, "google-client-secret", cfg.OAuth.Google.ClientSecret)
	assert.True(t, cfg.OAuth.GoogleNative.Enabled)
	assert.Equal(t, "google-native-client-id", cfg.OAuth.GoogleNative.ClientID)
	assert.Empty(t, cfg.OAuth.GoogleNative.ClientSecret)
	assert.True(t, cfg.OAuth.Yandex.Enabled)
	assert.Equal(t, "yandex-client-id", cfg.OAuth.Yandex.ClientID)
	assert.Equal(t, "yandex-client-secret", cfg.OAuth.Yandex.ClientSecret)
	assert.True(t, cfg.OAuth.YandexNativeSDKEnabled)
	assert.True(t, cfg.OAuth.AppleNative.Enabled)
	assert.Equal(t, "me.foodSea", cfg.OAuth.AppleNative.ClientID)
	assert.Equal(t, "https://apple.example.test/keys", cfg.OAuth.AppleNative.JWKSURL)
	assert.Equal(t, 30*time.Minute, cfg.OAuth.AppleNative.JWKSCacheTTL)
	assert.Equal(t, "https://apple.example.test", cfg.OAuth.AppleNative.Issuer)
}

func TestLoad_OAuthAppleNewEnvHasPriorityOverLegacy(t *testing.T) {
	setenv(t, "OAUTH_APPLE_CLIENT_ID", "legacy.client")
	setenv(t, "OAUTH_APPLE_JWKS_URL", "https://legacy.apple.test/keys")
	setenv(t, "OAUTH_APPLE_JWKS_CACHE_TTL", "11m")
	setenv(t, "OAUTH_APPLE_ISSUER", "https://legacy.apple.test")

	setenv(t, "APPLE_CLIENT_ID", "new.client")
	setenv(t, "APPLE_JWKS_URL", "https://new.apple.test/keys")
	setenv(t, "APPLE_JWKS_CACHE_TTL", "22m")
	setenv(t, "APPLE_ISSUER", "https://new.apple.test")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "new.client", cfg.OAuth.AppleNative.ClientID)
	assert.Equal(t, "https://new.apple.test/keys", cfg.OAuth.AppleNative.JWKSURL)
	assert.Equal(t, 22*time.Minute, cfg.OAuth.AppleNative.JWKSCacheTTL)
	assert.Equal(t, "https://new.apple.test", cfg.OAuth.AppleNative.Issuer)
}

func TestLoad_OAuthAppleEnabledRules(t *testing.T) {
	t.Run("fallback to client id when APPLE_ENABLED not set", func(t *testing.T) {
		unsetenv(t, "APPLE_ENABLED")
		unsetenv(t, "APPLE_CLIENT_ID")
		unsetenv(t, "OAUTH_APPLE_CLIENT_ID")
		setenv(t, "OAUTH_APPLE_CLIENT_ID", "legacy.client")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.True(t, cfg.OAuth.AppleNative.Enabled)
	})

	t.Run("explicit APPLE_ENABLED=false overrides client id", func(t *testing.T) {
		setenv(t, "APPLE_CLIENT_ID", "new.client")
		setenv(t, "APPLE_ENABLED", "false")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.False(t, cfg.OAuth.AppleNative.Enabled)
	})

	t.Run("explicit APPLE_ENABLED=true works without client id", func(t *testing.T) {
		unsetenv(t, "APPLE_CLIENT_ID")
		unsetenv(t, "OAUTH_APPLE_CLIENT_ID")
		setenv(t, "APPLE_ENABLED", "true")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.True(t, cfg.OAuth.AppleNative.Enabled)
	})
}

func TestLoad_ProdOAuthPartialCredentials(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	setenv(t, "OAUTH_GOOGLE_CLIENT_ID", "google-client-id")
	unsetenv(t, "OAUTH_GOOGLE_CLIENT_SECRET")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAUTH_GOOGLE_CLIENT_ID")
	assert.Contains(t, err.Error(), "OAUTH_GOOGLE_CLIENT_SECRET")
}

func TestLoad_ProdOAuthWithoutRedirectURIs(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	setenv(t, "OAUTH_GOOGLE_CLIENT_ID", "google-client-id")
	setenv(t, "OAUTH_GOOGLE_CLIENT_SECRET", "google-client-secret")
	unsetenv(t, "OAUTH_ALLOWED_REDIRECT_URIS")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAUTH_ALLOWED_REDIRECT_URIS")
}

func TestLoad_ProdNativeOAuthWithoutNativeRedirectURIs(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	setenv(t, "OAUTH_NATIVE_ENABLED", "true")
	setenv(t, "OAUTH_GOOGLE_NATIVE_CLIENT_ID", "google-native-client-id")
	unsetenv(t, "OAUTH_NATIVE_ALLOWED_REDIRECT_URIS")
	setenv(t, "OAUTH_ALLOWED_REDIRECT_URIS", "foodsea://legacy/callback")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAUTH_NATIVE_ALLOWED_REDIRECT_URIS")
}

func TestLoad_ProdNativeOAuthAppleWithoutNativeRedirectURIs(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	setenv(t, "OAUTH_NATIVE_ENABLED", "true")
	unsetenv(t, "APPLE_ENABLED")
	unsetenv(t, "APPLE_CLIENT_ID")
	setenv(t, "OAUTH_APPLE_CLIENT_ID", "me.foodSea")
	unsetenv(t, "OAUTH_NATIVE_ALLOWED_REDIRECT_URIS")
	setenv(t, "OAUTH_ALLOWED_REDIRECT_URIS", "foodsea://legacy/callback")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAUTH_NATIVE_ALLOWED_REDIRECT_URIS")
}

func TestLoad_PhotoSearchDefaults(t *testing.T) {
	unsetenv(t, "ML_GRPC_ADDR")
	unsetenv(t, "PHOTO_SEARCH_MAX_IMAGE_BYTES")
	unsetenv(t, "PHOTO_SEARCH_TIMEOUT")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "ml-service:50051", cfg.ML.GRPCAddr)
	assert.Equal(t, int64(8*1024*1024), cfg.PhotoSearch.MaxImageBytes)
	assert.Equal(t, 10*time.Second, cfg.PhotoSearch.Timeout)
}

func TestLoad_PhotoSearchCustomValues(t *testing.T) {
	setenv(t, "ML_GRPC_ADDR", "127.0.0.1:50052")
	setenv(t, "PHOTO_SEARCH_MAX_IMAGE_BYTES", "1048576")
	setenv(t, "PHOTO_SEARCH_TIMEOUT", "3s")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:50052", cfg.ML.GRPCAddr)
	assert.Equal(t, int64(1048576), cfg.PhotoSearch.MaxImageBytes)
	assert.Equal(t, 3*time.Second, cfg.PhotoSearch.Timeout)
}

func TestLoad_InvalidPhotoSearchConfig(t *testing.T) {
	setenv(t, "PHOTO_SEARCH_MAX_IMAGE_BYTES", "0")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PHOTO_SEARCH_MAX_IMAGE_BYTES")
}

func TestLoad_InvalidPhotoSearchTimeoutConfig(t *testing.T) {
	setenv(t, "PHOTO_SEARCH_TIMEOUT", "0s")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PHOTO_SEARCH_TIMEOUT")
}

func TestLoad_APNSEnvironmentDefaults(t *testing.T) {
	unsetenv(t, "ENV")
	unsetenv(t, "APNS_ENV")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "sandbox", cfg.APNS.Environment)

	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	unsetenv(t, "APNS_ENV")

	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.APNS.Environment)
}

func TestLoad_APNSInvalidEnvironment(t *testing.T) {
	setenv(t, "APNS_ENV", "staging")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APNS_ENV")
}
