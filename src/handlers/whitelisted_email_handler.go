package handlers

import (
	db "budgee-server/src/db/sql"
	"budgee-server/src/util"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateWhitelistedEmail(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			log.Printf("ERROR: Failed to decode create whitelisted email request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if !util.ValidateEmail(req.Email) {
			log.Printf("ERROR: Email validation failed during creation - Email: %s", req.Email)
			http.Error(w, "invalid email format", http.StatusBadRequest)
			return
		}

		email, err := db.CreateWhitelistedEmail(r.Context(), pool, req.Email)
		if err != nil {
			log.Printf("ERROR: Failed to create whitelisted email: %v", err)
			http.Error(w, "failed to create", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Whitelisted email created - Email: %s, ID: %d", email.Email, email.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(email)
	}
}

func GetAllWhitelistedEmails(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emails, err := db.GetAllWhitelistedEmails(r.Context(), pool)
		if err != nil {
			log.Printf("ERROR: Failed to get whitelisted emails: %v", err)
			http.Error(w, "failed to get emails", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emails)
	}
}

func GetWhitelistedEmailByID(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			log.Printf("ERROR: Invalid whitelisted email id param: %s", idStr)
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		email, err := db.GetWhitelistedEmailByID(r.Context(), pool, id)
		if err != nil {
			log.Printf("ERROR: Whitelisted email not found - id: %d", id)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(email)
	}
}

func UpdateWhitelistedEmail(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			log.Printf("ERROR: Invalid whitelisted email id param for update: %s", idStr)
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			log.Printf("ERROR: Failed to decode update whitelisted email request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if !util.ValidateEmail(req.Email) {
			log.Printf("ERROR: Email validation failed during update - Email: %s", req.Email)
			http.Error(w, "invalid email format", http.StatusBadRequest)
			return
		}

		email, err := db.UpdateWhitelistedEmail(r.Context(), pool, id, req.Email)
		if err != nil {
			log.Printf("ERROR: Failed to update whitelisted email id %d: %v", id, err)
			http.Error(w, "failed to update", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Whitelisted email updated - Email: %s, ID: %d", email.Email, email.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(email)
	}
}

func DeleteWhitelistedEmail(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "email_id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			log.Printf("ERROR: Invalid whitelisted email id param for delete: %s", idStr)
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		err = db.DeleteWhitelistedEmail(r.Context(), pool, id)
		if err != nil {
			log.Printf("ERROR: Failed to delete whitelisted email id %d: %v", id, err)
			http.Error(w, "failed to delete", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Whitelisted email deleted - ID: %d", id)
		w.WriteHeader(http.StatusNoContent)
	}
}
