package db

import (
	"budgee-server/src/models"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/v41/plaid"
)

func GetPlaidItemsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]models.PlaidItem, error) {
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

	return items, rows.Err()
}

func GetAccountsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID string) ([]models.Account, error) {
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

	return accounts, rows.Err()
}

func GetTransactionsSQL(ctx context.Context, pool *pgxpool.Pool, userID int64, accountID string) ([]models.Transaction, error) {
	query := `
		SELECT 
			t.id, t.account_id, t.transaction_id, t.plaid_category_id, t.category, t.type, t.name, t.merchant_name,
			t.amount, t.currency, t.date, t.pending, t.account_owner, t.created_at, t.updated_at
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
		err := rows.Scan(
			&transaction.ID,
			&transaction.AccountID,
			&transaction.TransactionID,
			&transaction.PlaidCategoryID,
			&transaction.Category,
			&transaction.Type,
			&transaction.Name,
			&transaction.MerchantName,
			&transaction.Amount,
			&transaction.Currency,
			&transaction.Date,
			&transaction.Pending,
			&transaction.AccountOwner,
			&transaction.CreatedAt,
			&transaction.UpdatedAt,
		)
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
			INSERT INTO transactions (account_id, transaction_id, amount, name, date, category, pending, type, merchant_name, currency, account_owner, created_at)
			SELECT a.id, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()
			FROM accounts a
			JOIN plaid_items p ON a.item_id = p.id
			WHERE p.user_id = $12 AND a.account_id = $1
			ON CONFLICT (transaction_id) DO NOTHING
		`

		category := ""
		if txn.PersonalFinanceCategory.IsSet() {
			category = txn.GetPersonalFinanceCategory().Primary
		}

		_, err := pool.Exec(ctx, query,
			txn.GetAccountId(),       // $1
			txn.GetTransactionId(),   // $2
			txn.GetAmount(),          // $3
			txn.GetName(),            // $4
			txn.GetDate(),            // $5
			category,                 // $6
			txn.GetPending(),         // $7
			txn.GetTransactionType(), // $8
			txn.GetMerchantName(),    // $9
			txn.GetIsoCurrencyCode(), // $10
			txn.GetAccountOwner(),    // $11
			userID,                   // $12
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func UpdateTransactions(ctx context.Context, pool *pgxpool.Pool, userID int64, transactions []plaid.Transaction) error {
	for _, txn := range transactions {
		query := `
			UPDATE transactions
			SET amount = $1, name = $2, date = $3, category = $4, pending = $5, merchant_name = $6, currency = $7, account_owner = $8, updated_at = NOW()
			WHERE transaction_id = $9 AND account_id IN (
				SELECT a.id FROM accounts a
				JOIN plaid_items p ON a.item_id = p.id
				WHERE p.user_id = $10
			)
		`

		category := ""
		if txn.PersonalFinanceCategory.IsSet() {
			category = txn.GetPersonalFinanceCategory().Primary
		}

		_, err := pool.Exec(ctx, query,
			txn.GetAmount(),          // $1
			txn.GetName(),            // $2
			txn.GetDate(),            // $3
			category,                 // $4
			txn.GetPending(),         // $5
			txn.GetMerchantName(),    // $6
			txn.GetIsoCurrencyCode(), // $7
			txn.GetAccountOwner(),    // $8
			txn.GetTransactionId(),   // $9
			userID,                   // $10
		)
		if err != nil {
			return err
		}
	}

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
	return err
}

func SaveAccounts(ctx context.Context, pool *pgxpool.Pool, userID int64, itemID string, accounts []plaid.AccountBase) error {
	for _, acc := range accounts {
		query := `
			INSERT INTO accounts (item_id, account_id, name, official_name, mask, type, subtype, current_balance, available_balance)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT DO NOTHING
		`

		_, err := pool.Exec(ctx, query,
			itemID,
			acc.GetAccountId(),
			acc.GetName(),
			acc.GetOfficialName(),
			acc.GetMask(),
			acc.GetType(),
			acc.GetSubtype(),
			acc.GetBalances().Current.Get(),
			acc.GetBalances().Available.Get(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func UpdatePlaidItemInstitution(ctx context.Context, pool *pgxpool.Pool, userID int64, institutionID string, institutionName string) error {
	query := `UPDATE plaid_items SET institution_id = $1, institution_name = $2, updated_at = NOW() WHERE user_id = $3 AND item_id IN (SELECT item_id FROM plaid_items WHERE user_id = $3 ORDER BY created_at DESC LIMIT 1)`
	_, err := pool.Exec(ctx, query, institutionID, institutionName, userID)
	return err
}

func DeletePlaidItem(ctx context.Context, pool *pgxpool.Pool, itemID string) error {
	query := `DELETE FROM plaid_items WHERE id = $1`
	_, err := pool.Exec(ctx, query, itemID)
	if err != nil {
		return fmt.Errorf("failed to delete plaid item: %w", err)
	}
	return nil
}

func GetAllPlaidItems(ctx context.Context, pool *pgxpool.Pool) ([]models.PlaidItem, error) {
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
	return items, rows.Err()
}
