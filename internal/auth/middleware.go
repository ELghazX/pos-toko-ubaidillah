package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

func isJSONRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

func RequireAuth(svc Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := ParseSessionCookie(r)
			if !ok {
				handleUnauthenticated(w, r)
				return
			}

			user, err := svc.GetSessionUserByID(r.Context(), userID)
			if err != nil || user == nil {
				ClearSessionCookie(w)
				handleUnauthenticated(w, r)
				return
			}

			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			handleUnauthenticated(w, r)
			return
		}

		if !user.IsAdmin() {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/pos")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if isJSONRequest(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   "Akses ditolak: hanya Admin yang diizinkan mengakses fitur ini",
				})
				return
			}
			http.Redirect(w, r, "/pos", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if isJSONRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Sesi berakhir, silakan login kembali",
		})
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
