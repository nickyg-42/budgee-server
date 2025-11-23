package handlers

import (
	db "budgee-server/src/db/sql"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/plaid"
)

func CreateLinkToken(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		user := plaid.LinkTokenCreateRequestUser{
			ClientUserId: strconv.FormatInt(userID, 10),
		}
		request := plaid.NewLinkTokenCreateRequest(
			"Budgee",
			"en",
			[]plaid.CountryCode{plaid.COUNTRYCODE_US},
			user,
		)
		request.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
		resp, _, err := plaidClient.PlaidApi.LinkTokenCreate(context.Background()).LinkTokenCreateRequest(*request).Execute()
		if err != nil {
			http.Error(w, "Failed to create link token", http.StatusInternalServerError)
			log.Printf("ERROR: Plaid link token creation failed for user %d: %v", userID, err)
			return
		}

		linkToken := resp.GetLinkToken()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(linkToken)
	}
}

func ExchangePublicToken(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		publicToken := chi.URLParam(r, "public_token")

		exchangePublicTokenReq := plaid.NewItemPublicTokenExchangeRequest(publicToken)
		exchangePublicTokenResp, _, err := plaidClient.PlaidApi.ItemPublicTokenExchange(context.Background()).ItemPublicTokenExchangeRequest(
			*exchangePublicTokenReq,
		).Execute()

		if err != nil {
			http.Error(w, "Failed to exchange public token", http.StatusInternalServerError)
			log.Printf("ERROR: Plaid public token exchange failed: %v", err)
			return
		}

		accessToken := exchangePublicTokenResp.GetAccessToken()
		itemID := exchangePublicTokenResp.GetItemId()

		err = db.SavePlaidItem(r.Context(), pool, userID, itemID, accessToken)
		if err != nil {
			http.Error(w, "Failed to save plaid item", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to save plaid item for user %d: %v", userID, err)
			return
		}

		// FIXME: THIS IS WRONG, why DB lookup right after save?
		// Fetch the saved item to return to client
		items, err := db.GetPlaidItemsSQL(r.Context(), pool, userID)
		if err != nil || len(items) == 0 {
			http.Error(w, "Failed to retrieve saved item", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to retrieve saved plaid item for user %d: %v", userID, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(items[len(items)-1])
	}
}

func GetPlaidAccounts(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		itemID := chi.URLParam(r, "item_id")

		query := `SELECT access_token FROM plaid_items WHERE user_id = $1 AND id = $2`
		var accessToken string
		err := pool.QueryRow(r.Context(), query, userID, itemID).Scan(&accessToken)
		if err != nil {
			http.Error(w, "Access token not found", http.StatusNotFound)
			log.Printf("ERROR: Failed to get access token for user %d, item %s: %v", userID, itemID, err)
			return
		}

		request := plaid.NewAccountsGetRequest(accessToken)
		accountsResp, _, err := plaidClient.PlaidApi.AccountsGet(context.Background()).AccountsGetRequest(*request).Execute()
		if err != nil {
			http.Error(w, "Failed to fetch accounts from Plaid", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to fetch accounts for user %d, item %s: %v", userID, itemID, err)
			return
		}

		err = db.SaveAccounts(r.Context(), pool, userID, accountsResp.GetAccounts())
		if err != nil {
			http.Error(w, "Failed to save accounts", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to save accounts for user %d: %v", userID, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accountsResp.GetAccounts())
	}
}

func SyncTransactions(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		itemID := chi.URLParam(r, "item_id")

		query := `SELECT id, access_token FROM plaid_items WHERE user_id = $1 AND id = $2`
		var dbItemID int64
		var accessToken string
		err := pool.QueryRow(r.Context(), query, userID, itemID).Scan(&dbItemID, &accessToken)
		if err != nil {
			http.Error(w, "Access token not found", http.StatusNotFound)
			log.Printf("ERROR: Failed to get access token for user %d, item %s: %v", userID, itemID, err)
			return
		}

		cursor, err := db.GetSyncCursor(r.Context(), pool, dbItemID)
		if err != nil {
			http.Error(w, "Failed to retrieve sync cursor", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to get sync cursor for item %d: %v", dbItemID, err)
			return
		}

		request := plaid.NewTransactionsSyncRequest(accessToken)
		if cursor != "" {
			request.SetCursor(cursor)
		}

		transactionsResp, _, err := plaidClient.PlaidApi.TransactionsSync(context.Background()).TransactionsSyncRequest(*request).Execute()
		if err != nil {
			http.Error(w, "Failed to fetch transactions", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to sync transactions for user %d, item %d: %v", userID, dbItemID, err)
			return
		}

		err = db.SaveTransactions(r.Context(), pool, userID, transactionsResp.GetAdded())
		if err != nil {
			http.Error(w, "Failed to save transactions", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to save transactions for user %d: %v", userID, err)
			return
		}

		err = db.UpdateSyncCursor(r.Context(), pool, dbItemID, transactionsResp.GetNextCursor())
		if err != nil {
			http.Error(w, "Failed to update sync cursor", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to update sync cursor for item %d: %v", dbItemID, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(transactionsResp)
	}
}

func GetPlaidItemsFromDB(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		items, err := db.GetPlaidItemsSQL(r.Context(), pool, userID)
		if err != nil {
			http.Error(w, "Failed to retrieve plaid items", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to get plaid items for user %d: %v", userID, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}

func GetAccountsFromDB(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		itemID := chi.URLParam(r, "item_id")

		accounts, err := db.GetAccountsSQL(r.Context(), pool, userID, itemID)
		if err != nil {
			http.Error(w, "Failed to retrieve accounts", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to get accounts for user %d, item %s: %v", userID, itemID, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accounts)
	}
}

func GetTransactionsFromDB(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		accountID := chi.URLParam(r, "account_id")

		transactions, err := db.GetTransactionsSQL(r.Context(), pool, userID, accountID)
		if err != nil {
			http.Error(w, "Failed to retrieve transactions", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to get transactions for user %d, account %s: %v", userID, accountID, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(transactions)
	}
}
