package handlers

import (
	db "budgee-server/src/db/sql"
	"budgee-server/src/models"
	"budgee-server/src/util"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
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

		webhookURL := os.Getenv("PLAID_WEBHOOK_URL")
		if webhookURL != "" {
			request.SetWebhook(webhookURL)
		}

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

func SyncTransactionsForItem(ctx context.Context, pool *pgxpool.Pool, plaidClient *plaid.APIClient, item *models.PlaidItem) error {
	itemIDInt, err := strconv.ParseInt(item.ID, 10, 64)
	if err != nil {
		return err
	}

	cursor, err := db.GetSyncCursor(ctx, pool, itemIDInt)
	if err != nil {
		return err
	}

	hasMore := true
	options := plaid.NewTransactionsSyncRequestOptions()
	options.SetIncludePersonalFinanceCategory(true)
	options.SetPersonalFinanceCategoryVersion(plaid.PERSONALFINANCECATEGORYVERSION_V2)

	var allAdded []plaid.Transaction
	var allModified []plaid.Transaction
	var allRemoved []plaid.RemovedTransaction

	for hasMore {
		request := plaid.NewTransactionsSyncRequest(item.AccessToken)
		request.SetOptions(*options)
		if cursor != "" {
			request.SetCursor(cursor)
		}
		transactionsResp, _, err := plaidClient.PlaidApi.TransactionsSync(ctx).TransactionsSyncRequest(*request).Execute()
		if err != nil {
			return err
		}
		allAdded = append(allAdded, transactionsResp.GetAdded()...)
		allModified = append(allModified, transactionsResp.GetModified()...)
		allRemoved = append(allRemoved, transactionsResp.GetRemoved()...)
		hasMore = transactionsResp.GetHasMore()
		cursor = transactionsResp.GetNextCursor()
	}

	// Save added transactions
	if err := db.SaveTransactions(ctx, pool, item.UserID, allAdded); err != nil {
		return err
	}
	// Update modified transactions
	if err := db.UpdateTransactions(ctx, pool, item.UserID, allModified); err != nil {
		return err
	}
	// Remove deleted transactions
	if err := db.RemoveTransactions(ctx, pool, item.UserID, allRemoved); err != nil {
		return err
	}
	// Update sync cursor
	if err := db.UpdateSyncCursor(ctx, pool, itemIDInt, cursor); err != nil {
		return err
	}

	log.Printf("INFO: Successfully synced transactions for user %d, item %d - Added: %d, Modified: %d, Removed: %d",
		item.UserID, itemIDInt, len(allAdded), len(allModified), len(allRemoved))

	// Apply transaction rules after syncing
	err = db.ApplyTransactionRulesToUser(ctx, pool, item.UserID)
	if err != nil {
		return err
	}

	return nil
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

		accounts, err := db.GetAccountsForUserAndItemSQL(r.Context(), pool, userID, itemID)
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
			Amount                         float64 `json:"amount"`
			PrimaryCategory                string  `json:"primary_category"`
			DetailedCategory               string  `json:"detailed_category"`
			MerchantName                   string  `json:"merchant_name"`
			Date                           string  `json:"date"` // Expecting YYYY-MM-DD
			PaymentChannel                 string  `json:"payment_channel"`
			PersonalFinanceCategoryIconURL string  `json:"personal_finance_category_icon_url"`
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
			SET amount = $1, primary_category = $2, detailed_category = $3, merchant_name = $4, date = $5, payment_channel = $6, personal_finance_category_icon_url = $7, updated_at = NOW()
			WHERE id = $8
		`
		_, err = pool.Exec(r.Context(), updateQuery, req.Amount, req.PrimaryCategory, req.DetailedCategory, req.MerchantName, req.Date, req.PaymentChannel, req.PersonalFinanceCategoryIconURL, transactionID)
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

func PlaidWebhook(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WebhookCode string `json:"webhook_code"`
			ItemID      string `json:"item_id"`
		}

		// Read the raw body for verification
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("ERROR: Failed to read webhook request body: %v", err)
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}
		// Restore the body for json decoding
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode webhook request body: %v", err)
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}

		// Convert headers to map[string]string
		headers := make(map[string]string)
		for k, v := range r.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		valid, err := util.VerifyWebhook(r.Context(), plaidClient, bodyBytes, headers)
		if err != nil || !valid {
			log.Printf("ERROR: Failed to verify webhook: %v", err)
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}

		switch req.WebhookCode {
		case "SYNC_UPDATES_AVAILABLE":
			log.Printf("INFO: Received Plaid webhook to sync transactions - Item: %s, Webhook Code: %s", req.ItemID, req.WebhookCode)

			// Trigger async in goroutine to ensure quick 200 response to Plaid
			go TriggerTransactionSyncFromWebhook(plaidClient, pool, req.ItemID)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "received"})
			return
		default:
			log.Printf("INFO: Received unhandled Plaid webhook - Item: %s, Webhook Code: %s", req.ItemID, req.WebhookCode)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "received"})
		}
	}
}

func TriggerTransactionSyncFromWebhook(plaidClient *plaid.APIClient, pool *pgxpool.Pool, itemID string) {
	// Get item from db using itemID lookup
	item, err := db.GetPlaidItemByItemID(context.Background(), pool, itemID)
	if err != nil {
		log.Printf("ERROR: From Webhook: Failed to fetch item for item_id %s: %v", itemID, err)
		return
	}
	err = SyncTransactionsForItem(context.Background(), pool, plaidClient, item)
	if err != nil {
		log.Printf("ERROR: From Webhook: Failed to sync transactions for item %s: %v", itemID, err)
	} else {
		log.Printf("INFO: From Webhook: Successfully synced transactions for item %s", itemID)
	}

	err = db.RecategorizeTransactions(context.Background(), pool)
	if err != nil {
		log.Printf("ERROR: From Webhook: Failed to recategorize transactions after sync for item %s: %v", itemID, err)
	} else {
		log.Printf("INFO: From Webhook: Successfully recategorized transactions after sync for item %s", itemID)
	}
	// After transaction sync and recategorization, update account balances
	err = UpdateAccountBalances(context.Background(), plaidClient, pool, itemID)
	if err != nil {
		log.Printf("ERROR: From Webhook: Failed to update account balances for item %s: %v", itemID, err)
	} else {
		log.Printf("INFO: From Webhook: Successfully updated account balances for item %s", itemID)
	}
}

// UpdateAccountBalances syncs account balances from Plaid to DB for the given itemID
func UpdateAccountBalances(ctx context.Context, plaidClient *plaid.APIClient, pool *pgxpool.Pool, itemID string) error {
	// 1. Get all accounts for the itemID from the DB
	dbAccounts, err := db.GetAccountsForItemSQL(ctx, pool, itemID)
	if err != nil {
		return err
	}

	// 2. Get all accounts for the itemID from Plaid
	var accessToken string
	err = pool.QueryRow(ctx, "SELECT access_token FROM plaid_items WHERE item_id = $1", itemID).Scan(&accessToken)
	if err != nil {
		return err
	}
	request := plaid.NewAccountsGetRequest(accessToken)
	accountsResp, _, err := plaidClient.PlaidApi.AccountsGet(ctx).AccountsGetRequest(*request).Execute()
	if err != nil {
		return err
	}
	plaidAccounts := accountsResp.GetAccounts()

	// 3. Compare balances and update DB if different
	// Build a map of DB accounts by account_id
	dbMap := make(map[string]*models.Account)
	for i := range dbAccounts {
		dbMap[dbAccounts[i].AccountID] = &dbAccounts[i]
	}
	for _, plaidAcc := range plaidAccounts {
		accID := plaidAcc.GetAccountId()
		dbAcc, found := dbMap[accID]
		if !found {
			continue // ignore accounts not in DB
		}
		// Convert Plaid balances to string for comparison
		var plaidCurrentStr, plaidAvailableStr string
		if plaidAcc.Balances.Current.IsSet() {
			plaidCurrentStr = strconv.FormatFloat(plaidAcc.Balances.GetCurrent(), 'f', 2, 64)
		}
		if plaidAcc.Balances.Available.IsSet() {
			plaidAvailableStr = strconv.FormatFloat(plaidAcc.Balances.GetAvailable(), 'f', 2, 64)
		}
		needsUpdate := false
		if dbAcc.CurrentBalance != plaidCurrentStr {
			needsUpdate = true
		}
		if dbAcc.AvailableBalance != plaidAvailableStr {
			needsUpdate = true
		}
		if needsUpdate {
			_, err := pool.Exec(ctx, "UPDATE accounts SET current_balance = $1, available_balance = $2 WHERE account_id = $3", plaidCurrentStr, plaidAvailableStr, accID)
			if err != nil {
				log.Printf("ERROR: Failed to update account balance for account_id %s: %v", accID, err)
			}
		}
	}
	return nil
}

func FireSandboxWebhook(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ItemID      string `json:"item_id"`
			WebhookCode string `json:"webhook_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode fire_webhook request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Look up the access token for the given item_id
		var accessToken string
		err := pool.QueryRow(r.Context(), "SELECT access_token FROM plaid_items WHERE item_id = $1", req.ItemID).Scan(&accessToken)
		if err != nil {
			log.Printf("ERROR: Failed to find access token for item_id %s: %v", req.ItemID, err)
			http.Error(w, "item not found", http.StatusNotFound)
			return
		}

		fireReq := plaid.NewSandboxItemFireWebhookRequest(accessToken, req.WebhookCode)
		_, _, err = plaidClient.PlaidApi.SandboxItemFireWebhook(r.Context()).SandboxItemFireWebhookRequest(*fireReq).Execute()
		if err != nil {
			log.Printf("ERROR: Failed to fire sandbox webhook for item_id %s: %v", req.ItemID, err)
			http.Error(w, "failed to fire webhook", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: Fired sandbox webhook for item_id %s, webhook_code %s", req.ItemID, req.WebhookCode)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "webhook fired"})
	}
}

func RecategorizeTransactions(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := db.RecategorizeTransactions(r.Context(), pool)
		if err != nil {
			log.Printf("ERROR: Failed to recategorize transactions: %v", err)
			http.Error(w, "failed to recategorize transactions", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "transactions recategorized"})
	}
}

func UpdateItemWebhook(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ItemID     string `json:"item_id"`
			WebhookURL string `json:"webhook_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode update webhook request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if req.WebhookURL == "" {
			http.Error(w, "webhook_url is required", http.StatusBadRequest)
			return
		}

		// Look up the access token for the given item_id
		var accessToken string
		err := pool.QueryRow(r.Context(), "SELECT access_token FROM plaid_items WHERE item_id = $1", req.ItemID).Scan(&accessToken)
		if err != nil {
			log.Printf("ERROR: Failed to find access token for item_id %s: %v", req.ItemID, err)
			http.Error(w, "item not found", http.StatusNotFound)
			return
		}

		request := plaid.NewItemWebhookUpdateRequest(accessToken)
		request.SetWebhook(req.WebhookURL)
		_, _, err = plaidClient.PlaidApi.ItemWebhookUpdate(r.Context()).ItemWebhookUpdateRequest(*request).Execute()
		if err != nil {
			log.Printf("ERROR: Failed to update webhook for item_id %s: %v", req.ItemID, err)
			http.Error(w, "failed to update webhook", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: Updated webhook for item_id %s to %s", req.ItemID, req.WebhookURL)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "webhook updated"})
	}
}

func CreateTransaction(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		var req struct {
			AccountID        string  `json:"account_id"`
			Amount           float64 `json:"amount"`
			Date             string  `json:"date"`
			Name             string  `json:"name"`
			MerchantName     string  `json:"merchant_name"`
			PrimaryCategory  string  `json:"primary_category"`
			DetailedCategory string  `json:"detailed_category"`
			PaymentChannel   string  `json:"payment_channel"`
			Expense          bool    `json:"expense"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode create transaction request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Check that the account belongs to the user
		query := `
			SELECT a.id, a.type FROM accounts a
			JOIN plaid_items p ON a.item_id = p.id
			WHERE a.id = $1 AND p.user_id = $2
		`
		var accountID int64
		var accountType string
		err := pool.QueryRow(r.Context(), query, req.AccountID, userID).Scan(&accountID, &accountType)
		if err != nil {
			log.Printf("ERROR: Account not found or forbidden for create transaction - account_id: %s, user_id: %d: %v", req.AccountID, userID, err)
			http.Error(w, "account not found or forbidden", http.StatusForbidden)
			return
		}

		income := util.IsIncome(accountType, req.Amount, req.PrimaryCategory)

		txn, err := db.InsertTransaction(
			r.Context(),
			pool,
			accountID,
			req.Amount,
			req.Date,
			req.Name,
			req.MerchantName,
			req.PrimaryCategory,
			req.DetailedCategory,
			req.PaymentChannel,
			req.Expense,
		)
		if err != nil {
			log.Printf("ERROR: Failed to insert transaction: %v", err)
			http.Error(w, "failed to create transaction", http.StatusInternalServerError)
			return
		}

		// Set the income field in the returned transaction
		txn.Income = income

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(txn)
	}
}
