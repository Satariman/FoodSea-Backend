package repository

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entbrand "github.com/foodsea/core/ent/brand"
	entcategory "github.com/foodsea/core/ent/category"
	entoffer "github.com/foodsea/core/ent/offer"
	"github.com/foodsea/core/ent/predicate"
	entproduct "github.com/foodsea/core/ent/product"
	"github.com/foodsea/core/internal/modules/catalog/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// ProductRepo implements domain.ProductRepository using Ent.
type ProductRepo struct {
	client *ent.Client
}

func NewProductRepo(client *ent.Client) *ProductRepo {
	return &ProductRepo{client: client}
}

func (r *ProductRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	e, err := r.client.Product.Query().
		Where(entproduct.ID(id)).
		WithCategory().
		WithSubcategory().
		WithBrand().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("querying product: %w", err)
	}
	p := toDomainProduct(e)
	return &p, nil
}

func (r *ProductRepo) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error) {
	e, err := r.client.Product.Query().
		Where(entproduct.ID(id)).
		WithCategory().
		WithSubcategory().
		WithBrand().
		WithNutrition().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("querying product with details: %w", err)
	}
	return toDomainProductDetail(e), nil
}

func (r *ProductRepo) GetByBarcode(ctx context.Context, barcode string) (*domain.ProductDetail, error) {
	e, err := r.client.Product.Query().
		Where(entproduct.BarcodeEQ(barcode)).
		WithCategory().
		WithSubcategory().
		WithBrand().
		WithNutrition().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("querying product by barcode: %w", err)
	}
	return toDomainProductDetail(e), nil
}

func (r *ProductRepo) ListAllForML(ctx context.Context) ([]domain.ProductMLData, error) {
	rows, err := r.client.Product.Query().
		Where(entproduct.InStock(true)).
		WithNutrition().
		WithOffers(func(q *ent.OfferQuery) {
			q.Where(entoffer.InStock(true))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing products for ml: %w", err)
	}

	result := make([]domain.ProductMLData, 0, len(rows))
	for _, row := range rows {
		item := domain.ProductMLData{
			ID:            row.ID,
			Name:          row.Name,
			Description:   row.Description,
			Composition:   row.Composition,
			CategoryID:    row.CategoryID,
			SubcategoryID: row.SubcategoryID,
			BrandID:       row.BrandID,
			Weight:        row.Weight,
		}

		if row.Edges.Nutrition != nil {
			item.Nutrition = &domain.Nutrition{
				Calories:      row.Edges.Nutrition.Calories,
				Protein:       row.Edges.Nutrition.Protein,
				Fat:           row.Edges.Nutrition.Fat,
				Carbohydrates: row.Edges.Nutrition.Carbohydrates,
			}
		}

		item.Offers = make([]domain.OfferBrief, 0, len(row.Edges.Offers))
		for _, offer := range row.Edges.Offers {
			item.Offers = append(item.Offers, domain.OfferBrief{
				StoreID:      offer.StoreID,
				PriceKopecks: int64(offer.PriceKopecks),
			})
		}

		result = append(result, item)
	}

	return result, nil
}

func (r *ProductRepo) List(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int, error) {
	preds := buildProductPredicates(filter)

	total, err := r.client.Product.Query().Where(preds...).Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting products: %w", err)
	}

	q := r.client.Product.Query().Where(preds...)
	q = applyProductSort(q, filter.Sort)
	q = q.Offset(filter.Pagination.Offset()).Limit(filter.Pagination.Limit())

	rows, err := q.WithCategory().WithSubcategory().WithBrand().
		WithOffers(func(q *ent.OfferQuery) {
			q.Where(entoffer.InStock(true))
		}).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing products: %w", err)
	}

	result := make([]domain.Product, len(rows))
	for i, row := range rows {
		result[i] = toDomainProduct(row)
	}
	return result, total, nil
}

func buildProductPredicates(filter domain.ProductFilter) []predicate.Product {
	var preds []predicate.Product

	if filter.CategoryID != nil {
		preds = append(preds, entproduct.HasCategoryWith(entcategory.ID(*filter.CategoryID)))
	}
	if filter.SubcategoryID != nil {
		preds = append(preds, entproduct.HasSubcategoryWith(entcategory.ID(*filter.SubcategoryID)))
	}
	if filter.BrandID != nil {
		preds = append(preds, entproduct.HasBrandWith(entbrand.ID(*filter.BrandID)))
	}
	if filter.InStockOnly {
		preds = append(preds, entproduct.InStock(true))
	}

	return preds
}

func applyProductSort(q *ent.ProductQuery, sort domain.ProductSort) *ent.ProductQuery {
	switch sort {
	case domain.SortNameAsc:
		return q.Order(ent.Asc(entproduct.FieldName))
	case domain.SortNameDesc:
		return q.Order(ent.Desc(entproduct.FieldName))
	case domain.SortCreatedDesc:
		return q.Order(ent.Desc(entproduct.FieldCreatedAt))
	default:
		return q.Order(sql.OrderByField(entproduct.FieldCreatedAt, sql.OrderDesc()).ToFunc())
	}
}

// --- mappers ---

func toDomainProduct(e *ent.Product) domain.Product {
	p := domain.Product{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Composition: e.Composition,
		Weight:      e.Weight,
		Barcode:     e.Barcode,
		ImageURL:    e.ImageURL,
		InStock:     e.InStock,
		CategoryID:  e.CategoryID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
	if e.SubcategoryID != nil {
		p.SubcategoryID = e.SubcategoryID
	}
	if e.BrandID != nil {
		p.BrandID = e.BrandID
	}
	// Compute min price and max discount from eagerly loaded in-stock offers.
	for _, o := range e.Edges.Offers {
		price := int64(o.PriceKopecks)
		if p.MinPriceKopecks == nil || price < *p.MinPriceKopecks {
			p.MinPriceKopecks = &price
		}
		dp := o.DiscountPercent
		if p.MaxDiscountPercent == nil || dp > *p.MaxDiscountPercent {
			p.MaxDiscountPercent = &dp
		}
	}
	return p
}

func toDomainProductDetail(e *ent.Product) *domain.ProductDetail {
	d := &domain.ProductDetail{
		Product: toDomainProduct(e),
	}

	if e.Edges.Category != nil {
		d.Category = toDomainCategory(e.Edges.Category)
	}
	if e.Edges.Subcategory != nil {
		sub := toDomainCategory(e.Edges.Subcategory)
		d.Subcategory = &sub
	}
	if e.Edges.Brand != nil {
		b := toDomainBrand(e.Edges.Brand)
		d.Brand = &b
	}
	if e.Edges.Nutrition != nil {
		d.Nutrition = &domain.Nutrition{
			Calories:      e.Edges.Nutrition.Calories,
			Protein:       e.Edges.Nutrition.Protein,
			Fat:           e.Edges.Nutrition.Fat,
			Carbohydrates: e.Edges.Nutrition.Carbohydrates,
		}
	}

	return d
}
