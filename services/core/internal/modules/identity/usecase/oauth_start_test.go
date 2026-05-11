package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStart_Execute(t *testing.T) {
	ctx := context.Background()
	stateTTL := 5 * time.Minute

	t.Run("start success", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderGoogle).Once()

		uc := usecase.NewOAuthStart(states, []domain.OAuthProvider{provider}, []string{"/cb"}, []string{"/native-cb"}, stateTTL)
		states.On("Create", ctx, mock.MatchedBy(func(s domain.OAuthSession) bool {
			return s.Provider == domain.OAuthProviderGoogle && s.RedirectTo == "/cb" && s.Mode == domain.OAuthFlowModeLegacy
		})).Return("state-123", nil).Once()
		provider.On("AuthURL", ctx, "state-123", mock.MatchedBy(func(s domain.OAuthSession) bool {
			return s.Provider == domain.OAuthProviderGoogle && s.RedirectTo == "/cb" && s.Mode == domain.OAuthFlowModeLegacy
		})).Return("https://oauth.example/auth?state=state-123", nil).Once()

		result, err := uc.Execute(ctx, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		})
		require.NoError(t, err)
		assert.Equal(t, "state-123", result.State)
		assert.Equal(t, "https://oauth.example/auth?state=state-123", result.AuthURL)
	})

	t.Run("disallowed redirect returns invalid input", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderGoogle).Once()

		uc := usecase.NewOAuthStart(states, []domain.OAuthProvider{provider}, []string{"/cb"}, []string{"/native-cb"}, stateTTL)

		_, err := uc.Execute(ctx, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/not-allowed",
		})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})

	t.Run("disabled provider returns invalid input", func(t *testing.T) {
		uc := usecase.NewOAuthStart(new(MockOAuthStateStore), nil, []string{"/cb"}, []string{"/native-cb"}, stateTTL)

		_, err := uc.Execute(ctx, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderVK,
			RedirectTo: "/cb",
		})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})

	t.Run("provider auth url error passthrough", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderApple).Once()

		uc := usecase.NewOAuthStart(states, []domain.OAuthProvider{provider}, []string{"/cb"}, []string{"/native-cb"}, stateTTL)

		states.On("Create", ctx, mock.Anything).Return("state-1", nil).Once()
		provider.On("AuthURL", ctx, "state-1", mock.Anything).Return("", errors.New("provider down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderApple,
			RedirectTo: "/cb",
		})
		require.Error(t, err)
		assert.Equal(t, "provider down", err.Error())
	})

	t.Run("state store create error passthrough", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderGoogle).Once()

		uc := usecase.NewOAuthStart(states, []domain.OAuthProvider{provider}, []string{"/cb"}, []string{"/native-cb"}, stateTTL)
		states.On("Create", ctx, mock.Anything).Return("", errors.New("redis down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		})
		require.Error(t, err)
		assert.Equal(t, "redis down", err.Error())
	})

	t.Run("native start success includes nonce and pkce", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderGoogle).Once()

		uc := usecase.NewOAuthStart(states, []domain.OAuthProvider{provider}, []string{"/cb"}, []string{"foodsea://oauth/callback"}, stateTTL)
		states.On("Create", ctx, mock.MatchedBy(func(s domain.OAuthSession) bool {
			return s.Provider == domain.OAuthProviderGoogle &&
				s.RedirectTo == "foodsea://oauth/callback" &&
				s.Mode == domain.OAuthFlowModeNative &&
				s.Nonce != "" &&
				s.PKCEVerifier != ""
		})).Return("state-native", nil).Once()
		provider.On("AuthURL", ctx, "state-native", mock.MatchedBy(func(s domain.OAuthSession) bool {
			return s.Mode == domain.OAuthFlowModeNative && s.Nonce != "" && s.PKCEVerifier != ""
		})).Return("https://oauth.example/native", nil).Once()

		result, err := uc.Execute(ctx, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "foodsea://oauth/callback",
			Mode:       domain.OAuthFlowModeNative,
		})
		require.NoError(t, err)
		assert.Equal(t, "state-native", result.State)
		assert.Equal(t, "https://oauth.example/native", result.AuthURL)
	})
}
