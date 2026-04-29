package domain

import "context"

// SearchCache stores and retrieves search results by query hash.
type SearchCache interface {
	Get(ctx context.Context, hash string) (*SearchResult, error)
	Set(ctx context.Context, hash string, result *SearchResult) error
}
