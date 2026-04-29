package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// OrderItem stores snapshot data at the time of order creation.
type OrderItem struct {
	ent.Schema
}

func (OrderItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("order_id", uuid.UUID{}),
		field.UUID("product_id", uuid.UUID{}),
		field.String("product_name"),
		field.UUID("store_id", uuid.UUID{}),
		field.String("store_name"),
		field.Int16("quantity"),
		field.Int64("price_kopecks"),
	}
}

func (OrderItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("order", Order.Type).
			Ref("items").
			Field("order_id").
			Required().
			Unique(),
	}
}

func (OrderItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id"),
	}
}
