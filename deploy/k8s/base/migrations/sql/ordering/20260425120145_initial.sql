-- Create "orders" table
CREATE TABLE "orders" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "user_id" uuid NOT NULL,
  "optimization_result_id" uuid NULL,
  "total_kopecks" bigint NOT NULL,
  "delivery_kopecks" bigint NOT NULL DEFAULT 0,
  "status" character varying NOT NULL DEFAULT 'created',
  PRIMARY KEY ("id"),
  CONSTRAINT "order_status_check" CHECK ((status)::text = ANY ((ARRAY['created'::character varying, 'confirmed'::character varying, 'in_delivery'::character varying, 'delivered'::character varying, 'cancelled'::character varying])::text[]))
);
-- Create index "order_status" to table: "orders"
CREATE INDEX "order_status" ON "orders" ("status");
-- Create index "order_user_id_created_at" to table: "orders"
CREATE INDEX "order_user_id_created_at" ON "orders" ("user_id", "created_at");
-- Create "saga_states" table
CREATE TABLE "saga_states" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "order_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "current_step" smallint NOT NULL DEFAULT 0,
  "status" character varying NOT NULL DEFAULT 'pending',
  "payload" jsonb NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "saga_status_check" CHECK ((status)::text = ANY ((ARRAY['pending'::character varying, 'completed'::character varying, 'compensating'::character varying, 'failed'::character varying])::text[]))
);
-- Create index "sagastate_order_id" to table: "saga_states"
CREATE INDEX "sagastate_order_id" ON "saga_states" ("order_id");
-- Create index "sagastate_status" to table: "saga_states"
CREATE INDEX "sagastate_status" ON "saga_states" ("status");
-- Create "order_items" table
CREATE TABLE "order_items" (
  "id" uuid NOT NULL,
  "product_id" uuid NOT NULL,
  "product_name" character varying NOT NULL,
  "store_id" uuid NOT NULL,
  "store_name" character varying NOT NULL,
  "quantity" smallint NOT NULL,
  "price_kopecks" bigint NOT NULL,
  "order_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "order_items_orders_items" FOREIGN KEY ("order_id") REFERENCES "orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "orderitem_order_id" to table: "order_items"
CREATE INDEX "orderitem_order_id" ON "order_items" ("order_id");
-- Create "order_status_histories" table
CREATE TABLE "order_status_histories" (
  "id" uuid NOT NULL,
  "status" character varying NOT NULL,
  "comment" character varying NULL,
  "changed_at" timestamptz NOT NULL,
  "order_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "order_status_histories_orders_history" FOREIGN KEY ("order_id") REFERENCES "orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "orderstatushistory_order_id_changed_at" to table: "order_status_histories"
CREATE INDEX "orderstatushistory_order_id_changed_at" ON "order_status_histories" ("order_id", "changed_at");
