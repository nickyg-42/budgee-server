package api

import (
	"budgee-server/src/handlers"
	"budgee-server/src/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Use(middleware.CORSMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Post("/login", handlers.Login(pool))
		r.Post("/register", handlers.Register(pool))

		// JWT required routes
		r.With(middleware.JWTAuthMiddleware).Group(func(r chi.Router) {

		})
	})

	return r
}
