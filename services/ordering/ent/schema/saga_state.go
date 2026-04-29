package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/ordering/ent/schema/mixin"
)

// SagaState tracks the distributed saga orchestration lifecycle.
type SagaState struct {
	ent.Schema
}

func (SagaState) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (SagaState) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("order_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Int8("current_step").Default(0),
		field.String("status").Default("pending"),
		field.JSON("payload", map[string]any{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
			}),
	}
}

func (SagaState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id"),
		index.Fields("status"),
	}
}

func (SagaState) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Checks: map[string]string{
				"saga_status_check": "status IN ('pending', 'completed', 'compensating', 'failed')",
			},
		},
	}
}
