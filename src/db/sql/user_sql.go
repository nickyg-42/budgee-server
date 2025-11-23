package db

import (
	"budgee-server/src/models"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetUserByID(id int, pool *pgxpool.Pool) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, username, email, first_name, last_name, password_hash, created_at
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
	)

	if err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func GetUserByUsername(username string, pool *pgxpool.Pool) (*models.User, error) {
	var user models.User
	query := `
        SELECT id, username, email, first_name, last_name, password_hash, created_at
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
		INSERT INTO users (first_name, last_name, username, email, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	var userID int

	err := pool.QueryRow(
		context.Background(),
		query,
		req.FirstName,
		req.LastName,
		req.Username,
		req.Email,
		hashedPassword,
	).Scan(&userID)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	resp := models.RegisterResponse{
		ID:       userID,
		Email:    req.Email,
		Username: req.Username,
	}

	return &resp, nil
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
