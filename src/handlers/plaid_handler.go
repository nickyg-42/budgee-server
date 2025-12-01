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

		var req struct {
			PublicToken string `json:"public_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode exchange public token request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		exchangePublicTokenReq := plaid.NewItemPublicTokenExchangeRequest(req.PublicToken)
		exchangePublicTokenResp, _, err := plaidClient.PlaidApi.ItemPublicTokenExchange(context.Background()).ItemPublicTokenExchangeRequest(
			*exchangePublicTokenReq,
		).Execute()

		if err != nil {
			http.Error(w, "Failed to exchange public token", http.StatusInternalServerError)
			log.Printf("ERROR: Plaid public token exchange failed for user %d: %v", userID, err)
			return
		}

		accessToken := exchangePublicTokenResp.GetAccessToken()
		itemID := exchangePublicTokenResp.GetItemId()

		// Fetch item details to get institution info
		itemReq := plaid.NewItemGetRequest(accessToken)
		itemResp, _, err := plaidClient.PlaidApi.ItemGet(context.Background()).ItemGetRequest(*itemReq).Execute()
		if err != nil {
			log.Printf("ERROR: Failed to fetch item details for user %d: %v", userID, err)
			// Don't fail the flow, institution details are optional
		}

		log.Print(itemResp)
		log.Print(itemResp.GetItem().InstitutionId.IsSet())
		log.Print(itemResp.GetItem().AdditionalProperties)

		institutionID := ""
		if itemResp.GetItem().InstitutionId.IsSet() {
			institutionID = *itemResp.GetItem().InstitutionId.Get()
		}
		institutionName := itemResp.GetItem().AdditionalProperties["institution_name"].(string)

		log.Print(institutionID)
		log.Print(institutionName)

		err = db.SavePlaidItem(r.Context(), pool, userID, itemID, accessToken, institutionID, institutionName)
		if err != nil {
			http.Error(w, "Failed to save plaid item", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to save plaid item for user %d: %v", userID, err)
			return
		}

		log.Printf("INFO: Successfully exchanged public token and saved plaid item for user %d, item %s", userID, itemID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"item_id":      itemID,
			"access_token": accessToken,
		})
	}
}

func GetPlaidAccounts(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
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
