package db

import (
	"budgee-server/src/db"
	"budgee-server/src/models"
	"budgee-server/src/util"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/v41/plaid"
)

func GetPlaidItemsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]models.PlaidItem, error) {
	cacheKey := "items_user_" + fmt.Sprint(userID)
	if val, found := db.Cache.Get(cacheKey); found {
		return val.([]models.PlaidItem), nil
	}

	query := `SELECT id, user_id, access_token, item_id, institution_id, institution_name, created_at FROM plaid_items WHERE user_id = $1`

	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.PlaidItem
	for rows.Next() {
		var item models.PlaidItem
		err := rows.Scan(&item.ID, &item.UserID, &item.AccessToken, &item.ItemID, &item.InstitutionID, &item.InstitutionName, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	db.SetItemCache(cacheKey, items)
	return items, rows.Err()
}

func GetAccountsForUserAndItemSQL(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID string) ([]models.Account, error) {
	cacheKey := "accounts_item_user_" + fmt.Sprint(userID) + fmt.Sprint("_") + fmt.Sprint(itemID)
	if val, found := db.Cache.Get(cacheKey); found {
		return val.([]models.Account), nil
	}

	query := `
		SELECT a.id, a.item_id, a.account_id, a.name, a.official_name, a.mask, a.type, a.subtype, 
		       COALESCE(a.current_balance, 0), COALESCE(a.available_balance, 0), a.created_at 
		FROM accounts a
		JOIN plaid_items p ON a.item_id = p.id
		WHERE p.user_id = $1 AND p.id = $2
	`

	rows, err := pool.Query(ctx, query, userID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var account models.Account
		err := rows.Scan(&account.ID, &account.ItemID, &account.AccountID, &account.Name, &account.OfficialName, &account.Mask, &account.Type, &account.Subtype, &account.CurrentBalance, &account.AvailableBalance, &account.CreatedAt)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	db.SetAccountCache(cacheKey, accounts)
	return accounts, rows.Err()
}

func GetAccountsForItemSQL(ctx context.Context, pool *pgxpool.Pool, itemID string) ([]models.Account, error) {
	cacheKey := "accounts_item_" + fmt.Sprint(itemID)
	if val, found := db.Cache.Get(cacheKey); found {
		return val.([]models.Account), nil
	}

	query := `
		SELECT a.id, a.item_id, a.account_id, a.name, a.official_name, a.mask, a.type, a.subtype, 
		       COALESCE(a.current_balance, 0), COALESCE(a.available_balance, 0), a.created_at 
		FROM accounts a
		JOIN plaid_items p ON a.item_id = p.id
		WHERE p.item_id = $1
	`

	rows, err := pool.Query(ctx, query, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var account models.Account
		err := rows.Scan(&account.ID, &account.ItemID, &account.AccountID, &account.Name, &account.OfficialName, &account.Mask, &account.Type, &account.Subtype, &account.CurrentBalance, &account.AvailableBalance, &account.CreatedAt)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	db.SetAccountCache(cacheKey, accounts)
	return accounts, rows.Err()
}

func GetTransactionsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64, accountID string) ([]models.Transaction, error) {
	cacheKey := "transactions_account_user_" + fmt.Sprint(userID) + fmt.Sprint("_") + fmt.Sprint(accountID)
	if val, found := db.Cache.Get(cacheKey); found {
		return val.([]models.Transaction), nil
	}

	query := `
		   SELECT 
			   t.id, t.account_id, t.transaction_id, t.primary_category, t.detailed_category, t.payment_channel, t.type, t.name, t.merchant_name,
			   t.amount, t.currency, t.date, t.pending, t.expense, t.income, t.account_owner, t.personal_finance_category_icon_url, t.created_at, t.updated_at
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		JOIN plaid_items p ON a.item_id = p.id
		WHERE p.user_id = $1 AND a.id = $2
		ORDER BY t.date DESC
	`

	rows, err := pool.Query(ctx, query, userID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		err := rows.Scan(
			&transaction.ID,
			&transaction.AccountID,
			&transaction.TransactionID,
			&transaction.PrimaryCategory,
			&transaction.DetailedCategory,
			&transaction.PaymentChannel,
			&transaction.Type,
			&transaction.Name,
			&transaction.MerchantName,
			&transaction.Amount,
			&transaction.Currency,
			&transaction.Date,
			&transaction.Pending,
			&transaction.Expense,
			&transaction.Income,
			&transaction.AccountOwner,
			&transaction.PersonalFinanceCategoryIconURL,
			&transaction.CreatedAt,
			&transaction.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}

	db.SetTransactionCache(cacheKey, transactions)
	return transactions, rows.Err()
}

func SaveTransactions(ctx context.Context, pool *pgxpool.Pool, userID int64, transactions []plaid.Transaction) error {
	for _, txn := range transactions {
		query := `
				INSERT INTO transactions (account_id, transaction_id, amount, name, date, primary_category, detailed_category, payment_channel, pending, expense, income, type, merchant_name, currency, account_owner, personal_finance_category_icon_url, created_at)
				SELECT a.id, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW()
				FROM accounts a
				JOIN plaid_items p ON a.item_id = p.id
				WHERE p.user_id = $17 AND a.account_id = $1
				ON CONFLICT (transaction_id) DO NOTHING
			`

		primaryCategory := ""
		detailedCategory := ""
		iconURL := ""
		if txn.PersonalFinanceCategory.IsSet() {
			primaryCategory = txn.GetPersonalFinanceCategory().Primary
			detailedCategory = txn.GetPersonalFinanceCategory().Detailed
			iconURL = txn.GetPersonalFinanceCategoryIconUrl()
		}

		accountType, err := GetAccountTypeByAccountID(ctx, pool, txn.GetAccountId())
		if err != nil {
			return err
		}
		expense := util.IsExpense(accountType, txn.GetAmount(), primaryCategory)
		income := util.IsIncome(accountType, txn.GetAmount(), primaryCategory)

		_, err = pool.Exec(ctx, query,
			txn.GetAccountId(),       // $1
			txn.GetTransactionId(),   // $2
			txn.GetAmount(),          // $3
			txn.GetName(),            // $4
			txn.GetDate(),            // $5
			primaryCategory,          // $6
			detailedCategory,         // $7
			txn.GetPaymentChannel(),  // $8
			txn.GetPending(),         // $9
			expense,                  // $10
			income,                   // $11
			txn.GetTransactionType(), // $12
			txn.GetMerchantName(),    // $13
			txn.GetIsoCurrencyCode(), // $14
			txn.GetAccountOwner(),    // $15
			iconURL,                  // $16
			userID,                   // $17
		)
		if err != nil {
			return err
		}
	}

	db.ClearAllTransactionCaches()
	return nil
}

func UpdateTransactions(ctx context.Context, pool *pgxpool.Pool, userID int64, transactions []plaid.Transaction) error {
	for _, txn := range transactions {
		query := `
			UPDATE transactions
			SET amount = $1, name = $2, date = $3, primary_category = $4, detailed_category = $5, payment_channel = $6, pending = $7, merchant_name = $8, currency = $9, account_owner = $10, personal_finance_category_icon_url = $11, expense = $12, income = $13, updated_at = NOW()
			WHERE transaction_id = $14 AND account_id IN (
				SELECT a.id FROM accounts a
				JOIN plaid_items p ON a.item_id = p.id
				WHERE p.user_id = $15
			)
		`

		primaryCategory := ""
		detailedCategory := ""
		iconURL := ""
		if txn.PersonalFinanceCategory.IsSet() {
			primaryCategory = txn.GetPersonalFinanceCategory().Primary
			detailedCategory = txn.GetPersonalFinanceCategory().Detailed
			iconURL = txn.GetPersonalFinanceCategoryIconUrl()
		}

		accountType, err := GetAccountTypeByAccountID(ctx, pool, txn.GetAccountId())
		if err != nil {
			return err
		}
		expense := util.IsExpense(accountType, txn.GetAmount(), primaryCategory)
		income := util.IsIncome(accountType, txn.GetAmount(), primaryCategory)

		_, err = pool.Exec(ctx, query,
			txn.GetAmount(),          // $1
			txn.GetName(),            // $2
			txn.GetDate(),            // $3
			primaryCategory,          // $4
			detailedCategory,         // $5
			txn.GetPaymentChannel(),  // $6
			txn.GetPending(),         // $7
			txn.GetMerchantName(),    // $8
			txn.GetIsoCurrencyCode(), // $9
			txn.GetAccountOwner(),    // $10
			iconURL,                  // $11
			expense,                  // $12
			income,                   // $13
			txn.GetTransactionId(),   // $14
			userID,                   // $15
		)
		if err != nil {
			return err
		}
	}

	db.ClearAllTransactionCaches()
	return nil
}

func RemoveTransactions(ctx context.Context, pool *pgxpool.Pool, userID int64, removedTransactions []plaid.RemovedTransaction) error {
	for _, txn := range removedTransactions {
		query := `
			DELETE FROM transactions
			WHERE transaction_id = $1 AND account_id IN (
				SELECT a.id FROM accounts a
				JOIN plaid_items p ON a.item_id = p.id
				WHERE p.user_id = $2
			)
		`

		_, err := pool.Exec(ctx, query,
			txn.GetTransactionId(),
			userID,
		)
		if err != nil {
			return err
		}
	}

	db.ClearAllTransactionCaches()
	return nil
}

func GetSyncCursor(ctx context.Context, pool *pgxpool.Pool, itemID int64) (string, error) {
	query := `SELECT COALESCE(sync_cursor, '') FROM plaid_items WHERE id = $1`
	var cursor string
	err := pool.QueryRow(ctx, query, itemID).Scan(&cursor)
	if err != nil {
		return "", err
	}
	return cursor, nil
}

func UpdateSyncCursor(ctx context.Context, pool *pgxpool.Pool, itemID int64, cursor string) error {
	query := `UPDATE plaid_items SET sync_cursor = $1 WHERE id = $2`
	_, err := pool.Exec(ctx, query, cursor, itemID)
	return err
}

func SavePlaidItem(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID, accessToken, institutionID, institutionName string) error {
	query := `
		INSERT INTO plaid_items (user_id, item_id, access_token, institution_id, institution_name, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (item_id) DO NOTHING
	`

	_, err := pool.Exec(ctx, query, userID, itemID, accessToken, institutionID, institutionName, "active")
	db.DelItemCache("items_user_" + fmt.Sprint(userID))
	db.DelItemCache("items_all")
	return err
}

func SaveAccounts(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID string, accounts []plaid.AccountBase) error {
	for _, acc := range accounts {
		query := `
			INSERT INTO accounts (item_id, account_id, name, official_name, mask, type, subtype, current_balance, available_balance)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT DO NOTHING
		`

		var currentBalance, availableBalance float64
		if acc.GetBalances().Current.IsSet() {
			currentBalance = acc.Balances.GetCurrent() //acc.GetBalances().Current.Get()
		} else {
			currentBalance = 0
		}
		if acc.GetBalances().Available.IsSet() {
			availableBalance = acc.Balances.GetAvailable()
		} else {
			availableBalance = 0
		}

		_, err := pool.Exec(ctx, query,
			itemID,
			acc.GetAccountId(),
			acc.GetName(),
			acc.GetOfficialName(),
			acc.GetMask(),
			acc.GetType(),
			acc.GetSubtype(),
			currentBalance,
			availableBalance,
		)
		if err != nil {
			return err
		}
	}

	db.DelAccountCache("accounts_item_user_" + fmt.Sprint(userID) + fmt.Sprint("_") + fmt.Sprint(itemID))
	return nil
}

func UpdatePlaidItemInstitution(ctx context.Context, pool *pgxpool.Pool, userID int64, institutionID string, institutionName string) error {
	query := `UPDATE plaid_items SET institution_id = $1, institution_name = $2, updated_at = NOW() WHERE user_id = $3 AND item_id IN (SELECT item_id FROM plaid_items WHERE user_id = $3 ORDER BY created_at DESC LIMIT 1)`
	_, err := pool.Exec(ctx, query, institutionID, institutionName, userID)
	db.DelItemCache("items_user_" + fmt.Sprint(userID))
	db.DelItemCache("items_all")
	return err
}

func DeletePlaidItem(ctx context.Context, pool *pgxpool.Pool, itemID string, userID int64) error {
	query := `DELETE FROM plaid_items WHERE id = $1`
	_, err := pool.Exec(ctx, query, itemID)
	if err != nil {
		return fmt.Errorf("failed to delete plaid item: %w", err)
	}
	db.DelItemCache("items_user_" + fmt.Sprint(userID))
	db.DelItemCache("items_all")
	return nil
}

func GetAllPlaidItems(ctx context.Context, pool *pgxpool.Pool) ([]models.PlaidItem, error) {
	cacheKey := "items_all"
	if val, found := db.Cache.Get(cacheKey); found {
		return val.([]models.PlaidItem), nil
	}

	query := `SELECT id, user_id, access_token, item_id, institution_id, institution_name, created_at FROM plaid_items`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.PlaidItem
	for rows.Next() {
		var item models.PlaidItem
		err := rows.Scan(&item.ID, &item.UserID, &item.AccessToken, &item.ItemID, &item.InstitutionID, &item.InstitutionName, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	db.SetItemCache(cacheKey, items)
	return items, rows.Err()
}

func GetPlaidItemByItemID(ctx context.Context, pool *pgxpool.Pool, itemID string) (*models.PlaidItem, error) {
	query := `SELECT id, user_id, access_token, item_id, institution_id, institution_name, created_at FROM plaid_items WHERE item_id = $1`

	var item models.PlaidItem
	err := pool.QueryRow(ctx, query, itemID).Scan(&item.ID, &item.UserID, &item.AccessToken, &item.ItemID, &item.InstitutionID, &item.InstitutionName, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func GetAccountTypeByAccountID(ctx context.Context, pool *pgxpool.Pool, accountID string) (string, error) {
	var accountType string
	err := pool.QueryRow(ctx, "SELECT type FROM accounts WHERE account_id = $1", accountID).Scan(&accountType)
	return accountType, err
}

// RecategorizeTransactions fetches all transactions, recalculates isExpense, and updates if needed.
func RecategorizeTransactions(ctx context.Context, pool *pgxpool.Pool) error {
	query := `
    	SELECT t.id, t.amount, t.primary_category, t.expense, t.income, a.type
    	FROM transactions t
    	JOIN accounts a ON t.account_id = a.id
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	type txnRow struct {
		ID              int
		Amount          float64
		PrimaryCategory *string
		Expense         bool
		Income          bool
		AccountType     string
	}

	var toUpdate []struct {
		ID      int
		Expense bool
		Income  bool
	}
	for rows.Next() {
		var row txnRow
		err := rows.Scan(&row.ID, &row.Amount, &row.PrimaryCategory, &row.Expense, &row.Income, &row.AccountType)
		if err != nil {
			return err
		}
		category := ""
		if row.PrimaryCategory != nil {
			category = *row.PrimaryCategory
		}
		isExpense := util.IsExpense(row.AccountType, row.Amount, category)
		isIncome := util.IsIncome(row.AccountType, row.Amount, category)
		if isExpense != row.Expense || isIncome != row.Income {
			toUpdate = append(toUpdate, struct {
				ID      int
				Expense bool
				Income  bool
			}{row.ID, isExpense, isIncome})
		}
	}
	// Update only those that changed
	for _, upd := range toUpdate {
		_, err := pool.Exec(ctx, "UPDATE transactions SET expense = $1, income = $2 WHERE id = $3", upd.Expense, upd.Income, upd.ID)
		if err != nil {
			return err
		}
	}
	db.ClearAllTransactionCaches()
	return nil
}

// RecategorizeTransaction recalculates isExpense and isIncome for a single transaction and updates if needed.
func RecategorizeTransaction(ctx context.Context, pool *pgxpool.Pool, transactionID int, userID int, accountID int) error {
	query := `
    	SELECT t.id, t.amount, t.primary_category, t.expense, t.income, a.type
    	FROM transactions t
    	JOIN accounts a ON t.account_id = a.id
    	WHERE t.id = $1
	`
	var (
		id              int
		amount          float64
		primaryCategory *string
		expense         bool
		income          bool
		accountType     string
	)
	err := pool.QueryRow(ctx, query, transactionID).Scan(&id, &amount, &primaryCategory, &expense, &income, &accountType)
	if err != nil {
		return err
	}
	category := ""
	if primaryCategory != nil {
		category = *primaryCategory
	}
	isExpense := util.IsExpense(accountType, amount, category)
	isIncome := util.IsIncome(accountType, amount, category)
	if isExpense != expense || isIncome != income {
		_, err := pool.Exec(ctx, "UPDATE transactions SET expense = $1, income = $2 WHERE id = $3", isExpense, isIncome, id)
		if err != nil {
			return err
		}
		db.DelTransactionCache("transactions_account_user_" + fmt.Sprint(userID) + fmt.Sprint("_") + fmt.Sprint(accountID))
	}
	return nil
}

// Unused currently - add cache invalidation if ever used
func InsertTransaction(ctx context.Context, pool *pgxpool.Pool, accountID int64, amount float64, date, name, merchantName, primaryCategory, detailedCategory, paymentChannel string, expense bool) (models.Transaction, error) {
	insertQuery := `
		INSERT INTO transactions
			(account_id, amount, date, name, merchant_name, primary_category, detailed_category, payment_channel, expense, income, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, account_id, transaction_id, primary_category, detailed_category, payment_channel, type, name, merchant_name, amount, currency, date, pending, expense, income, account_owner, personal_finance_category_icon_url, created_at, updated_at
	`
	var txn models.Transaction

	// Determine income using util.IsIncome
	accountType, err := GetAccountTypeByAccountID(ctx, pool, "")
	if err != nil {
		accountType = ""
	}
	income := util.IsIncome(accountType, amount, primaryCategory)

	err = pool.QueryRow(
		ctx,
		insertQuery,
		accountID,
		amount,
		date,
		name,
		merchantName,
		primaryCategory,
		detailedCategory,
		paymentChannel,
		expense,
		income,
	).Scan(
		&txn.ID,
		&txn.AccountID,
		&txn.TransactionID,
		&txn.PrimaryCategory,
		&txn.DetailedCategory,
		&txn.PaymentChannel,
		&txn.Type,
		&txn.Name,
		&txn.MerchantName,
		&txn.Amount,
		&txn.Currency,
		&txn.Date,
		&txn.Pending,
		&txn.Expense,
		&txn.Income,
		&txn.AccountOwner,
		&txn.PersonalFinanceCategoryIconURL,
		&txn.CreatedAt,
		&txn.UpdatedAt,
	)
	return txn, err
}

func UpdateTransaction(ctx context.Context, pool *pgxpool.Pool, transactionID int, req models.UpdateTransactionRequest, userID int64, accountID int64) error {
	updateQuery := `
		UPDATE transactions
		SET amount = $1, primary_category = $2, detailed_category = $3, merchant_name = $4, date = $5, payment_channel = $6, personal_finance_category_icon_url = $7, updated_at = NOW()
		WHERE id = $8
	`
	_, err := pool.Exec(ctx, updateQuery, req.Amount, req.PrimaryCategory, req.DetailedCategory, req.MerchantName, req.Date, req.PaymentChannel, req.PersonalFinanceCategoryIconURL, transactionID)
	db.DelTransactionCache("transactions_account_user_" + fmt.Sprint(userID) + fmt.Sprint("_") + fmt.Sprint(accountID))
	return err
}

func DeleteTransaction(ctx context.Context, pool *pgxpool.Pool, transactionID int, userID int64, accountID int64) error {
	_, err := pool.Exec(ctx, "DELETE FROM transactions WHERE id = $1", transactionID)
	db.DelTransactionCache("transactions_account_user_" + fmt.Sprint(userID) + fmt.Sprint("_") + fmt.Sprint(accountID))
	return err
}

func UpdateAccountBalance(ctx context.Context, pool *pgxpool.Pool, accountID, currentBalance, availableBalance, itemID string) error {
	_, err := pool.Exec(ctx, "UPDATE accounts SET current_balance = $1, available_balance = $2 WHERE account_id = $3", currentBalance, availableBalance, accountID)
	db.DelAccountCache("accounts_item_" + fmt.Sprint(itemID))
	return err
}

func ClearCache(ctx context.Context, pool *pgxpool.Pool, cacheName string) error {
	switch cacheName {
	case "items":
		db.ClearAllItemCaches()
	case "accounts":
		db.ClearAllAccountCaches()
	case "transactions":
		db.ClearAllTransactionCaches()
	default:
		return fmt.Errorf("invalid cache name")
	}

	return nil
}
