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
	"github.com/plaid/plaid-go/v41/plaid"
)

func CreateLinkToken(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		user := plaid.LinkTokenCreateRequestUser{
			ClientUserId: strconv.FormatInt(userID, 10),
		}

		var days int32 = 370
		transactions := plaid.LinkTokenTransactions{
			DaysRequested: &days,
		}

		request := plaid.NewLinkTokenCreateRequest(
			"Budgee",
			"en",
			[]plaid.CountryCode{plaid.COUNTRYCODE_US},
		)
		request.SetUser(user)
		request.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
		request.SetTransactions(transactions)

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

		institutionID := ""
		if itemResp.GetItem().InstitutionId.IsSet() {
			institutionID = *itemResp.GetItem().InstitutionId.Get()
		}
		institutionName := ""
		if itemResp.GetItem().InstitutionName.IsSet() {
			institutionName = *itemResp.GetItem().InstitutionName.Get()
		}

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

		log.Println("Fetching accounts for user:", userID, "item:", itemID)

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

		err = db.SaveAccounts(r.Context(), pool, userID, itemID, accountsResp.GetAccounts())
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

		hasMore := true
		var allAdded []plaid.Transaction
		var allModified []plaid.Transaction
		var allRemoved []plaid.RemovedTransaction

		for hasMore {
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

			allAdded = append(allAdded, transactionsResp.GetAdded()...)
			allModified = append(allModified, transactionsResp.GetModified()...)
			allRemoved = append(allRemoved, transactionsResp.GetRemoved()...)

			hasMore = transactionsResp.GetHasMore()
			cursor = transactionsResp.GetNextCursor()
		}

		// Save added transactions
		err = db.SaveTransactions(r.Context(), pool, userID, allAdded)
		if err != nil {
			http.Error(w, "Failed to save transactions", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to save transactions for user %d: %v", userID, err)
			return
		}

		// Update modified transactions
		err = db.UpdateTransactions(r.Context(), pool, userID, allModified)
		if err != nil {
			http.Error(w, "Failed to update transactions", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to update transactions for user %d: %v", userID, err)
			return
		}

		// Remove deleted transactions
		err = db.RemoveTransactions(r.Context(), pool, userID, allRemoved)
		if err != nil {
			http.Error(w, "Failed to remove transactions", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to remove transactions for user %d: %v", userID, err)
			return
		}

		err = db.UpdateSyncCursor(r.Context(), pool, dbItemID, cursor)
		if err != nil {
			http.Error(w, "Failed to update sync cursor", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to update sync cursor for item %d: %v", dbItemID, err)
			return
		}

		log.Printf("INFO: Successfully synced transactions for user %d, item %d - Added: %d, Modified: %d, Removed: %d", userID, dbItemID, len(allAdded), len(allModified), len(allRemoved))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"added":    len(allAdded),
			"modified": len(allModified),
			"removed":  len(allRemoved),
		})
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

func DeletePlaidItem(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		itemID := chi.URLParam(r, "item_id")

		// Fetch owner and access token
		query := `SELECT user_id, access_token FROM plaid_items WHERE id = $1`
		var ownerUserID int64
		var accessToken string
		err := pool.QueryRow(r.Context(), query, itemID).Scan(&ownerUserID, &accessToken)
		if err != nil {
			http.Error(w, "Item not found", http.StatusNotFound)
			log.Printf("ERROR: Failed to find plaid item - item_id: %s: %v", itemID, err)
			return
		}

		if userID != ownerUserID {
			log.Printf("ERROR: Unauthorized plaid item deletion attempt - Authenticated user: %d, Item owner: %d, Item: %s", userID, ownerUserID, itemID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Remove item from Plaid
		request := plaid.NewItemRemoveRequest(accessToken)
		_, _, err = plaidClient.PlaidApi.ItemRemove(r.Context()).ItemRemoveRequest(*request).Execute()
		if err != nil {
			http.Error(w, "Failed to remove item from Plaid", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to remove item from Plaid - item_id: %s, user_id: %d: %v", itemID, userID, err)
			return
		}

		// Remove item from DB
		err = db.DeletePlaidItem(r.Context(), pool, itemID)
		if err != nil {
			http.Error(w, "Failed to delete plaid item", http.StatusInternalServerError)
			log.Printf("ERROR: Failed to delete plaid item - item_id: %s, user_id: %d: %v", itemID, userID, err)
			return
		}

		log.Printf("INFO: Plaid item deleted - User: %d, Item: %s", userID, itemID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "item deleted successfully",
		})
	}
}

func UpdateTransaction(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		transactionIDStr := chi.URLParam(r, "transaction_id")
		transactionID, err := strconv.Atoi(transactionIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid transaction_id: %s", transactionIDStr)
			http.Error(w, "invalid transaction id", http.StatusBadRequest)
			return
		}

		var req struct {
			Amount       float64 `json:"amount"`
			Category     string  `json:"category"`
			MerchantName string  `json:"merchant_name"`
			Date         string  `json:"date"` // Expecting YYYY-MM-DD
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode update transaction request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Check ownership using integer keys
		query := `
			SELECT t.id FROM transactions t
			JOIN accounts a ON t.account_id = a.id
			JOIN plaid_items p ON a.item_id = p.id
			WHERE t.id = $1 AND p.user_id = $2
		`
		var id int
		err = pool.QueryRow(r.Context(), query, transactionID, userID).Scan(&id)
		if err != nil {
			log.Printf("ERROR: Transaction not found or forbidden for update - transaction_id: %d, user_id: %d: %v", transactionID, userID, err)
			http.Error(w, "transaction not found or forbidden", http.StatusForbidden)
			return
		}

		updateQuery := `
			UPDATE transactions
			SET amount = $1, category = $2, merchant_name = $3, date = $4, updated_at = NOW()
			WHERE id = $5
		`
		_, err = pool.Exec(r.Context(), updateQuery, req.Amount, req.Category, req.MerchantName, req.Date, transactionID)
		if err != nil {
			log.Printf("ERROR: Failed to update transaction - transaction_id: %d, user_id: %d: %v", transactionID, userID, err)
			http.Error(w, "failed to update transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "transaction updated"})
	}
}

func DeleteTransaction(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		transactionIDStr := chi.URLParam(r, "transaction_id")
		transactionID, err := strconv.Atoi(transactionIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid transaction_id: %s", transactionIDStr)
			http.Error(w, "invalid transaction id", http.StatusBadRequest)
			return
		}

		// Check ownership using integer keys
		query := `
			SELECT t.id FROM transactions t
			JOIN accounts a ON t.account_id = a.id
			JOIN plaid_items p ON a.item_id = p.id
			WHERE t.id = $1 AND p.user_id = $2
		`
		var id int
		err = pool.QueryRow(r.Context(), query, transactionID, userID).Scan(&id)
		if err != nil {
			log.Printf("ERROR: Transaction not found or forbidden for delete - transaction_id: %d, user_id: %d: %v", transactionID, userID, err)
			http.Error(w, "transaction not found or forbidden", http.StatusForbidden)
			return
		}

		_, err = pool.Exec(r.Context(), "DELETE FROM transactions WHERE id = $1", transactionID)
		if err != nil {
			log.Printf("ERROR: Failed to delete transaction - transaction_id: %d, user_id: %d: %v", transactionID, userID, err)
			http.Error(w, "failed to delete transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "transaction deleted"})
	}
}
