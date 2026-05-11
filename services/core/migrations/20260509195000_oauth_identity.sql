-- Make users.password_hash nullable for OAuth-only accounts
ALTER TABLE "users" ALTER COLUMN "password_hash" DROP NOT NULL;

-- Create "oauth_identities" table
CREATE TABLE "oauth_identities" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "provider" character varying NOT NULL,
  "provider_user_id" character varying NOT NULL,
  "email" character varying NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "oauth_identities_users_oauth_identities" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);

-- Create index "oauthidentity_provider_provider_user_id" to table: "oauth_identities"
CREATE UNIQUE INDEX "oauthidentity_provider_provider_user_id" ON "oauth_identities" ("provider", "provider_user_id");
-- Create index "oauthidentity_provider_user_id" to table: "oauth_identities"
CREATE UNIQUE INDEX "oauthidentity_provider_user_id" ON "oauth_identities" ("provider", "user_id");
