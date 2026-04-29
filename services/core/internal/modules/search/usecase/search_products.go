package usecase

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/foodsea/core/internal/modules/search/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

const maxTextLen = 200

// SearchProducts executes a full-text search with filters.
type SearchProducts struct {
	repo  domain.SearchRepository
	cache domain.SearchCache
	log   *slog.Logger
}

func NewSearchProducts(repo domain.SearchRepository, cache domain.SearchCache, log *slog.Logger) *SearchProducts {
	return &SearchProducts{repo: repo, cache: cache, log: log}
}

func (uc *SearchProducts) Execute(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	// Normalize
	if utf8.RuneCountInString(q.Text) > maxTextLen {
		// Truncate at rune boundary to avoid cutting mid-character.
		runes := []rune(q.Text)
		q.Text = string(runes[:maxTextLen])
	}
	if q.Sort == "" {
		q.Sort = domain.SortRelevance
	}

	// Validate
	if utf8.RuneCountInString(q.Text) < 2 {
		return domain.SearchResult{}, fmt.Errorf("%w: query must be at least 2 characters", sherrors.ErrInvalidInput)
	}
	if !q.Sort.IsValid() {
		return domain.SearchResult{}, fmt.Errorf("%w: invalid sort option %q", sherrors.ErrInvalidInput, q.Sort)
	}
	if q.MinPriceKopecks != nil && q.MaxPriceKopecks != nil && *q.MinPriceKopecks > *q.MaxPriceKopecks {
		return domain.SearchResult{}, fmt.Errorf("%w: min_price cannot exceed max_price", sherrors.ErrInvalidInput)
	}

	hash := hashQuery(q)

	if uc.cache != nil {
		cached, err := uc.cache.Get(ctx, hash)
		if err != nil {
			uc.log.WarnContext(ctx, "search cache get error", "error", err)
		} else if cached != nil {
			return *cached, nil
		}
	}

	result, err := uc.repo.Search(ctx, q)
	if err != nil {
		return domain.SearchResult{}, fmt.Errorf("search.SearchProducts: %w", err)
	}

	if uc.cache != nil {
		if err := uc.cache.Set(ctx, hash, &result); err != nil {
			uc.log.WarnContext(ctx, "search cache set error", "error", err)
		}
	}

	return result, nil
}

func hashQuery(q domain.SearchQuery) string {
	var sb strings.Builder
	sb.WriteString("t=")
	sb.WriteString(q.Text)
	sb.WriteString("|cat=")
	if q.CategoryID != nil {
		sb.WriteString(q.CategoryID.String())
	}
	sb.WriteString("|sub=")
	if q.SubcategoryID != nil {
		sb.WriteString(q.SubcategoryID.String())
	}
	sb.WriteString("|brand=")
	if q.BrandID != nil {
		sb.WriteString(q.BrandID.String())
	}
	sb.WriteString("|store=")
	if q.StoreID != nil {
		sb.WriteString(q.StoreID.String())
	}
	sb.WriteString("|minp=")
	if q.MinPriceKopecks != nil {
		sb.WriteString(strconv.FormatInt(*q.MinPriceKopecks, 10))
	}
	sb.WriteString("|maxp=")
	if q.MaxPriceKopecks != nil {
		sb.WriteString(strconv.FormatInt(*q.MaxPriceKopecks, 10))
	}
	sb.WriteString("|ins=")
	if q.InStockOnly {
		sb.WriteByte('1')
	}
	sb.WriteString("|disc=")
	if q.HasDiscountOnly {
		sb.WriteByte('1')
	}
	sb.WriteString("|sort=")
	sb.WriteString(string(q.Sort))
	sb.WriteString("|page=")
	sb.WriteString(strconv.Itoa(q.Pagination.Page))
	sb.WriteString("|ps=")
	sb.WriteString(strconv.Itoa(q.Pagination.PageSize))

	h := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(h[:])
}
