package middleware

import (
	db "budgee-server/src/db/sql"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ParseTokenFromRequest extracts and validates JWT token from request, returning claims if valid
func ParseTokenFromRequest(r *http.Request) (jwt.MapClaims, error) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		return nil, fmt.Errorf("missing token")
	}

	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method")
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func JWTAuthMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := ParseTokenFromRequest(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			username := claims["username"].(string)
			userID := claims["user_id"].(float64)
			superAdmin := false
			if v, ok := claims["super_admin"].(bool); ok {
				superAdmin = v
			}

			// Check if user is locked
			user, err := db.GetUserByID(int(userID), pool)
			if err == nil && user.Locked {
				http.Error(w, "user account is locked", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), "username", username)
			ctx = context.WithValue(ctx, "user_id", int64(userID))
			ctx = context.WithValue(ctx, "super_admin", superAdmin)

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func SuperAdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		superAdmin, ok := r.Context().Value("super_admin").(bool)
		if !ok || !superAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
