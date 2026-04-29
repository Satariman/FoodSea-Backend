package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Category holds the schema definition for the Category entity.
// Supports two-level hierarchy: category (parent_id=NULL) and subcategory (parent_id → category).
type Category struct {
	ent.Schema
}

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty().Unique(),
		field.Int("sort_order").Default(0),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Immutable(),
	}
}

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Category.Type),
		edge.From("parent", Category.Type).
			Ref("children").
			Field("parent_id").
			Unique(),
		edge.To("products", Product.Type),
		edge.To("subcategory_products", Product.Type),
	}
}

func (Category) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").Unique(),
		index.Fields("parent_id"),
	}
}
