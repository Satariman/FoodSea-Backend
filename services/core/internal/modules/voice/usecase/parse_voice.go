package usecase

import (
	"context"

	"github.com/foodsea/core/internal/modules/voice/domain"
)

type MLVoiceClient interface {
	ParseShoppingList(ctx context.Context, text, locale string) ([]domain.VoiceItem, []string, error)
}

type ParseVoice struct {
	ml MLVoiceClient
}

func NewParseVoice(ml MLVoiceClient) *ParseVoice {
	return &ParseVoice{ml: ml}
}

func (u *ParseVoice) Execute(ctx context.Context, text, locale string) ([]domain.VoiceItem, []string, error) {
	return u.ml.ParseShoppingList(ctx, text, locale)
}
