package api

import (
	"budgee-server/src/handlers"
	"budgee-server/src/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/plaid"
)

func NewRouter(pool *pgxpool.Pool, plaidClient *plaid.APIClient) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.CORSMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/login", handlers.Login(pool))
		r.Post("/register", handlers.Register(pool))

		// JWT required routes
		r.With(middleware.JWTAuthMiddleware).Group(func(r chi.Router) {
			r.Get("/user/{user_id}", handlers.GetUser(pool))
			r.Put("/user", handlers.UpdateUser(pool))
			r.Post("/user/change-password", handlers.ChangePassword(pool))
			r.Delete("/user", handlers.DeleteUser(pool))

			r.Post("/plaid/create-link-token", handlers.CreateLinkToken(plaidClient, pool))
			r.Post("/plaid/exchange-public-token", handlers.ExchangePublicToken(plaidClient, pool))
			r.Get("/plaid/items", handlers.GetPlaidItemsFromDB(pool))
			r.Get("/plaid/accounts/{item_id}", handlers.GetPlaidAccounts(plaidClient, pool))
			r.Get("/plaid/accounts/{item_id}/db", handlers.GetAccountsFromDB(pool))
			r.Get("/plaid/transactions/{item_id}/sync", handlers.SyncTransactions(plaidClient, pool))
			r.Get("/plaid/transactions/{account_id}", handlers.GetTransactionsFromDB(pool))
			r.Delete("/plaid/items/{item_id}", handlers.DeletePlaidItem(plaidClient, pool))
			r.Put("/plaid/transactions/{transaction_id}", handlers.UpdateTransaction(pool))
			r.Delete("/plaid/transactions/{transaction_id}", handlers.DeleteTransaction(pool))
		})
	})

	return r
}
