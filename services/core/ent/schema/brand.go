package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Brand holds the schema definition for the Brand entity.
type Brand struct {
	ent.Schema
}

func (Brand) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("name").NotEmpty().Unique(),
		field.Time("created_at").Immutable(),
	}
}

func (Brand) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("products", Product.Type),
	}
}
