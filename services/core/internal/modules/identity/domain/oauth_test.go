package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestParseOAuthProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    OAuthProviderKind
		wantErr error
	}{
		{name: "google with spaces and case", raw: "  GoOgLe  ", want: OAuthProviderGoogle},
		{name: "apple", raw: "apple", want: OAuthProviderApple},
		{name: "vk", raw: "vk", want: OAuthProviderVK},
		{name: "yandex", raw: "yandex", want: OAuthProviderYandex},
		{name: "unsupported", raw: "github", wantErr: sherrors.ErrInvalidInput},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseOAuthProvider(tt.raw)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseOAuthProviderName(t *testing.T) {
	t.Parallel()

	got, err := ParseOAuthProviderName("YANDEX")
	assert.NoError(t, err)
	assert.Equal(t, OAuthProviderYandex, got)

	_, err = ParseOAuthProviderName("unknown-provider")
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}
