package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/ent/oauthidentity"
	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthIdentityRepo struct {
	client *ent.Client
}

func NewOAuthIdentityRepo(client *ent.Client) *OAuthIdentityRepo {
	return &OAuthIdentityRepo{client: client}
}

func (r *OAuthIdentityRepo) GetByProviderUserID(
	ctx context.Context,
	provider domain.OAuthProviderKind,
	providerUserID string,
) (*domain.OAuthIdentity, error) {
	identity, err := r.client.OAuthIdentity.Query().
		Where(
			oauthidentity.Provider(string(provider)),
			oauthidentity.ProviderUserID(providerUserID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting oauth identity by provider user id: %w", err)
	}

	return toDomainOAuthIdentity(identity), nil
}

func (r *OAuthIdentityRepo) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	created, err := r.client.OAuthIdentity.Create().
		SetID(identity.ID).
		SetUserID(identity.UserID).
		SetProvider(string(identity.Provider)).
		SetProviderUserID(identity.ProviderUserID).
		SetNillableEmail(identity.Email).
		Save(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			return sherrors.ErrAlreadyExists
		}
		return fmt.Errorf("creating oauth identity: %w", err)
	}

	identity.CreatedAt = created.CreatedAt
	identity.UpdatedAt = created.UpdatedAt
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	// Fallback when driver-specific error isn't available through wrappers.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key value") && strings.Contains(msg, "unique constraint")
}

func toDomainOAuthIdentity(identity *ent.OAuthIdentity) *domain.OAuthIdentity {
	return &domain.OAuthIdentity{
		ID:             identity.ID,
		UserID:         identity.UserID,
		Provider:       domain.OAuthProviderKind(identity.Provider),
		ProviderUserID: identity.ProviderUserID,
		Email:          identity.Email,
		CreatedAt:      identity.CreatedAt,
		UpdatedAt:      identity.UpdatedAt,
	}
}
