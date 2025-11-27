package handlers

import (
	db "budgee-server/src/db/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		requestedUserID := chi.URLParam(r, "user_id")

		parsedUserID, err := strconv.ParseInt(requestedUserID, 10, 64)
		if err != nil {
			log.Printf("ERROR: Failed to parse user_id from URL - user_id: %s: %v", requestedUserID, err)
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}

		if userID != parsedUserID {
			log.Printf("ERROR: Unauthorized user access attempt - Authenticated user: %d, Requested user: %d", userID, parsedUserID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		user, err := db.GetUserByID(int(userID), pool)
		if err != nil {
			log.Printf("ERROR: Failed to get user - user_id: %d: %v", userID, err)
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}
