package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/optimization/ent/schema/mixin"
)

// OptimizationResult stores an optimization snapshot for a user's cart hash.
type OptimizationResult struct {
	ent.Schema
}

func (OptimizationResult) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Timestamps{}}
}

func (OptimizationResult) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("total_kopecks"),
		field.Int64("delivery_kopecks").Default(0),
		field.Int64("savings_kopecks").Default(0),
		field.String("cart_hash"),
		field.String("status").Default("active"),
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("user_id", uuid.UUID{}),
		field.Bool("is_approximate").Default(false),
	}
}

func (OptimizationResult) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", OptimizationItem.Type),
		edge.To("substitutions", Substitution.Type),
	}
}

func (OptimizationResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at").Annotations(entsql.DescColumns("created_at")),
		index.Fields("cart_hash"),
		index.Fields("status"),
	}
}

func (OptimizationResult) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Checks: map[string]string{
				"optimization_result_status_check": "status IN ('active', 'locked', 'expired')",
			},
		},
	}
}
