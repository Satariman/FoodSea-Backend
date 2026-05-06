-- Create "optimization_results" table
CREATE TABLE "optimization_results" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "total_kopecks" bigint NOT NULL,
  "delivery_kopecks" bigint NOT NULL DEFAULT 0,
  "savings_kopecks" bigint NOT NULL DEFAULT 0,
  "cart_hash" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'active',
  "user_id" uuid NOT NULL,
  "is_approximate" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "optimization_result_status_check" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'locked'::character varying, 'expired'::character varying])::text[]))
);
-- Create index "optimizationresult_cart_hash" to table: "optimization_results"
CREATE INDEX "optimizationresult_cart_hash" ON "optimization_results" ("cart_hash");
-- Create index "optimizationresult_status" to table: "optimization_results"
CREATE INDEX "optimizationresult_status" ON "optimization_results" ("status");
-- Create index "optimizationresult_user_id_created_at" to table: "optimization_results"
CREATE INDEX "optimizationresult_user_id_created_at" ON "optimization_results" ("user_id", "created_at" DESC);
-- Create "optimization_items" table
CREATE TABLE "optimization_items" (
  "id" uuid NOT NULL,
  "price_kopecks" bigint NOT NULL,
  "product_name" character varying NOT NULL,
  "store_name" character varying NOT NULL,
  "quantity" smallint NOT NULL,
  "product_id" uuid NOT NULL,
  "store_id" uuid NOT NULL,
  "result_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "optimization_items_optimization_results_items" FOREIGN KEY ("result_id") REFERENCES "optimization_results" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "optimizationitem_result_id" to table: "optimization_items"
CREATE INDEX "optimizationitem_result_id" ON "optimization_items" ("result_id");
-- Create "substitutions" table
CREATE TABLE "substitutions" (
  "id" uuid NOT NULL,
  "total_saving_kopecks" bigint NOT NULL,
  "price_delta_kopecks" bigint NOT NULL,
  "delivery_delta_kopecks" bigint NOT NULL,
  "old_price_kopecks" bigint NOT NULL,
  "new_price_kopecks" bigint NOT NULL,
  "analog_product_name" character varying NOT NULL,
  "new_store_name" character varying NOT NULL,
  "original_product_name" character varying NOT NULL,
  "score" numeric(5,4) NOT NULL,
  "original_store_id" uuid NOT NULL,
  "original_product_id" uuid NOT NULL,
  "analog_product_id" uuid NOT NULL,
  "new_store_id" uuid NOT NULL,
  "is_cross_store" boolean NOT NULL,
  "result_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "substitutions_optimization_results_substitutions" FOREIGN KEY ("result_id") REFERENCES "optimization_results" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "substitution_result_id" to table: "substitutions"
CREATE INDEX "substitution_result_id" ON "substitutions" ("result_id");
