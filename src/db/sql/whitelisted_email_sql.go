package db

import (
	"budgee-server/src/models"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateWhitelistedEmail(ctx context.Context, pool *pgxpool.Pool, email string) (*models.WhitelistedEmail, error) {
	query := `
		INSERT INTO whitelisted_emails (email)
		VALUES ($1)
		RETURNING id, email, created_at, updated_at
	`
	var w models.WhitelistedEmail
	err := pool.QueryRow(ctx, query, email).Scan(&w.ID, &w.Email, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func GetAllWhitelistedEmails(ctx context.Context, pool *pgxpool.Pool) ([]models.WhitelistedEmail, error) {
	query := `SELECT id, email, created_at, updated_at FROM whitelisted_emails ORDER BY id`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.WhitelistedEmail
	for rows.Next() {
		var w models.WhitelistedEmail
		if err := rows.Scan(&w.ID, &w.Email, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		emails = append(emails, w)
	}
	return emails, rows.Err()
}

func GetWhitelistedEmailByID(ctx context.Context, pool *pgxpool.Pool, id int64) (*models.WhitelistedEmail, error) {
	query := `SELECT id, email, created_at, updated_at FROM whitelisted_emails WHERE id = $1`
	var w models.WhitelistedEmail
	err := pool.QueryRow(ctx, query, id).Scan(&w.ID, &w.Email, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func UpdateWhitelistedEmail(ctx context.Context, pool *pgxpool.Pool, id int64, email string) (*models.WhitelistedEmail, error) {
	query := `
		UPDATE whitelisted_emails
		SET email = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, email, created_at, updated_at
	`
	var w models.WhitelistedEmail
	err := pool.QueryRow(ctx, query, email, id).Scan(&w.ID, &w.Email, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func DeleteWhitelistedEmail(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	query := `DELETE FROM whitelisted_emails WHERE id = $1`
	cmd, err := pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("whitelisted email not found")
	}
	return nil
}
