package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent/schema/mixin"
)

// UserDevice stores APNS device metadata for a user.
type UserDevice struct {
	ent.Schema
}

func (UserDevice) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (UserDevice) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			StorageKey("user_id").
			Immutable(),
		field.String("apns_token").NotEmpty(),
		field.String("bundle_id").NotEmpty(),
		field.Enum("environment").Values("sandbox", "production"),
		field.String("app_version").Optional().Nillable(),
	}
}

func (UserDevice) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Required().
			Unique().
			StorageKey(edge.Column("user_id")),
	}
}

func (UserDevice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("apns_token"),
	}
}
