package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/search/domain"
)

const searchSQL = `
WITH ranked AS (
    SELECT
        p.id, p.name, p.image_url, p.barcode, p.in_stock,
        p.category_id, p.subcategory_id, p.brand_id,
        ts_rank(
            to_tsvector('russian', p.name || ' ' || coalesce(p.description,'') || ' ' || coalesce(p.composition,'')),
            plainto_tsquery('russian', $1)
        ) AS score
    FROM products p
    WHERE to_tsvector('russian', p.name || ' ' || coalesce(p.description,'') || ' ' || coalesce(p.composition,''))
          @@ plainto_tsquery('russian', $1)
      AND ($2::uuid IS NULL OR p.category_id = $2)
      AND ($3::uuid IS NULL OR p.subcategory_id = $3)
      AND ($4::uuid IS NULL OR p.brand_id = $4)
      AND (NOT $5::bool OR p.in_stock = true)
),
priced AS (
    SELECT r.*,
           MIN(o.price_kopecks)    FILTER (WHERE o.in_stock AND ($6::uuid IS NULL OR o.store_id = $6)) AS min_price,
           COUNT(o.id)             FILTER (WHERE o.in_stock AND ($6::uuid IS NULL OR o.store_id = $6)) AS offers_count,
           MAX(o.discount_percent) FILTER (WHERE o.in_stock AND ($6::uuid IS NULL OR o.store_id = $6)) AS max_discount_pct
    FROM ranked r
    LEFT JOIN offers o ON o.product_id = r.id
    GROUP BY r.id, r.name, r.image_url, r.barcode, r.in_stock,
             r.category_id, r.subcategory_id, r.brand_id, r.score
),
filtered AS (
    SELECT *
    FROM priced
    WHERE ($7::bigint IS NULL OR min_price >= $7)
      AND ($8::bigint IS NULL OR min_price <= $8)
      AND (NOT $12::bool OR COALESCE(max_discount_pct, 0) > 0)
)
SELECT
    id, name, image_url, barcode, in_stock,
    category_id, subcategory_id, brand_id,
    score, min_price, offers_count, max_discount_pct,
    COUNT(*) OVER () AS total_count
FROM filtered
ORDER BY
    CASE WHEN $9 = 'relevance'     THEN score          END DESC NULLS LAST,
    CASE WHEN $9 = 'price_asc'     THEN min_price      END ASC  NULLS LAST,
    CASE WHEN $9 = 'price_desc'    THEN min_price      END DESC NULLS LAST,
    CASE WHEN $9 = 'discount_desc' THEN max_discount_pct END DESC NULLS LAST,
    CASE WHEN $9 = 'name_asc'      THEN name           END ASC,
    CASE WHEN $9 = 'name_desc'     THEN name           END DESC,
    name ASC
LIMIT $10 OFFSET $11
`

// SearchRepo implements domain.SearchRepository using raw SQL against core_db.
type SearchRepo struct {
	db *sql.DB
}

func NewSearchRepo(db *sql.DB) *SearchRepo {
	return &SearchRepo{db: db}
}

func (r *SearchRepo) Search(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	args := []any{
		q.Text,                        // $1
		uuidParam(q.CategoryID),       // $2
		uuidParam(q.SubcategoryID),    // $3
		uuidParam(q.BrandID),          // $4
		q.InStockOnly,                 // $5
		uuidParam(q.StoreID),          // $6
		int64Param(q.MinPriceKopecks), // $7
		int64Param(q.MaxPriceKopecks), // $8
		string(q.Sort),                // $9
		q.Pagination.Limit(),          // $10
		q.Pagination.Offset(),         // $11
		q.HasDiscountOnly,             // $12
	}

	rows, err := r.db.QueryContext(ctx, searchSQL, args...)
	if err != nil {
		return domain.SearchResult{}, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var items []domain.SearchResultItem
	total := 0

	for rows.Next() {
		item, rowTotal, err := rowToItem(rows)
		if err != nil {
			return domain.SearchResult{}, fmt.Errorf("scanning search row: %w", err)
		}
		items = append(items, item)
		total = rowTotal
	}
	if err := rows.Err(); err != nil {
		return domain.SearchResult{}, fmt.Errorf("iterating search rows: %w", err)
	}

	if items == nil {
		items = []domain.SearchResultItem{}
	}

	return domain.SearchResult{Items: items, Total: total}, nil
}

func rowToItem(rows *sql.Rows) (domain.SearchResultItem, int, error) {
	var (
		id             string
		name           string
		imageURL       sql.NullString
		barcode        sql.NullString
		inStock        bool
		categoryID     string
		subcategoryID  sql.NullString
		brandID        sql.NullString
		score          float64
		minPrice       sql.NullInt64
		offersCount    int64
		maxDiscountPct sql.NullInt64
		totalCount     int64
	)

	if err := rows.Scan(
		&id, &name, &imageURL, &barcode, &inStock,
		&categoryID, &subcategoryID, &brandID,
		&score, &minPrice, &offersCount, &maxDiscountPct, &totalCount,
	); err != nil {
		return domain.SearchResultItem{}, 0, err
	}

	productID, err := uuid.Parse(id)
	if err != nil {
		return domain.SearchResultItem{}, 0, fmt.Errorf("invalid product id %q: %w", id, err)
	}
	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return domain.SearchResultItem{}, 0, fmt.Errorf("invalid category_id %q: %w", categoryID, err)
	}

	brief := domain.ProductBrief{
		ID:         productID,
		Name:       name,
		InStock:    inStock,
		CategoryID: catID,
	}
	if imageURL.Valid {
		brief.ImageURL = &imageURL.String
	}
	if barcode.Valid {
		brief.Barcode = &barcode.String
	}
	if subcategoryID.Valid {
		sub, err := uuid.Parse(subcategoryID.String)
		if err != nil {
			return domain.SearchResultItem{}, 0, fmt.Errorf("invalid subcategory_id %q: %w", subcategoryID.String, err)
		}
		brief.SubcategoryID = &sub
	}
	if brandID.Valid {
		br, err := uuid.Parse(brandID.String)
		if err != nil {
			return domain.SearchResultItem{}, 0, fmt.Errorf("invalid brand_id %q: %w", brandID.String, err)
		}
		brief.BrandID = &br
	}

	item := domain.SearchResultItem{
		Product:     brief,
		Score:       score,
		OffersCount: int16(offersCount),
	}
	if minPrice.Valid {
		item.MinPriceKopecks = minPrice.Int64
	}
	if maxDiscountPct.Valid {
		item.MaxDiscountPercent = int8(maxDiscountPct.Int64)
	}

	return item, int(totalCount), nil
}

func uuidParam(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func int64Param(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
