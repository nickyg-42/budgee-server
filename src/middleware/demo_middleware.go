package middleware

import (
	"net/http"
)

func DemoModeMiddleware(isDemo bool) func(http.Handler) http.Handler {
	allowedPosts := map[string]bool{
		"/api/login":         true,
		"/api/register":      true,
		"/api/plaid/webhook": true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if superAdmin, ok := r.Context().Value("super_admin").(bool); ok && superAdmin {
				next.ServeHTTP(w, r)
				return
			}

			if isDemo && r.Method != http.MethodGet {
				if r.Method == http.MethodPost && allowedPosts[r.URL.Path] {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "Demo mode: only GET requests are allowed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
