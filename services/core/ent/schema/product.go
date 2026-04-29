package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent/schema/mixin"
)

// Product holds the schema definition for the Product entity.
type Product struct {
	ent.Schema
}

func (Product) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Timestamps{},
	}
}

func (Product) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("name").NotEmpty().MaxLen(255),
		field.Text("description").Optional().Nillable(),
		field.Text("composition").Optional().Nillable(),
		field.String("weight").Optional().Nillable(),
		field.String("barcode").Optional().Nillable().Unique(),
		field.String("image_url").Optional().Nillable(),
		field.Bool("in_stock").Default(true),
		field.UUID("category_id", uuid.UUID{}),
		field.UUID("subcategory_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("brand_id", uuid.UUID{}).Optional().Nillable(),
	}
}

func (Product) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("category", Category.Type).
			Ref("products").
			Field("category_id").
			Required().
			Unique(),
		edge.From("subcategory", Category.Type).
			Ref("subcategory_products").
			Field("subcategory_id").
			Unique(),
		edge.From("brand", Brand.Type).
			Ref("products").
			Field("brand_id").
			Unique(),
		edge.To("nutrition", ProductNutrition.Type).Unique(),
		edge.To("offers", Offer.Type),
		edge.To("cart_items", CartItem.Type),
	}
}

func (Product) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("barcode").
			Unique().
			Annotations(entsql.IndexWhere("barcode IS NOT NULL")),
		index.Fields("category_id"),
		index.Fields("subcategory_id").
			Annotations(entsql.IndexWhere("subcategory_id IS NOT NULL")),
		index.Fields("brand_id").
			Annotations(entsql.IndexWhere("brand_id IS NOT NULL")),
		index.Fields("in_stock").
			Annotations(entsql.IndexWhere("in_stock = true")),
	}
}
