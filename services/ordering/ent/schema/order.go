package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/ordering/ent/schema/mixin"
)

// Order represents a customer order in ordering_db.
type Order struct {
	ent.Schema
}

func (Order) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (Order) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("optimization_result_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("total_kopecks"),
		field.Int64("delivery_kopecks").Default(0),
		field.String("status").Default("created"),
	}
}

func (Order) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", OrderItem.Type),
		edge.To("history", OrderStatusHistory.Type),
	}
}

func (Order) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("status"),
	}
}

func (Order) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Checks: map[string]string{
				"order_status_check": "status IN ('created', 'confirmed', 'in_delivery', 'delivered', 'cancelled')",
			},
		},
	}
}
