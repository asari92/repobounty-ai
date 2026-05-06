package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/repobounty/repobounty-ai/internal/store"
)

type contextKey string

var userContextKey contextKey = "user"

type AuthMiddleware struct {
	jwtMgr *JWTManager
	store  store.CampaignStore
}

func NewAuthMiddleware(jwtMgr *JWTManager, s store.CampaignStore) *AuthMiddleware {
	return &AuthMiddleware{
		jwtMgr: jwtMgr,
		store:  s,
	}
}

func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		claims, err := a.jwtMgr.ValidateToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		githubID, err := strconv.Atoi(claims.Sub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token subject")
			return
		}

		user, err := a.store.GetUserByGitHubID(githubID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserFromContext(ctx context.Context) (*store.User, bool) {
	user, ok := ctx.Value(userContextKey).(*store.User)
	return user, ok
}

func OptionalAuthMiddleware(jwtMgr *JWTManager, s store.CampaignStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := jwtMgr.ValidateToken(parts[1])
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			githubID, err := strconv.Atoi(claims.Sub)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			user, err := s.GetUserByGitHubID(githubID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"error": msg}); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}
