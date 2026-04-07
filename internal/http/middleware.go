package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"aggregatorIB/internal/auth"
)

type contextKey string

const userClaimsKey contextKey = "userClaims"

func (a *App) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := readToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "требуется авторизация"})
			return
		}

		claims, err := auth.ParseToken(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "сессия недействительна"})
			return
		}

		ctx := context.WithValue(r.Context(), userClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func (a *App) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return a.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims := userFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "доступ только для администратора"})
			return
		}
		next(w, r)
	})
}

func userFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(userClaimsKey).(*auth.Claims)
	return claims
}

func readToken(r *http.Request) string {
	if cookie, err := r.Cookie("token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
