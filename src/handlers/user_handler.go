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
	"golang.org/x/crypto/bcrypt"
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

func UpdateUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		var req struct {
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode update user request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if !util.ValidateEmail(req.Email) {
			log.Printf("ERROR: Email validation failed during user update - Email: %s, User: %d", req.Email, userID)
			http.Error(w, "invalid email format", http.StatusBadRequest)
			return
		}

		err := db.UpdateUserProfile(r.Context(), pool, userID, req.Email, req.FirstName, req.LastName)
		if err != nil {
			log.Printf("ERROR: Failed to update user profile - user_id: %d: %v", userID, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: User profile updated - User: %d", userID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "profile updated successfully",
		})
	}
}

func ChangePassword(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode change password request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		user, err := db.GetUserByID(int(userID), pool)
		if err != nil {
			log.Printf("ERROR: Failed to get user for password change - user_id: %d: %v", userID, err)
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.CurrentPassword)); err != nil {
			log.Printf("ERROR: Invalid current password attempt for user %d", userID)
			http.Error(w, "current password is incorrect", http.StatusUnauthorized)
			return
		}

		if !util.ValidatePassword(req.NewPassword) {
			log.Printf("ERROR: Password validation failed during change password - User: %d", userID)
			http.Error(w, "password must be at least 8 characters with uppercase, lowercase, digit, and special character", http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("ERROR: Failed to hash new password for user %d: %v", userID, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		err = db.UpdateUserPassword(r.Context(), pool, userID, string(hashedPassword))
		if err != nil {
			log.Printf("ERROR: Failed to update user password - user_id: %d: %v", userID, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: User password changed - User: %d", userID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "password changed successfully",
		})
	}
}

func DeleteUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)

		log.Printf("INFO: DeleteUser called for user_id: %d", userID)

		// Security: Only allow user to delete themselves
		var req struct {
			UserID int64 `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode delete user request body for user_id: %d: %v", userID, err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if req.UserID != userID {
			log.Printf("ERROR: Forbidden delete attempt - Authenticated user: %d, Requested user: %d", userID, req.UserID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		log.Printf("INFO: Deleting user %d and all associated data", userID)
		err := db.DeleteUser(int(userID), pool)
		if err != nil {
			log.Printf("ERROR: Failed to delete user %d: %v", userID, err)
			http.Error(w, "failed to delete user", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: User %d deleted successfully. Instructing client to remove JWT and redirect.", userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message":  "user deleted",
			"redirect": "/register",
		})
	}
}
