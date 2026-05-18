package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// OrderLiveActivity stores APNS Live Activity token bindings for orders.
type OrderLiveActivity struct {
	ent.Schema
}

func (OrderLiveActivity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			StorageKey("order_id").
			Immutable(),
		field.UUID("user_id", uuid.UUID{}),
		field.String("push_token").NotEmpty(),
		field.String("bundle_id").NotEmpty(),
		field.Enum("environment").Values("sandbox", "production"),
		field.Time("started_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (OrderLiveActivity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Required().
			Unique(),
	}
}

func (OrderLiveActivity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
