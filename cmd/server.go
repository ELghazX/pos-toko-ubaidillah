package main

import (
	"log"
	"net/http"
	"time"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/categories"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	config config
	db     *pgxpool.Pool
}

func (app *application) run(h http.Handler) error {

	server := &http.Server{
		Addr:         app.config.addr,
		Handler:      h,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	log.Printf("Server started at %s", server.Addr)
	return server.ListenAndServe()
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Healthy"))
	})

	categoriesHandler := categories.NewHandler(categories.NewService(repo.New(app.db)))
	r.Get("/categories", categoriesHandler.ListCategories)
	r.Delete("/categories/{id}", categoriesHandler.DeleteCategory)

	return r

}

type config struct {
	addr string
	db   dbConfig
}

type dbConfig struct {
	dsn string
}
