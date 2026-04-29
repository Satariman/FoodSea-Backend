package usecase

import (
	"context"
	"fmt"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type Refresh struct {
	tokens domain.TokenService
}

func NewRefresh(tokens domain.TokenService) *Refresh {
	return &Refresh{tokens: tokens}
}

func (r *Refresh) Execute(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	if refreshToken == "" {
		return domain.TokenPair{}, fmt.Errorf("%w: refresh token is required", sherrors.ErrInvalidInput)
	}
	return r.tokens.RotateRefresh(ctx, refreshToken)
}
