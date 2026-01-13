package db

import (
	"budgee-server/src/db"
	"budgee-server/src/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateTransactionRule(ctx context.Context, pool *pgxpool.Pool, rule *models.TransactionRule) (*models.TransactionRule, error) {
	query := `
		INSERT INTO transaction_rules (user_id, name, conditions, personal_finance_category)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, conditions, personal_finance_category, created_at, updated_at
	`
	var r models.TransactionRule
	err := pool.QueryRow(ctx, query, rule.UserID, rule.Name, rule.Conditions, rule.PersonalFinanceCategory).
		Scan(&r.ID, &r.UserID, &r.Name, &r.Conditions, &r.PersonalFinanceCategory, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func GetTransactionRuleByID(ctx context.Context, pool *pgxpool.Pool, userID, ruleID int) (*models.TransactionRule, error) {
	query := `
		SELECT id, user_id, name, conditions, personal_finance_category, created_at, updated_at
		FROM transaction_rules
		WHERE id = $1 AND user_id = $2
	`
	var r models.TransactionRule
	err := pool.QueryRow(ctx, query, ruleID, userID).
		Scan(&r.ID, &r.UserID, &r.Name, &r.Conditions, &r.PersonalFinanceCategory, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func GetAllTransactionRules(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]models.TransactionRule, error) {
	query := `
		SELECT id, user_id, name, conditions, personal_finance_category, created_at, updated_at
		FROM transaction_rules
		WHERE user_id = $1
		ORDER BY id
	`
	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.TransactionRule
	for rows.Next() {
		var r models.TransactionRule
		err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.Conditions, &r.PersonalFinanceCategory, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func UpdateTransactionRule(ctx context.Context, pool *pgxpool.Pool, rule *models.TransactionRule) (*models.TransactionRule, error) {
	query := `
		UPDATE transaction_rules
		SET name = $1, conditions = $2, personal_finance_category = $3, updated_at = NOW()
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, name, conditions, personal_finance_category, created_at, updated_at
	`
	var r models.TransactionRule
	err := pool.QueryRow(ctx, query, rule.Name, rule.Conditions, rule.PersonalFinanceCategory, rule.ID, rule.UserID).
		Scan(&r.ID, &r.UserID, &r.Name, &r.Conditions, &r.PersonalFinanceCategory, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func DeleteTransactionRule(ctx context.Context, pool *pgxpool.Pool, userID, ruleID int) error {
	query := `DELETE FROM transaction_rules WHERE id = $1 AND user_id = $2`
	cmd, err := pool.Exec(ctx, query, ruleID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("transaction rule not found")
	}
	return nil
}

func ApplyTransactionRulesToUser(ctx context.Context, pool *pgxpool.Pool, userID int64) error {
	rules, err := GetAllTransactionRules(ctx, pool, userID)
	if err != nil {
		return fmt.Errorf("failed to fetch transaction rules: %w", err)
	}
	if len(rules) == 0 {
		return nil
	}

	// Fetch all transactions for the user (across all accounts)
	query := `
        SELECT t.id, t.name, t.merchant_name, t.amount, a.name as account_name, t.primary_category
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        JOIN plaid_items p ON a.item_id = p.id
        WHERE p.user_id = $1
    `
	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to fetch transactions: %w", err)
	}
	defer rows.Close()

	type txnRow struct {
		ID           int
		Name         string
		MerchantName *string
		Amount       float64
		AccountName  string
		Category     *string
	}

	var txns []txnRow
	for rows.Next() {
		var row txnRow
		err := rows.Scan(&row.ID, &row.Name, &row.MerchantName, &row.Amount, &row.AccountName, &row.Category)
		if err != nil {
			return fmt.Errorf("failed to scan transaction: %w", err)
		}
		txns = append(txns, row)
	}

	// Track adjustments for logging
	type adjustment struct {
		TxnID    int
		OldValue string
		NewValue string
	}
	var adjusted []adjustment

	// For each transaction, apply rules
	for _, txn := range txns {
		for _, rule := range rules {
			var cond models.Condition
			if err := json.Unmarshal(rule.Conditions, &cond); err != nil {
				continue // skip invalid rule
			}
			if evaluateCondition(cond, txn) {
				// Update transaction's primary_category if it differs
				oldVal := ""
				if txn.Category != nil {
					oldVal = *txn.Category
				}
				if txn.Category == nil || *txn.Category != rule.PersonalFinanceCategory {
					_, err := pool.Exec(ctx, "UPDATE transactions SET primary_category = $1, updated_at = NOW() WHERE id = $2", rule.PersonalFinanceCategory, txn.ID)
					if err != nil {
						return fmt.Errorf("failed to update transaction category: %w", err)
					}
					adjusted = append(adjusted, adjustment{
						TxnID:    txn.ID,
						OldValue: oldVal,
						NewValue: rule.PersonalFinanceCategory,
					})
				}
				break // Stop at first matching rule
			}
		}
	}

	if len(adjusted) > 0 {
		log.Printf("ApplyTransactionRulesToUser: %d transactions adjusted by rules:", len(adjusted))
		for _, adj := range adjusted {
			log.Printf("  Transaction ID %d: primary_category changed from '%s' to '%s'", adj.TxnID, adj.OldValue, adj.NewValue)
		}
		db.ClearAllTransactionCaches()
	} else {
		log.Printf("ApplyTransactionRulesToUser: No transactions adjusted by rules for user %d", userID)
	}

	return nil
}

func evaluateCondition(cond models.Condition, txn struct {
	ID           int
	Name         string
	MerchantName *string
	Amount       float64
	AccountName  string
	Category     *string
}) bool {
	// Logical AND
	if len(cond.And) > 0 {
		for _, c := range cond.And {
			if !evaluateCondition(c, txn) {
				return false
			}
		}
		return true
	}
	// Logical OR
	if len(cond.Or) > 0 {
		for _, c := range cond.Or {
			if evaluateCondition(c, txn) {
				return true
			}
		}
		return false
	}
	// Leaf node: evaluate field/op/value
	var fieldValue interface{}
	switch cond.Field {
	case "name":
		fieldValue = txn.Name
	case "merchant_name":
		if txn.MerchantName != nil {
			fieldValue = *txn.MerchantName
		} else {
			fieldValue = ""
		}
	case "amount":
		fieldValue = txn.Amount
	case "account":
		fieldValue = txn.AccountName
	default:
		return false
	}
	switch cond.Op {
	case "equals":
		// Support both string and float64 equality
		switch v := fieldValue.(type) {
		case string:
			val, ok2 := cond.Value.(string)
			return ok2 && strings.EqualFold(v, val)
		case float64:
			val, ok2 := cond.Value.(float64)
			return ok2 && v == val
		default:
			return false
		}
	case "contains":
		s, ok := fieldValue.(string)
		val, ok2 := cond.Value.(string)
		return ok && ok2 && strings.Contains(strings.ToLower(s), strings.ToLower(val))
	case "gte":
		f, ok := fieldValue.(float64)
		val, ok2 := cond.Value.(float64)
		return ok && ok2 && f >= val
	case "lte":
		f, ok := fieldValue.(float64)
		val, ok2 := cond.Value.(float64)
		return ok && ok2 && f <= val
	case "gt":
		f, ok := fieldValue.(float64)
		val, ok2 := cond.Value.(float64)
		return ok && ok2 && f > val
	case "lt":
		f, ok := fieldValue.(float64)
		val, ok2 := cond.Value.(float64)
		return ok && ok2 && f < val
	case "in":
		s, ok := fieldValue.(string)
		arr, ok2 := cond.Value.([]interface{})
		if ok && ok2 {
			for _, v := range arr {
				if str, ok := v.(string); ok && strings.EqualFold(s, str) {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}
