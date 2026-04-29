package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ProductNutrition holds nutritional values per 100g/ml.
type ProductNutrition struct {
	ent.Schema
}

func (ProductNutrition) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("product_id", uuid.UUID{}),
		field.Float("calories").
			SchemaType(map[string]string{dialect.Postgres: "decimal(7,2)"}),
		field.Float("protein").
			SchemaType(map[string]string{dialect.Postgres: "decimal(7,2)"}),
		field.Float("fat").
			SchemaType(map[string]string{dialect.Postgres: "decimal(7,2)"}),
		field.Float("carbohydrates").
			SchemaType(map[string]string{dialect.Postgres: "decimal(7,2)"}),
	}
}

func (ProductNutrition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("product", Product.Type).
			Ref("nutrition").
			Field("product_id").
			Required().
			Unique(),
	}
}
