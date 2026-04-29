package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent/schema/mixin"
)

// Cart holds the user's shopping cart. One cart per user.
type Cart struct {
	ent.Schema
}

func (Cart) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (Cart) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Unique(),
	}
}

func (Cart) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("cart").
			Field("user_id").
			Required().
			Unique(),
		edge.To("items", CartItem.Type),
	}
}
