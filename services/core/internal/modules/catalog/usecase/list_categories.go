package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/foodsea/core/internal/modules/catalog/domain"
)

// ListCategories returns the full two-level category tree.
type ListCategories struct {
	categories domain.CategoryRepository
	cache      domain.ProductCache
	log        *slog.Logger
}

func NewListCategories(categories domain.CategoryRepository, cache domain.ProductCache, log *slog.Logger) *ListCategories {
	return &ListCategories{categories: categories, cache: cache, log: log}
}

func (uc *ListCategories) Execute(ctx context.Context) ([]domain.Category, error) {
	cached, err := uc.cache.GetCategoriesTree(ctx)
	if err != nil {
		uc.log.WarnContext(ctx, "catalog: categories cache get error", "error", err)
	}
	if cached != nil {
		return cached, nil
	}

	all, err := uc.categories.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("catalog.ListCategories: %w", err)
	}

	tree := buildTree(all)

	if cacheErr := uc.cache.SetCategoriesTree(ctx, tree); cacheErr != nil {
		uc.log.WarnContext(ctx, "catalog: categories cache set error", "error", cacheErr)
	}

	return tree, nil
}

// buildTree assembles a flat list of categories into a two-level tree sorted by SortOrder then Name.
func buildTree(all []domain.Category) []domain.Category {
	childrenOf := make(map[string][]domain.Category)
	var roots []domain.Category

	for _, c := range all {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else {
			key := c.ParentID.String()
			childrenOf[key] = append(childrenOf[key], c)
		}
	}

	sortCategories := func(cats []domain.Category) {
		sort.Slice(cats, func(i, j int) bool {
			if cats[i].SortOrder != cats[j].SortOrder {
				return cats[i].SortOrder < cats[j].SortOrder
			}
			return cats[i].Name < cats[j].Name
		})
	}

	sortCategories(roots)

	for i := range roots {
		children := childrenOf[roots[i].ID.String()]
		sortCategories(children)
		roots[i].Children = children
	}

	return roots
}
