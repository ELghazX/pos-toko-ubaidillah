package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/ELghazX/pos-toko-ubaidillah/internal/env"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {

	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config{
		addr: ":8080",
		db: dbConfig{
			dsn: env.GetString("GOOSE_DBSTRING", "host=localhost user=postgres password=postgres dbname=ecom sslmode=disable"),
		},
	}

	// conn, err := pgx.Connect(ctx, cfg.db.dsn)
	pool, err := pgxpool.New(ctx, cfg.db.dsn)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	logger.Info("Connected to database pool", "dsn", cfg.db.dsn)

	web := application{
		config: cfg,
		db:     pool,
	}

	web.run(web.mount())

}
