package db

import (
	"budgee-server/src/models"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/plaid"
)

func GetPlaidItemsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]models.PlaidItem, error) {
	query := `SELECT id, user_id, access_token, item_id, created_at FROM plaid_items WHERE user_id = $1`

	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.PlaidItem
	for rows.Next() {
		var item models.PlaidItem
		err := rows.Scan(&item.ID, &item.UserID, &item.AccessToken, &item.ItemID, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func GetAccountsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID string) ([]models.Account, error) {
	query := `
		SELECT a.id, a.item_id, a.account_id, a.name, a.official_name, a.mask, a.type, a.subtype, a.current_balance, a.available_balance, a.created_at 
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

	return accounts, rows.Err()
}

func GetTransactionsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64, accountID string) ([]models.Transaction, error) {
	query := `
		SELECT t.id, t.account_id, t.amount, t.name, t.date, t.category, t.created_at
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		JOIN plaid_items p ON a.item_id = p.id
		WHERE p.user_id = $1 AND a.id = $2
	`

	rows, err := pool.Query(ctx, query, userID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		err := rows.Scan(&transaction.ID, &transaction.AccountID, &transaction.Amount, &transaction.Description, &transaction.Date, &transaction.Category, &transaction.CreatedAt)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}

	return transactions, rows.Err()
}

func SaveTransactions(ctx context.Context, pool *pgxpool.Pool, userID int64, transactions []plaid.Transaction) error {
	for _, txn := range transactions {
		query := `
			INSERT INTO transactions (account_id, transaction_id, amount, name, date, category, pending, created_at)
			SELECT $1, $2, $3, $4, $5, $6, $7, NOW()
			FROM accounts a
			JOIN plaid_items p ON a.item_id = p.id
			WHERE p.user_id = $8 AND a.account_id = $9
			ON CONFLICT (transaction_id) DO NOTHING
		`

		_, err := pool.Exec(ctx, query,
			txn.GetAccountId(),
			txn.GetTransactionId(),
			txn.GetAmount(),
			txn.GetName(),
			txn.GetDate(),
			txn.GetPersonalFinanceCategory(),
			txn.GetPending(),
			userID,
			txn.GetAccountId(),
		)
		if err != nil {
			return err
		}
	}

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

func SavePlaidItem(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID string, accessToken string) error {
	query := `
		INSERT INTO plaid_items (user_id, item_id, access_token, institution_id, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (item_id) DO NOTHING
	`

	_, err := pool.Exec(ctx, query, userID, itemID, accessToken, "", "active")
	return err
}

func SaveAccounts(ctx context.Context, pool *pgxpool.Pool, userID int64, accounts []plaid.AccountBase) error {
	for _, acc := range accounts {
		query := `
			INSERT INTO accounts (item_id, account_id, name, official_name, mask, type, subtype, current_balance, available_balance)
			SELECT p.id, $1, $2, $3, $4, $5, $6, $7, $8
			FROM plaid_items p
			WHERE p.user_id = $9
			ON CONFLICT (account_id) DO UPDATE SET 
				name = $2, 
				official_name = $3,
				current_balance = $7,
				available_balance = $8,
				updated_at = NOW()
		`

		_, err := pool.Exec(ctx, query,
			acc.GetAccountId(),
			acc.GetName(),
			acc.GetOfficialName(),
			acc.GetMask(),
			acc.GetSubtype(),
			acc.GetType(),
			acc.GetBalances().Current,
			acc.GetBalances().Available,
			userID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
