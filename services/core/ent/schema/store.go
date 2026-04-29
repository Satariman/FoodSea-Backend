package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Store holds the schema definition for a partner store.
type Store struct {
	ent.Schema
}

func (Store) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty().Unique(),
		field.String("logo_url").Optional().Nillable(),
		field.Bool("is_active").Default(true),
		field.Time("created_at").Immutable(),
	}
}

func (Store) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("offers", Offer.Type),
		edge.To("delivery_condition", DeliveryCondition.Type).Unique(),
	}
}
