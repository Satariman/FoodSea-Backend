package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/foodsea/ordering/ent"
	"github.com/foodsea/ordering/internal/platform/config"
)

// Open connects to PostgreSQL via the pgx driver and returns a ready Ent client
// alongside the underlying *sql.DB.
func Open(ctx context.Context, cfg config.DatabaseConfig, log *slog.Logger) (*ent.Client, *sql.DB, error) {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("pinging database: %w", err)
	}

	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))

	log.InfoContext(ctx, "database connected", "url_masked", maskURL(cfg.URL))
	return client, db, nil
}

// maskURL hides the password in a connection string for logging.
func maskURL(url string) string {
	for i := 0; i < len(url); i++ {
		if url[i] == ':' && i+1 < len(url) {
			for j := i + 1; j < len(url); j++ {
				if url[j] == '@' {
					return url[:i+1] + "***" + url[j:]
				}
			}
		}
	}
	return url
}
