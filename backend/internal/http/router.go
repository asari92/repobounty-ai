package http

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/repobounty/repobounty-ai/internal/auth"
)

func NewRouter(h *Handlers, env string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RecoveryMiddleware)
	r.Use(StructuredLogger)
	r.Use(RateLimitMiddleware(100, 20))

	var allowedOrigins []string
	if env == "production" {
		originsStr := os.Getenv("ALLOWED_ORIGINS")
		if originsStr != "" {
			allowedOrigins = strings.Split(originsStr, ",")
			for i := range allowedOrigins {
				allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
			}
		} else {
			log.Printf("WARNING: ALLOWED_ORIGINS not set in production — all cross-origin requests will be blocked")
		}
	} else {
		allowedOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
	r.Use(CorsMiddleware(allowedOrigins))

	authMiddleware := auth.NewAuthMiddleware(h.jwt, h.store)
	requireAuth := authMiddleware.RequireAuth
	optionalAuth := auth.OptionalAuthMiddleware(h.jwt, h.store)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.HealthCheck)

		r.Route("/auth", func(r chi.Router) {
			r.Get("/github/url", h.GetGitHubAuthURL)
			r.Post("/github/callback", h.GitHubCallback)
			r.With(requireAuth).Get("/me", h.GetMe)
			r.With(requireAuth).Post("/wallet/link", h.LinkWallet)
			r.With(requireAuth).Get("/claims", h.GetClaims)
		})

		r.Route("/campaigns", func(r chi.Router) {
			r.Use(optionalAuth)
			r.Get("/", h.ListCampaigns)
			r.Get("/{id}", h.GetCampaign)
			r.With(requireAuth).Post("/", h.CreateCampaign)
			r.With(requireAuth).Post("/{id}/finalize-preview", h.FinalizePreview)
			r.With(requireAuth).Post("/{id}/finalize", h.Finalize)
			r.With(requireAuth).Post("/{id}/claim", h.ClaimPermit)
			r.With(requireAuth).Post("/{id}/fund-tx", h.FundTx)
		})
	})

	return r
}
