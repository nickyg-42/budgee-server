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
			t.id, t.account_id, t.transaction_id, t.primary_category, t.detailed_category, t.payment_channel, t.type, t.name, t.merchant_name,
			t.amount, t.currency, t.date, t.pending, t.account_owner, t.personal_finance_category_icon_url, t.created_at, t.updated_at
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

	return transactions, rows.Err()
}

func SaveTransactions(ctx context.Context, pool *pgxpool.Pool, userID int64, transactions []plaid.Transaction) error {
	for _, txn := range transactions {
		query := `
            INSERT INTO transactions (account_id, transaction_id, amount, name, date, primary_category, detailed_category, payment_channel, pending, type, merchant_name, currency, account_owner, personal_finance_category_icon_url, created_at)
            SELECT a.id, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW()
            FROM accounts a
            JOIN plaid_items p ON a.item_id = p.id
            WHERE p.user_id = $15 AND a.account_id = $1
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

		_, err := pool.Exec(ctx, query,
			txn.GetAccountId(),       // $1
			txn.GetTransactionId(),   // $2
			txn.GetAmount(),          // $3
			txn.GetName(),            // $4
			txn.GetDate(),            // $5
			primaryCategory,          // $6
			detailedCategory,         // $7
			txn.GetPaymentChannel(),  // $8
			txn.GetPending(),         // $9
			txn.GetTransactionType(), // $10
			txn.GetMerchantName(),    // $11
			txn.GetIsoCurrencyCode(), // $12
			txn.GetAccountOwner(),    // $13
			iconURL,                  // $14
			userID,                   // $15
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
			SET amount = $1, name = $2, date = $3, primary_category = $4, detailed_category = $5, payment_channel = $6, pending = $7, merchant_name = $8, currency = $9, account_owner = $10, personal_finance_category_icon_url = $11, updated_at = NOW()
			WHERE transaction_id = $12 AND account_id IN (
				SELECT a.id FROM accounts a
				JOIN plaid_items p ON a.item_id = p.id
				WHERE p.user_id = $13
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

		_, err := pool.Exec(ctx, query,
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
			txn.GetTransactionId(),   // $12
			userID,                   // $13
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

func GetPlaidItemByItemID(ctx context.Context, pool *pgxpool.Pool, itemID string) (*models.PlaidItem, error) {
	query := `SELECT id, user_id, access_token, item_id, institution_id, institution_name, created_at FROM plaid_items WHERE item_id = $1`

	var item models.PlaidItem
	err := pool.QueryRow(ctx, query, itemID).Scan(&item.ID, &item.UserID, &item.AccessToken, &item.ItemID, &item.InstitutionID, &item.InstitutionName, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
