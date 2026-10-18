package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ELghazX/pos-toko-ubaidillah/internal/env"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {

	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config{
		addr: ":8080",
		db: dbConfig{
			dsn: env.GetString("", "host=localhost user=postgres password=postgres dbname=pos sslmode=disable"),
		},
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		panic(err)
	}
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, offsetSec := time.Now().Zone()
		offsetHours := offsetSec / 3600
		_, err := conn.Exec(ctx, fmt.Sprintf("SET TIME ZONE %d", offsetHours))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	logger.Info("Connected to database pool", "dsn", cfg.db.dsn)

	web := application{
		config: cfg,
		db:     pool,
	}

	err = web.run(web.mount())
	if err != nil {
		panic(err)
	}

}
