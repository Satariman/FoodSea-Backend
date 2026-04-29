package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// DeliveryCondition holds store delivery terms.
type DeliveryCondition struct {
	ent.Schema
}

func (DeliveryCondition) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("store_id", uuid.UUID{}),
		field.Int("min_order_kopecks").Default(0),
		field.Int("delivery_cost_kopecks").Default(0),
		field.Int("free_from_kopecks").Optional().Nillable(),
		field.Int("estimated_minutes").Optional().Nillable(),
	}
}

func (DeliveryCondition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("store", Store.Type).
			Ref("delivery_condition").
			Field("store_id").
			Required().
			Unique(),
	}
}
