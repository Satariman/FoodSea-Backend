-- Create "user_devices" table
CREATE TABLE "user_devices" (
  "user_id" uuid NOT NULL,
  "apns_token" text NOT NULL,
  "bundle_id" text NOT NULL,
  "environment" text NOT NULL,
  "app_version" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("user_id"),
  CONSTRAINT "user_devices_environment_check" CHECK ("environment" IN ('sandbox', 'production')),
  CONSTRAINT "user_devices_users_user_devices" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

-- Create index "userdevice_apns_token" to table: "user_devices"
CREATE INDEX "userdevice_apns_token" ON "user_devices" ("apns_token");

-- Create "order_live_activities" table
CREATE TABLE "order_live_activities" (
  "order_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "push_token" text NOT NULL,
  "bundle_id" text NOT NULL,
  "environment" text NOT NULL,
  "started_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("order_id"),
  CONSTRAINT "order_live_activities_environment_check" CHECK ("environment" IN ('sandbox', 'production')),
  CONSTRAINT "order_live_activities_users_order_live_activities" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

-- Create index "orderliveactivity_user_id" to table: "order_live_activities"
CREATE INDEX "orderliveactivity_user_id" ON "order_live_activities" ("user_id");
