package domain

import "context"

// SearchRepository fetches search results from the data store.
type SearchRepository interface {
	Search(ctx context.Context, query SearchQuery) (SearchResult, error)
}
