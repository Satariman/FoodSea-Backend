package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/search/domain"
	"github.com/foodsea/core/internal/modules/search/usecase"
	shared "github.com/foodsea/core/internal/shared/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// --- mocks ---

type mockSearchRepo struct{ mock.Mock }

func (m *mockSearchRepo) Search(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	args := m.Called(ctx, q)
	if v, ok := args.Get(0).(domain.SearchResult); ok {
		return v, args.Error(1)
	}
	return domain.SearchResult{}, args.Error(1)
}

type mockSearchCache struct{ mock.Mock }

func (m *mockSearchCache) Get(ctx context.Context, hash string) (*domain.SearchResult, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.SearchResult)
	return v, args.Error(1)
}

func (m *mockSearchCache) Set(ctx context.Context, hash string, result *domain.SearchResult) error {
	args := m.Called(ctx, hash, result)
	return args.Error(0)
}

// --- helpers ---

func fakeResult() domain.SearchResult {
	return domain.SearchResult{Items: []domain.SearchResultItem{}, Total: 0}
}

func baseQuery() domain.SearchQuery {
	return domain.SearchQuery{
		Text:       "молоко",
		Sort:       domain.SortRelevance,
		Pagination: shared.NewPagination(1, 20),
	}
}

// --- tests ---

func TestSearchProducts_TextTooShort(t *testing.T) {
	uc := usecase.NewSearchProducts(new(mockSearchRepo), new(mockSearchCache), slog.Default())

	// Single ASCII character → 1 rune → invalid
	_, err := uc.Execute(context.Background(), domain.SearchQuery{Text: "a"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
}

func TestSearchProducts_EmptyText(t *testing.T) {
	uc := usecase.NewSearchProducts(new(mockSearchRepo), new(mockSearchCache), slog.Default())

	_, err := uc.Execute(context.Background(), domain.SearchQuery{Text: ""})

	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
}

func TestSearchProducts_InvalidSort(t *testing.T) {
	uc := usecase.NewSearchProducts(new(mockSearchRepo), new(mockSearchCache), slog.Default())

	q := baseQuery()
	q.Sort = "invalid"
	_, err := uc.Execute(context.Background(), q)

	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
}

func TestSearchProducts_MinPriceExceedsMaxPrice(t *testing.T) {
	uc := usecase.NewSearchProducts(new(mockSearchRepo), new(mockSearchCache), slog.Default())

	q := baseQuery()
	min := int64(500)
	max := int64(100)
	q.MinPriceKopecks = &min
	q.MaxPriceKopecks = &max

	_, err := uc.Execute(context.Background(), q)

	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
}

func TestSearchProducts_CacheHit_RepoNotCalled(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	expected := fakeResult()
	expected.Total = 3

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(&expected, nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	result, err := uc.Execute(context.Background(), baseQuery())

	require.NoError(t, err)
	assert.Equal(t, expected.Total, result.Total)
	repo.AssertNotCalled(t, "Search")
	cache.AssertNotCalled(t, "Set")
}

func TestSearchProducts_CacheMiss_RepoCalledAndCacheSet(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	expected := fakeResult()
	expected.Total = 2

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("Search", mock.Anything, mock.AnythingOfType("domain.SearchQuery")).Return(expected, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	result, err := uc.Execute(context.Background(), baseQuery())

	require.NoError(t, err)
	assert.Equal(t, expected.Total, result.Total)
	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestSearchProducts_CacheSetError_ResultStillReturned(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	expected := fakeResult()

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("Search", mock.Anything, mock.AnythingOfType("domain.SearchQuery")).Return(expected, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(errors.New("redis error"))

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	result, err := uc.Execute(context.Background(), baseQuery())

	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestSearchProducts_CacheGetError_FallsBackToRepo(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	expected := fakeResult()

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, errors.New("redis error"))
	repo.On("Search", mock.Anything, mock.AnythingOfType("domain.SearchQuery")).Return(expected, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	result, err := uc.Execute(context.Background(), baseQuery())

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestSearchProducts_HashStable(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	var capturedHashes []string
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			capturedHashes = append(capturedHashes, args.String(1))
		}).
		Return(nil, nil)
	repo.On("Search", mock.Anything, mock.AnythingOfType("domain.SearchQuery")).Return(fakeResult(), nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	q := baseQuery()

	_, err := uc.Execute(context.Background(), q)
	require.NoError(t, err)
	_, err = uc.Execute(context.Background(), q)
	require.NoError(t, err)

	require.Len(t, capturedHashes, 2)
	assert.Equal(t, capturedHashes[0], capturedHashes[1], "hash must be stable for identical queries")
}

func TestSearchProducts_HasDiscountOnly_PassedToRepo(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("Search", mock.Anything, mock.MatchedBy(func(q domain.SearchQuery) bool {
		return q.HasDiscountOnly
	})).Return(fakeResult(), nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	q := baseQuery()
	q.HasDiscountOnly = true
	_, err := uc.Execute(context.Background(), q)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSearchProducts_DiscountDescSort_Valid(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("Search", mock.Anything, mock.MatchedBy(func(q domain.SearchQuery) bool {
		return q.Sort == domain.SortDiscountDesc
	})).Return(fakeResult(), nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())
	q := baseQuery()
	q.Sort = domain.SortDiscountDesc
	_, err := uc.Execute(context.Background(), q)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSearchProducts_HasDiscountAndDiscountSort_DifferentHash(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	var capturedHashes []string
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			capturedHashes = append(capturedHashes, args.String(1))
		}).
		Return(nil, nil)
	repo.On("Search", mock.Anything, mock.AnythingOfType("domain.SearchQuery")).Return(fakeResult(), nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())

	q1 := baseQuery()
	q2 := baseQuery()
	q2.HasDiscountOnly = true

	_, err := uc.Execute(context.Background(), q1)
	require.NoError(t, err)
	_, err = uc.Execute(context.Background(), q2)
	require.NoError(t, err)

	require.Len(t, capturedHashes, 2)
	assert.NotEqual(t, capturedHashes[0], capturedHashes[1], "different HasDiscountOnly must produce different hashes")
}

func TestSearchProducts_DefaultSort_IsRelevance(t *testing.T) {
	repo := new(mockSearchRepo)
	cache := new(mockSearchCache)

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("Search", mock.Anything, mock.MatchedBy(func(q domain.SearchQuery) bool {
		return q.Sort == domain.SortRelevance
	})).Return(fakeResult(), nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	uc := usecase.NewSearchProducts(repo, cache, slog.Default())

	q := baseQuery()
	q.Sort = "" // empty — should default to relevance
	_, err := uc.Execute(context.Background(), q)
	require.NoError(t, err)
}
