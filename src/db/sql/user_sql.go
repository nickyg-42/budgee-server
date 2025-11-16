package db

import (
	"budgee-server/src/models"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func GetUserByID(ctx context.Context, id int, pool *pgxpool.Pool) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, username, email, first_name, last_name, password_hash, role, created_at
		FROM users 
		WHERE id = $1
	`
	err := pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func GetUserByUsername(ctx context.Context, username string, pool *pgxpool.Pool) (*models.User, error) {
	var user models.User
	query := `
        SELECT id, username, email, first_name, last_name, password_hash, role, created_at
        FROM users 
        WHERE username = $1
    `
	err := pool.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("query error: %w", err)
	}
	return &user, nil
}

func IsUsernameTaken(ctx context.Context, username string, pool *pgxpool.Pool) (bool, error) {
	query := `
		SELECT 1
		FROM users
		WHERE username = $1
	`
	var result int

	err := pool.QueryRow(ctx, query, username).Scan(&result)

	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("query error: %w", err)
	}
	return true, nil
}

func CreateUser(ctx context.Context, user *models.User, pool *pgxpool.Pool) (*models.User, error) {
	query := `
		INSERT INTO users (first_name, last_name, username, email, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := pool.QueryRow(
		ctx,
		query,
		user.FirstName,
		user.LastName,
		user.Username,
		user.Email,
		user.PasswordHash,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func DeleteUser(ctx context.Context, userID int, pool *pgxpool.Pool) error {
	query := `
		DELETE FROM users
		WHERE id = $1;
	`
	_, err := pool.Exec(
		ctx,
		query,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
