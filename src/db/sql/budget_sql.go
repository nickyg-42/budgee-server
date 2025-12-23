package db

import (
	"budgee-server/src/models"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateBudget(ctx context.Context, pool *pgxpool.Pool, budget *models.Budget) (*models.Budget, error) {
	query := `
		INSERT INTO budgets (user_id, amount, personal_finance_category)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, amount, personal_finance_category, created_at, updated_at
	`
	var b models.Budget
	err := pool.QueryRow(ctx, query, budget.UserID, budget.Amount, budget.PersonalFinanceCategory).
		Scan(&b.ID, &b.UserID, &b.Amount, &b.PersonalFinanceCategory, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func GetBudgetByID(ctx context.Context, pool *pgxpool.Pool, userID, budgetID int) (*models.Budget, error) {
	query := `
		SELECT id, user_id, amount, personal_finance_category, created_at, updated_at
		FROM budgets WHERE id = $1 AND user_id = $2
	`
	var b models.Budget
	err := pool.QueryRow(ctx, query, budgetID, userID).
		Scan(&b.ID, &b.UserID, &b.Amount, &b.PersonalFinanceCategory, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func GetBudgetByCategory(ctx context.Context, pool *pgxpool.Pool, userID int, category string) (*models.Budget, error) {
	query := `
		SELECT id, user_id, amount, personal_finance_category, created_at, updated_at
		FROM budgets WHERE user_id = $1 AND personal_finance_category = $2
	`
	var b models.Budget
	err := pool.QueryRow(ctx, query, userID, category).
		Scan(&b.ID, &b.UserID, &b.Amount, &b.PersonalFinanceCategory, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func GetAllBudgetsForUser(ctx context.Context, pool *pgxpool.Pool, userID int) ([]models.Budget, error) {
	query := `
		SELECT id, user_id, amount, personal_finance_category, created_at, updated_at
		FROM budgets WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var budgets []models.Budget
	for rows.Next() {
		var b models.Budget
		err := rows.Scan(&b.ID, &b.UserID, &b.Amount, &b.PersonalFinanceCategory, &b.CreatedAt, &b.UpdatedAt)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, rows.Err()
}

func UpdateBudget(ctx context.Context, pool *pgxpool.Pool, budget *models.Budget) (*models.Budget, error) {
	query := `
		UPDATE budgets
		SET amount = $1, personal_finance_category = $2, updated_at = NOW()
		WHERE id = $3 AND user_id = $4
		RETURNING id, user_id, amount, personal_finance_category, created_at, updated_at
	`
	var b models.Budget
	err := pool.QueryRow(ctx, query, budget.Amount, budget.PersonalFinanceCategory, budget.ID, budget.UserID).
		Scan(&b.ID, &b.UserID, &b.Amount, &b.PersonalFinanceCategory, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func DeleteBudget(ctx context.Context, pool *pgxpool.Pool, userID, budgetID int) error {
	query := `DELETE FROM budgets WHERE id = $1 AND user_id = $2`
	cmd, err := pool.Exec(ctx, query, budgetID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("budget not found")
	}
	return nil
}
