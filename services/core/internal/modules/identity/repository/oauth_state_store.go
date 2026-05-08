package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthStateStore struct {
	redis *redis.Client
	ttl   time.Duration
}

var (
	oauthStateNow      = time.Now
	oauthStateRandRead = rand.Read
	oauthStateMarshal  = json.Marshal
)

func NewOAuthStateStore(redisClient *redis.Client, ttl time.Duration) *OAuthStateStore {
	return &OAuthStateStore{
		redis: redisClient,
		ttl:   ttl,
	}
}

func (s *OAuthStateStore) Create(ctx context.Context, session domain.OAuthSession) (string, error) {
	state, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("generating oauth state: %w", err)
	}

	now := oauthStateNow()
	session.State = state
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.ExpiresAt = now.Add(s.ttl)

	payload, err := oauthStateMarshal(session)
	if err != nil {
		return "", fmt.Errorf("marshaling oauth session: %w", err)
	}

	key := oauthStateKey(state)
	if err := s.redis.Set(ctx, key, payload, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("storing oauth state: %w", err)
	}

	return state, nil
}

func (s *OAuthStateStore) Consume(ctx context.Context, state string) (domain.OAuthSession, error) {
	if state == "" {
		return domain.OAuthSession{}, sherrors.ErrUnauthorized
	}

	key := oauthStateKey(state)
	raw, err := s.redis.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return domain.OAuthSession{}, sherrors.ErrUnauthorized
		}
		return domain.OAuthSession{}, fmt.Errorf("consuming oauth state: %w", err)
	}

	var session domain.OAuthSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return domain.OAuthSession{}, fmt.Errorf("unmarshaling oauth session: %w", err)
	}
	if session.ExpiresAt.IsZero() || oauthStateNow().After(session.ExpiresAt) {
		return domain.OAuthSession{}, sherrors.ErrUnauthorized
	}

	return session, nil
}

func oauthStateKey(state string) string {
	hash := sha256.Sum256([]byte(state))
	return "oauth:state:" + hex.EncodeToString(hash[:])
}

func randomURLToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := oauthStateRandRead(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
