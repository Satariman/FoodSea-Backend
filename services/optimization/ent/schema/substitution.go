package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Substitution stores a snapshot for an analog replacement proposal.
type Substitution struct {
	ent.Schema
}

func (Substitution) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("total_saving_kopecks"),
		field.Int64("price_delta_kopecks"),
		field.Int64("delivery_delta_kopecks"),
		field.Int64("old_price_kopecks"),
		field.Int64("new_price_kopecks"),
		field.String("analog_product_name"),
		field.String("new_store_name"),
		field.String("original_product_name"),
		field.Float("score").SchemaType(map[string]string{dialect.Postgres: "decimal(5,4)"}),
		field.UUID("original_store_id", uuid.UUID{}),
		field.UUID("original_product_id", uuid.UUID{}),
		field.UUID("analog_product_id", uuid.UUID{}),
		field.UUID("new_store_id", uuid.UUID{}),
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Bool("is_cross_store"),
		field.UUID("result_id", uuid.UUID{}),
	}
}

func (Substitution) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("result", OptimizationResult.Type).
			Ref("substitutions").
			Field("result_id").
			Required().
			Unique(),
	}
}

func (Substitution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("result_id"),
	}
}
