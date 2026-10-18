package main

import (
	"context"
	"log"
	"net/http"
	"time"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/auth"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/categories"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/customers"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/dashboard"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/debts"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/products"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/purchases"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/sales"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/settings"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/suppliers"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/whatsapp"
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

	queries := repo.New(app.db)

	authSvc := auth.NewService(queries)
	if err := authSvc.EnsureDefaultUsers(context.Background()); err != nil {
		log.Printf("Peringatan: Gagal menginisialisasi akun default: %v", err)
	}
	authHandler := auth.NewHandler(authSvc)
	authHandler.RegisterPublicRoutes(r)

	dashboardSvc := dashboard.NewService(queries, app.db)
	dashboardHandler := dashboard.NewHandler(dashboardSvc)

	categoriesHandler := categories.NewHandler(categories.NewService(queries))

	productSvc := products.NewService(queries, app.db)
	productHandler := products.NewHandler(productSvc, queries)

	suppliersHandler := suppliers.NewHandler(suppliers.NewService(queries))
	customersHandler := customers.NewHandler(customers.NewService(queries))

	whatsAppSvc, err := whatsapp.NewService(app.config.db.dsn)
	var whatsAppHandler *whatsapp.Handler
	if err != nil {
		log.Printf("Peringatan: Gagal menginisialisasi layanan WhatsApp: %v", err)
	} else {
		whatsAppHandler = whatsapp.NewHandler(whatsAppSvc)
	}

	purchaseSvc := purchases.NewService(queries, app.db, whatsAppSvc)
	purchaseHandler := purchases.NewHandler(purchaseSvc)

	salesSvc := sales.NewService(queries, app.db, whatsAppSvc)
	salesHandler := sales.NewHandler(salesSvc)

	debtsSvc := debts.NewService(queries, app.db, whatsAppSvc)
	debtsHandler := debts.NewHandler(debtsSvc)

	settingsHandler := settings.NewHandler(settings.NewService(queries))

	r.Group(func(authRouter chi.Router) {
		authRouter.Use(auth.RequireAuth(authSvc))

		authRouter.Get("/", authHandler.HandleRootRedirect)

		salesHandler.RegisterRoutes(authRouter)
		debtsHandler.RegisterRoutes(authRouter)
		customersHandler.RegisterRoutes(authRouter)

		authRouter.Group(func(adminRouter chi.Router) {
			adminRouter.Use(auth.RequireAdmin)

			dashboardHandler.RegisterRoutes(adminRouter)
			categoriesHandler.RegisterRoutes(adminRouter)
			productHandler.RegisterRoutes(adminRouter)
			suppliersHandler.RegisterRoutes(adminRouter)
			purchaseHandler.RegisterRoutes(adminRouter)
			settingsHandler.RegisterRoutes(adminRouter)
			authHandler.RegisterAdminRoutes(adminRouter)
			if whatsAppHandler != nil {
				whatsAppHandler.RegisterRoutes(adminRouter)
			}
		})
	})

	return r
}

type config struct {
	addr string
	db   dbConfig
}

type dbConfig struct {
	dsn string
}
