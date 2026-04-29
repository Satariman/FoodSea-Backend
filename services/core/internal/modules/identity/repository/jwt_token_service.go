package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type jwtClaims struct {
	jwt.RegisteredClaims
}

type JWTTokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	redis      *redis.Client
}

func NewJWTTokenService(secret string, accessTTL, refreshTTL time.Duration, redisClient *redis.Client) *JWTTokenService {
	return &JWTTokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		redis:      redisClient,
	}
}

func (s *JWTTokenService) IssuePair(ctx context.Context, userID uuid.UUID) (domain.TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(s.accessTTL)

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(accessExp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.secret)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("signing access token: %w", err)
	}

	rawRefresh := make([]byte, 32)
	if _, err := rand.Read(rawRefresh); err != nil {
		return domain.TokenPair{}, fmt.Errorf("generating refresh token: %w", err)
	}
	refreshToken := base64.RawURLEncoding.EncodeToString(rawRefresh)
	refreshHash := tokenHash(refreshToken)

	refreshExp := now.Add(s.refreshTTL)
	key := sessionKey(userID, refreshHash)

	if err := s.redis.Set(ctx, key, userID.String(), s.refreshTTL).Err(); err != nil {
		return domain.TokenPair{}, fmt.Errorf("storing refresh token: %w", err)
	}

	return domain.TokenPair{
		Access:           accessToken,
		Refresh:          refreshToken,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
	}, nil
}

func (s *JWTTokenService) ValidateAccess(tokenStr string) (domain.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return s.secret, nil
		},
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return domain.Claims{}, sherrors.ErrUnauthorized
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok {
		return domain.Claims{}, sherrors.ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return domain.Claims{}, sherrors.ErrUnauthorized
	}

	return domain.Claims{UserID: userID}, nil
}

// RotateRefresh finds the refresh token in Redis by hash, invalidates it,
// and issues a new token pair. Uses SCAN to avoid storing a separate reverse index.
func (s *JWTTokenService) RotateRefresh(ctx context.Context, refresh string) (domain.TokenPair, error) {
	if refresh == "" {
		return domain.TokenPair{}, sherrors.ErrUnauthorized
	}

	hash := tokenHash(refresh)
	pattern := "session:*:" + hash

	foundKey, err := s.scanForKey(ctx, pattern)
	if err != nil {
		return domain.TokenPair{}, err
	}
	if foundKey == "" {
		return domain.TokenPair{}, sherrors.ErrUnauthorized
	}

	userIDStr, err := s.redis.Get(ctx, foundKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return domain.TokenPair{}, sherrors.ErrUnauthorized
		}
		return domain.TokenPair{}, fmt.Errorf("getting refresh token: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return domain.TokenPair{}, sherrors.ErrUnauthorized
	}

	if err := s.redis.Del(ctx, foundKey).Err(); err != nil {
		return domain.TokenPair{}, fmt.Errorf("deleting old refresh token: %w", err)
	}

	return s.IssuePair(ctx, userID)
}

// Revoke deletes all refresh tokens for the given user.
func (s *JWTTokenService) Revoke(ctx context.Context, userID uuid.UUID) error {
	pattern := fmt.Sprintf("session:%s:*", userID.String())

	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scanning sessions: %w", err)
		}
		if len(keys) > 0 {
			if err := s.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("deleting sessions: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (s *JWTTokenService) scanForKey(ctx context.Context, pattern string) (string, error) {
	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 10).Result()
		if err != nil {
			return "", fmt.Errorf("scanning tokens: %w", err)
		}
		if len(keys) > 0 {
			return keys[0], nil
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return "", nil
}

func sessionKey(userID uuid.UUID, hash string) string {
	return fmt.Sprintf("session:%s:%s", userID.String(), hash)
}

func tokenHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
