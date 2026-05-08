package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/identity/domain"
)

func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "pg duplicate key", err: &pgconn.PgError{Code: "23505"}, want: true},
		{name: "pg foreign key", err: &pgconn.PgError{Code: "23503"}, want: false},
		{
			name: "fallback duplicate text",
			err:  errors.New(`ERROR: duplicate key value violates unique constraint "oauth_identities_provider_key"`),
			want: true,
		},
		{name: "generic error", err: errors.New("db down"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isUniqueViolation(tt.err))
		})
	}
}

func TestToDomainOAuthIdentity(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	now := time.Now().UTC()
	src := &ent.OAuthIdentity{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		Provider:       string(domain.OAuthProviderGoogle),
		ProviderUserID: "sub-1",
		Email:          &email,
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Second),
	}

	got := toDomainOAuthIdentity(src)
	assert.Equal(t, src.ID, got.ID)
	assert.Equal(t, src.UserID, got.UserID)
	assert.Equal(t, domain.OAuthProviderGoogle, got.Provider)
	assert.Equal(t, src.ProviderUserID, got.ProviderUserID)
	assert.Equal(t, src.Email, got.Email)
	assert.Equal(t, src.CreatedAt, got.CreatedAt)
	assert.Equal(t, src.UpdatedAt, got.UpdatedAt)
}
