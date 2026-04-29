-- Create "users" table
CREATE TABLE "users" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "phone" character varying NULL,
  "email" character varying NULL,
  "password_hash" character varying NOT NULL,
  "onboarding_done" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id")
);
-- Create index "users_email_key" to table: "users"
CREATE UNIQUE INDEX "users_email_key" ON "users" ("email");
-- Create index "users_phone_key" to table: "users"
CREATE UNIQUE INDEX "users_phone_key" ON "users" ("phone");
-- Create "carts" table
CREATE TABLE "carts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "carts_users_cart" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "carts_user_id_key" to table: "carts"
CREATE UNIQUE INDEX "carts_user_id_key" ON "carts" ("user_id");
-- Create "brands" table
CREATE TABLE "brands" (
  "id" uuid NOT NULL,
  "name" character varying NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "brands_name_key" to table: "brands"
CREATE UNIQUE INDEX "brands_name_key" ON "brands" ("name");
-- Create "categories" table
CREATE TABLE "categories" (
  "id" uuid NOT NULL,
  "name" character varying NOT NULL,
  "slug" character varying NOT NULL,
  "sort_order" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL,
  "parent_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "categories_categories_children" FOREIGN KEY ("parent_id") REFERENCES "categories" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "categories_slug_key" to table: "categories"
CREATE UNIQUE INDEX "categories_slug_key" ON "categories" ("slug");
-- Create index "category_parent_id" to table: "categories"
CREATE INDEX "category_parent_id" ON "categories" ("parent_id");
-- Create index "category_slug" to table: "categories"
CREATE UNIQUE INDEX "category_slug" ON "categories" ("slug");
-- Create "products" table
CREATE TABLE "products" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "description" text NULL,
  "composition" text NULL,
  "weight" character varying NULL,
  "barcode" character varying NULL,
  "image_url" character varying NULL,
  "in_stock" boolean NOT NULL DEFAULT true,
  "brand_id" uuid NULL,
  "category_id" uuid NOT NULL,
  "subcategory_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "products_brands_products" FOREIGN KEY ("brand_id") REFERENCES "brands" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "products_categories_products" FOREIGN KEY ("category_id") REFERENCES "categories" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "products_categories_subcategory_products" FOREIGN KEY ("subcategory_id") REFERENCES "categories" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "product_barcode" to table: "products"
CREATE UNIQUE INDEX "product_barcode" ON "products" ("barcode") WHERE (barcode IS NOT NULL);
-- Create index "product_brand_id" to table: "products"
CREATE INDEX "product_brand_id" ON "products" ("brand_id") WHERE (brand_id IS NOT NULL);
-- Create index "product_category_id" to table: "products"
CREATE INDEX "product_category_id" ON "products" ("category_id");
-- Create index "product_in_stock" to table: "products"
CREATE INDEX "product_in_stock" ON "products" ("in_stock") WHERE (in_stock = true);
-- Create index "product_subcategory_id" to table: "products"
CREATE INDEX "product_subcategory_id" ON "products" ("subcategory_id") WHERE (subcategory_id IS NOT NULL);
-- Create index "products_barcode_key" to table: "products"
CREATE UNIQUE INDEX "products_barcode_key" ON "products" ("barcode");
-- Create "cart_items" table
CREATE TABLE "cart_items" (
  "id" uuid NOT NULL,
  "quantity" smallint NOT NULL,
  "added_at" timestamptz NOT NULL,
  "cart_id" uuid NOT NULL,
  "product_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "cart_items_carts_items" FOREIGN KEY ("cart_id") REFERENCES "carts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "cart_items_products_cart_items" FOREIGN KEY ("product_id") REFERENCES "products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "cartitem_cart_id_product_id" to table: "cart_items"
CREATE UNIQUE INDEX "cartitem_cart_id_product_id" ON "cart_items" ("cart_id", "product_id");
-- Create "stores" table
CREATE TABLE "stores" (
  "id" uuid NOT NULL,
  "name" character varying NOT NULL,
  "slug" character varying NOT NULL,
  "logo_url" character varying NULL,
  "is_active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "stores_slug_key" to table: "stores"
CREATE UNIQUE INDEX "stores_slug_key" ON "stores" ("slug");
-- Create "delivery_conditions" table
CREATE TABLE "delivery_conditions" (
  "id" uuid NOT NULL,
  "min_order_kopecks" bigint NOT NULL DEFAULT 0,
  "delivery_cost_kopecks" bigint NOT NULL DEFAULT 0,
  "free_from_kopecks" bigint NULL,
  "estimated_minutes" bigint NULL,
  "store_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "delivery_conditions_stores_delivery_condition" FOREIGN KEY ("store_id") REFERENCES "stores" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "delivery_conditions_store_id_key" to table: "delivery_conditions"
CREATE UNIQUE INDEX "delivery_conditions_store_id_key" ON "delivery_conditions" ("store_id");
-- Create "offers" table
CREATE TABLE "offers" (
  "id" uuid NOT NULL,
  "price_kopecks" bigint NOT NULL,
  "original_price_kopecks" bigint NULL,
  "discount_percent" smallint NOT NULL DEFAULT 0,
  "in_stock" boolean NOT NULL DEFAULT true,
  "updated_at" timestamptz NOT NULL,
  "product_id" uuid NOT NULL,
  "store_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "offers_products_offers" FOREIGN KEY ("product_id") REFERENCES "products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "offers_stores_offers" FOREIGN KEY ("store_id") REFERENCES "stores" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "offer_price_kopecks" to table: "offers"
CREATE INDEX "offer_price_kopecks" ON "offers" ("price_kopecks");
-- Create index "offer_product_id" to table: "offers"
CREATE INDEX "offer_product_id" ON "offers" ("product_id") WHERE (in_stock = true);
-- Create index "offer_product_id_store_id" to table: "offers"
CREATE UNIQUE INDEX "offer_product_id_store_id" ON "offers" ("product_id", "store_id");
-- Create index "offer_store_id" to table: "offers"
CREATE INDEX "offer_store_id" ON "offers" ("store_id");
-- Create "product_nutritions" table
CREATE TABLE "product_nutritions" (
  "id" uuid NOT NULL,
  "calories" numeric(7,2) NOT NULL,
  "protein" numeric(7,2) NOT NULL,
  "fat" numeric(7,2) NOT NULL,
  "carbohydrates" numeric(7,2) NOT NULL,
  "product_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "product_nutritions_products_nutrition" FOREIGN KEY ("product_id") REFERENCES "products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "product_nutritions_product_id_key" to table: "product_nutritions"
CREATE UNIQUE INDEX "product_nutritions_product_id_key" ON "product_nutritions" ("product_id");
