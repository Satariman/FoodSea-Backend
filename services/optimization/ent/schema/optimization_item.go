package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// OptimizationItem stores the chosen offer snapshot in an optimization result.
type OptimizationItem struct {
	ent.Schema
}

func (OptimizationItem) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("price_kopecks"),
		field.String("product_name"),
		field.String("store_name"),
		field.Int16("quantity"),
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("product_id", uuid.UUID{}),
		field.UUID("store_id", uuid.UUID{}),
		field.UUID("result_id", uuid.UUID{}),
	}
}

func (OptimizationItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("result", OptimizationResult.Type).
			Ref("items").
			Field("result_id").
			Required().
			Unique(),
	}
}

func (OptimizationItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("result_id"),
	}
}
