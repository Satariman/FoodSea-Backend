package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Offer represents a product's price in a specific store.
type Offer struct {
	ent.Schema
}

func (Offer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("product_id", uuid.UUID{}),
		field.UUID("store_id", uuid.UUID{}),
		field.Int("price_kopecks"),
		field.Int("original_price_kopecks").Optional().Nillable(),
		field.Int8("discount_percent").Default(0),
		field.Bool("in_stock").Default(true),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Offer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("product", Product.Type).
			Ref("offers").
			Field("product_id").
			Required().
			Unique(),
		edge.From("store", Store.Type).
			Ref("offers").
			Field("store_id").
			Required().
			Unique(),
	}
}

func (Offer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("product_id", "store_id").Unique(),
		index.Fields("product_id").
			Annotations(entsql.IndexWhere("in_stock = true")),
		index.Fields("store_id"),
		index.Fields("price_kopecks"),
	}
}
