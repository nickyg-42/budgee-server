package db

import (
	"budgee-server/src/models"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetUserByID(id int, pool *pgxpool.Pool) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, username, email, first_name, last_name, password_hash, created_at, theme, super_admin, last_login, locked
		FROM users 
		WHERE id = $1
	`
	err := pool.QueryRow(context.Background(), query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.Theme,
		&user.SuperAdmin,
		&user.LastLogin,
		&user.Locked,
	)

	if err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func GetUserByUsername(username string, pool *pgxpool.Pool) (*models.User, error) {
	var user models.User
	query := `
        SELECT id, username, email, first_name, last_name, password_hash, created_at, theme, super_admin, last_login, locked
        FROM users 
        WHERE username = $1
    `
	err := pool.QueryRow(context.Background(), query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.Theme,
		&user.SuperAdmin,
		&user.LastLogin,
		&user.Locked,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("query error: %w", err)
	}
	return &user, nil
}

func CreateUser(req models.RegisterRequest, hashedPassword string, pool *pgxpool.Pool) (*models.RegisterResponse, error) {
	query := `
		INSERT INTO users (first_name, last_name, username, email, password_hash, last_login)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, super_admin
	`

	var userID int
	var superAdmin bool

	now := time.Now().UTC()

	err := pool.QueryRow(
		context.Background(),
		query,
		req.FirstName,
		req.LastName,
		req.Username,
		req.Email,
		hashedPassword,
		now,
	).Scan(&userID, &superAdmin)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	resp := models.RegisterResponse{
		ID:         userID,
		Email:      req.Email,
		Username:   req.Username,
		SuperAdmin: superAdmin,
	}

	return &resp, nil
}

func UpdateUserLastLogin(pool *pgxpool.Pool, userID int64) error {
	query := `
		UPDATE users
		SET last_login = $1
		WHERE id = $2
	`
	_, err := pool.Exec(context.Background(), query, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("failed to update last_login: %w", err)
	}
	return nil
}

func DeleteUser(userID int, pool *pgxpool.Pool) error {
	query := `
		DELETE FROM users
		WHERE id = $1;
	`
	_, err := pool.Exec(
		context.Background(),
		query,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func UpdateUserProfile(ctx context.Context, pool *pgxpool.Pool, userID int64, email string, firstName string, lastName string, theme string) error {
	query := `
		UPDATE users
		SET email = $1, first_name = $2, last_name = $3, theme = $4, updated_at = NOW()
		WHERE id = $5
	`
	_, err := pool.Exec(ctx, query, email, firstName, lastName, theme, userID)
	if err != nil {
		return fmt.Errorf("failed to update user profile: %w", err)
	}
	return nil
}

func UpdateUserPassword(ctx context.Context, pool *pgxpool.Pool, userID int64, hashedPassword string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := pool.Exec(ctx, query, hashedPassword, userID)
	if err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}
	return nil
}

func GetAllUsers(pool *pgxpool.Pool) ([]models.User, error) {
	query := `
		SELECT id, username, email, first_name, last_name, password_hash, created_at, theme, super_admin, last_login, locked
		FROM users
		ORDER BY id
	`
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.FirstName,
			&user.LastName,
			&user.PasswordHash,
			&user.CreatedAt,
			&user.Theme,
			&user.SuperAdmin,
			&user.LastLogin,
			&user.Locked,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("row error: %w", rows.Err())
	}
	return users, nil
}

func LockUser(ctx context.Context, pool *pgxpool.Pool, userID int64) error {
	query := `UPDATE users SET locked = TRUE, updated_at = NOW() WHERE id = $1`
	_, err := pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to lock user: %w", err)
	}
	return nil
}

func UnlockUser(ctx context.Context, pool *pgxpool.Pool, userID int64) error {
	query := `UPDATE users SET locked = FALSE, updated_at = NOW() WHERE id = $1`
	_, err := pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to unlock user: %w", err)
	}
	return nil
}
