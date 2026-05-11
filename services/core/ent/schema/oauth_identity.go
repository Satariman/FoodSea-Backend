package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent/schema/mixin"
)

// OAuthIdentity stores external provider identity bindings for users.
type OAuthIdentity struct {
	ent.Schema
}

func (OAuthIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (OAuthIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("provider").NotEmpty(),
		field.String("provider_user_id").NotEmpty(),
		field.String("email").Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}),
	}
}

func (OAuthIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("oauth_identities").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (OAuthIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_user_id").Unique(),
		index.Fields("provider", "user_id").Unique(),
	}
}
