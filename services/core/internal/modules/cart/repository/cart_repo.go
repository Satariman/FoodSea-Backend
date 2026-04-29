package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entcart "github.com/foodsea/core/ent/cart"
	entcartitem "github.com/foodsea/core/ent/cartitem"
	entproduct "github.com/foodsea/core/ent/product"
	"github.com/foodsea/core/internal/modules/cart/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type CartRepo struct {
	client *ent.Client
}

func NewCartRepo(client *ent.Client) *CartRepo {
	return &CartRepo{client: client}
}

func (r *CartRepo) GetByUser(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	entCart, err := r.client.Cart.Query().
		Where(entcart.UserID(userID)).
		WithItems(func(q *ent.CartItemQuery) {
			q.WithProduct()
			q.Order(ent.Asc(entcartitem.FieldAddedAt))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			created, createErr := r.client.Cart.Create().
				SetUserID(userID).
				Save(ctx)
			if createErr != nil {
				return nil, fmt.Errorf("creating cart for user: %w", createErr)
			}
			return &domain.Cart{
				ID:        created.ID,
				UserID:    created.UserID,
				Items:     []domain.CartItem{},
				CreatedAt: created.CreatedAt,
				UpdatedAt: created.UpdatedAt,
			}, nil
		}
		return nil, fmt.Errorf("querying cart by user: %w", err)
	}
	return toCart(entCart), nil
}

func (r *CartRepo) AddOrIncrementItem(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	entCart, err := tx.Cart.Query().
		Where(entcart.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			entCart, err = tx.Cart.Create().
				SetUserID(userID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("creating cart: %w", err)
			}
		} else {
			return fmt.Errorf("querying cart: %w", err)
		}
	}

	exists, err := tx.Product.Query().
		Where(entproduct.ID(productID)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("checking product existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("product %s: %w", productID, sherrors.ErrNotFound)
	}

	existing, err := tx.CartItem.Query().
		Where(
			entcartitem.CartID(entCart.ID),
			entcartitem.ProductID(productID),
		).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("querying cart item: %w", err)
	}

	if existing != nil {
		newQty := int16(existing.Quantity) + qty
		if newQty > 99 {
			return fmt.Errorf("total quantity %d exceeds 99: %w", newQty, sherrors.ErrInvalidInput)
		}
		_, err = tx.CartItem.UpdateOneID(existing.ID).
			SetQuantity(int8(newQty)).
			Save(ctx)
	} else {
		_, err = tx.CartItem.Create().
			SetCartID(entCart.ID).
			SetProductID(productID).
			SetQuantity(int8(qty)).
			Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("upserting cart item: %w", err)
	}

	return tx.Commit()
}

func (r *CartRepo) UpdateItemQuantity(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	entCart, err := r.client.Cart.Query().
		Where(entcart.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("cart item: %w", sherrors.ErrNotFound)
		}
		return fmt.Errorf("querying cart: %w", err)
	}

	item, err := r.client.CartItem.Query().
		Where(
			entcartitem.CartID(entCart.ID),
			entcartitem.ProductID(productID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("cart item: %w", sherrors.ErrNotFound)
		}
		return fmt.Errorf("querying cart item: %w", err)
	}

	_, err = r.client.CartItem.UpdateOneID(item.ID).
		SetQuantity(int8(qty)).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("updating cart item quantity: %w", err)
	}
	return nil
}

func (r *CartRepo) RemoveItem(ctx context.Context, userID, productID uuid.UUID) error {
	entCart, err := r.client.Cart.Query().
		Where(entcart.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("querying cart: %w", err)
	}

	_, err = r.client.CartItem.Delete().
		Where(
			entcartitem.CartID(entCart.ID),
			entcartitem.ProductID(productID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("removing cart item: %w", err)
	}
	return nil
}

func (r *CartRepo) Clear(ctx context.Context, userID uuid.UUID) error {
	entCart, err := r.client.Cart.Query().
		Where(entcart.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("querying cart: %w", err)
	}

	_, err = r.client.CartItem.Delete().
		Where(entcartitem.CartID(entCart.ID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("clearing cart items: %w", err)
	}
	return nil
}

func (r *CartRepo) Restore(ctx context.Context, userID uuid.UUID, items []domain.CartItem) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	entCart, err := tx.Cart.Query().
		Where(entcart.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			entCart, err = tx.Cart.Create().
				SetUserID(userID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("creating cart: %w", err)
			}
		} else {
			return fmt.Errorf("querying cart: %w", err)
		}
	}

	_, err = tx.CartItem.Delete().
		Where(entcartitem.CartID(entCart.ID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("clearing cart for restore: %w", err)
	}

	if len(items) > 0 {
		bulk := make([]*ent.CartItemCreate, len(items))
		for i, item := range items {
			bulk[i] = tx.CartItem.Create().
				SetCartID(entCart.ID).
				SetProductID(item.ProductID).
				SetQuantity(int8(item.Quantity))
		}
		_, err = tx.CartItem.CreateBulk(bulk...).Save(ctx)
		if err != nil {
			return fmt.Errorf("bulk inserting cart items: %w", err)
		}
	}

	return tx.Commit()
}

func toCart(e *ent.Cart) *domain.Cart {
	items := make([]domain.CartItem, 0, len(e.Edges.Items))
	for _, ci := range e.Edges.Items {
		item := domain.CartItem{
			ID:        ci.ID,
			ProductID: ci.ProductID,
			Quantity:  int16(ci.Quantity),
			AddedAt:   ci.AddedAt,
		}
		if ci.Edges.Product != nil {
			item.ProductName = ci.Edges.Product.Name
		}
		items = append(items, item)
	}
	return &domain.Cart{
		ID:        e.ID,
		UserID:    e.UserID,
		Items:     items,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}
