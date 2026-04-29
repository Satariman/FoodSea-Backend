package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// OrderStatusHistory tracks every status transition of an order.
type OrderStatusHistory struct {
	ent.Schema
}

func (OrderStatusHistory) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("order_id", uuid.UUID{}),
		field.String("status"),
		field.String("comment").Optional().Nillable(),
		field.Time("changed_at").Default(time.Now).Immutable(),
	}
}

func (OrderStatusHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("order", Order.Type).
			Ref("history").
			Field("order_id").
			Required().
			Unique(),
	}
}

func (OrderStatusHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id", "changed_at"),
	}
}
