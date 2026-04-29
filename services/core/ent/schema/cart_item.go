package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CartItem represents a product entry inside a cart.
type CartItem struct {
	ent.Schema
}

func (CartItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("cart_id", uuid.UUID{}),
		field.UUID("product_id", uuid.UUID{}),
		field.Int8("quantity").Min(1).Max(99),
		field.Time("added_at").Default(time.Now).Immutable(),
	}
}

func (CartItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("cart", Cart.Type).
			Ref("items").
			Field("cart_id").
			Required().
			Unique(),
		edge.From("product", Product.Type).
			Ref("cart_items").
			Field("product_id").
			Required().
			Unique(),
	}
}

func (CartItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("cart_id", "product_id").Unique(),
	}
}
