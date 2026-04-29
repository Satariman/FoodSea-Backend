package mixin

import (
	"context"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	entmixin "entgo.io/ent/schema/mixin"
)

// Timestamps adds created_at (immutable) and updated_at fields with an auto-update hook.
type Timestamps struct {
	entmixin.Schema
}

func (Timestamps) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Timestamps) Hooks() []ent.Hook {
	return []ent.Hook{
		func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				if mx, ok := m.(interface{ SetUpdatedAt(time.Time) }); ok {
					mx.SetUpdatedAt(time.Now())
				}
				return next.Mutate(ctx, m)
			})
		},
	}
}
