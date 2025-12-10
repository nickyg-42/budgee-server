package handlers

import (
	db "budgee-server/src/db/sql"
	"budgee-server/src/models"
	"budgee-server/src/util"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func Register(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode register request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(req.Email)
		req.Username = strings.TrimSpace(req.Username)

		// Gmail whitelist check
		allowedGmails := strings.Split(os.Getenv("ALLOWED_GMAILS"), ",")
		emailAllowed := false
		for _, allowed := range allowedGmails {
			if strings.EqualFold(strings.TrimSpace(allowed), req.Email) {
				emailAllowed = true
				break
			}
		}
		if !emailAllowed {
			log.Printf("ERROR: Registration denied for non-whitelisted email: %s", req.Email)
			http.Error(w, "registration is restricted to invited emails", http.StatusForbidden)
			return
		}

		if !util.ValidateEmail(req.Email) {
			log.Printf("ERROR: Email validation failed during registration - Email: %s", req.Email)
			http.Error(w, "invalid email format", http.StatusBadRequest)
			return
		}

		if !util.ValidateUsername(req.Username) {
			log.Printf("ERROR: Username validation failed during registration - Username: %s", req.Username)
			http.Error(w, "username must be between 3 and 30 characters", http.StatusBadRequest)
			return
		}

		if !util.ValidatePassword(req.Password) {
			log.Printf("ERROR: Password validation failed during registration - Username: %s", req.Username)
			http.Error(w, "password must be at least 8 characters with uppercase, lowercase, digit, and special character", http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("ERROR: Failed to hash password for user %s: %v", req.Username, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var resp *models.RegisterResponse
		resp, err = db.CreateUser(req, string(hashedPassword), pool)
		if err != nil {
			// Handle duplicate key
			if strings.Contains(err.Error(), "duplicate key") {
				log.Printf("ERROR: Registration failed - email or username already exists - Email: %s, Username: %s", req.Email, req.Username)
				http.Error(w, "email or username already exists", http.StatusConflict)
				return
			}
			log.Printf("ERROR: Failed to create user %s: %v", req.Username, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: Successful registration - User: %s, ID: %d", resp.Username, resp.ID)

		// Generate JWT token for the new user
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":  resp.ID,
			"username": resp.Username,
			"exp":      time.Now().Add(time.Hour * 504).Unix(),
		})

		tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
		if err != nil {
			log.Printf("ERROR: Failed to generate JWT token for user %s: %v", resp.Username, err)
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"token": tokenString,
		})
	}
}

func Login(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		var credentials struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			log.Printf("ERROR: Failed to decode login request body: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		user, err := db.GetUserByUsername(strings.ToLower(credentials.Username), pool)
		if err != nil {
			log.Printf("ERROR: Failed to find user during login - Username: %s: %v", credentials.Username, err)
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(credentials.Password)); err != nil {
			log.Printf("ERROR: Invalid password attempt for user %s from IP %s",
				credentials.Username, r.RemoteAddr)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Create the JWT token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":  user.ID,
			"username": user.Username,
			"exp":      time.Now().Add(time.Hour * 504).Unix(),
		})

		tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
		if err != nil {
			log.Printf("ERROR: Failed to generate JWT token for user %s: %v",
				user.Username, err)
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		log.Printf("INFO: Successful login - User: %s, ID: %d", user.Username, user.ID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": tokenString,
		})
	}
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
