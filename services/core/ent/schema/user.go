package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent/schema/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("phone").Optional().Nillable().Unique(),
		field.String("email").Optional().Nillable().Unique(),
		field.String("password_hash").NotEmpty(),
		field.Bool("onboarding_done").Default(false),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("cart", Cart.Type).Unique(),
	}
}
