package api

import (
	"budgee-server/src/handlers"
	"budgee-server/src/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/v41/plaid"
)

func NewRouter(pool *pgxpool.Pool, plaidClient *plaid.APIClient, plaidEnv string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.CORSMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/login", handlers.Login(pool))
		r.Post("/register", handlers.Register(pool))
		r.Post("/plaid/webhook", handlers.PlaidWebhook(plaidClient, pool))
		if plaidEnv == "sandbox" {
			r.Post("/plaid/sandbox/fire_webhook", handlers.FireSandboxWebhook(plaidClient, pool))
			r.Post("/item/webhook/update", handlers.UpdateItemWebhook(plaidClient, pool))
			r.Post("/plaid/transactions/sync", handlers.SyncTransactionsSandbox(plaidClient, pool))
		}

		// Protected routes
		r.With(middleware.JWTAuthMiddleware).Group(func(r chi.Router) {
			// User
			r.Get("/user/{user_id}", handlers.GetUser(pool))
			r.Put("/user", handlers.UpdateUser(pool))
			r.Post("/user/change-password", handlers.ChangePassword(pool))
			r.Delete("/user", handlers.DeleteUser(pool))

			// Plaid
			r.Post("/plaid/create-link-token", handlers.CreateLinkToken(plaidClient, pool))
			r.Post("/plaid/exchange-public-token", handlers.ExchangePublicToken(plaidClient, pool))
			r.Get("/plaid/items", handlers.GetPlaidItemsFromDB(pool))
			r.Get("/plaid/accounts/{item_id}", handlers.GetPlaidAccounts(plaidClient, pool))
			r.Get("/plaid/accounts/{item_id}/db", handlers.GetAccountsFromDB(pool))
			r.Get("/plaid/transactions/{item_id}/sync", handlers.SyncTransactions(plaidClient, pool))
			r.Get("/plaid/transactions/{account_id}", handlers.GetTransactionsFromDB(pool))
			r.Post("/plaid/transactions", handlers.CreateTransaction(pool))
			r.Delete("/plaid/items/{item_id}", handlers.DeletePlaidItem(plaidClient, pool))
			r.Put("/plaid/transactions/{transaction_id}", handlers.UpdateTransaction(pool))
			r.Delete("/plaid/transactions/{transaction_id}", handlers.DeleteTransaction(pool))

			// Budget
			r.Post("/budgets", handlers.CreateBudget(pool))
			r.Get("/budgets", handlers.GetAllBudgetsForUser(pool))
			r.Get("/budgets/{budget_id}", handlers.GetBudgetByID(pool))
			r.Get("/budgets/category/{category}", handlers.GetBudgetByCategory(pool))
			r.Put("/budgets/{budget_id}", handlers.UpdateBudget(pool))
			r.Delete("/budgets/{budget_id}", handlers.DeleteBudget(pool))

			// Transaction Rules
			r.Post("/transaction-rules", handlers.CreateTransactionRule(pool))
			r.Post("/transaction-rules/trigger", handlers.TriggerTransactionRules(pool))
			r.Get("/transaction-rules", handlers.GetAllTransactionRules(pool))
			r.Get("/transaction-rules/{rule_id}", handlers.GetTransactionRuleByID(pool))
			r.Put("/transaction-rules/{rule_id}", handlers.UpdateTransactionRule(pool))
			r.Delete("/transaction-rules/{rule_id}", handlers.DeleteTransactionRule(pool))
		})

		// Super Admin Routes
		r.With(middleware.JWTAuthMiddleware, middleware.SuperAdminMiddleware).Group(func(r chi.Router) {
			r.Get("/admin/users", handlers.GetAllUsers(pool))
			r.Post("/item/webhook/update-all", handlers.UpdateAllItemWebhooks(plaidClient, pool))
			r.Post("/plaid/transactions/recategorize", handlers.RecategorizeTransactions(plaidClient, pool))
		})
	})

	return r
}
