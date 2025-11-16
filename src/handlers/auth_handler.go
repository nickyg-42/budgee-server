package handlers

import (
	"budgee-server/src/models"
	"budgee-server/src/util"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func Register(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(req.Email)
		req.Username = strings.TrimSpace(req.Username)

		if !util.ValidateEmail(req.Email) || !util.ValidateUsername(req.Username) || !util.ValidatePassword(req.Password) {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Insert into DB
		var userID int
		query := `INSERT INTO users (email, username, password_hash) 
                  VALUES ($1, $2, $3) RETURNING id`
		err = pool.QueryRow(context.Background(), query, req.Email, req.Username, string(hashedPassword)).Scan(&userID)
		if err != nil {
			// Handle duplicate key
			if strings.Contains(err.Error(), "duplicate key") {
				http.Error(w, "email or username already exists", http.StatusConflict)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := models.RegisterResponse{
			ID:       userID,
			Email:    req.Email,
			Username: req.Username,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

// func Login(pool *pgxpool.Pool) http.HandlerFunc {

// }
