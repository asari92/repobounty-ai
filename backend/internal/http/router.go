package http

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
		}
	} else {
		allowedOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
	r.Use(CorsMiddleware(allowedOrigins))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.HealthCheck)

		r.Route("/campaigns", func(r chi.Router) {
			r.Get("/", h.ListCampaigns)
			r.Post("/", h.CreateCampaign)
			r.Get("/{id}", h.GetCampaign)
			r.Post("/{id}/finalize-preview", h.FinalizePreview)
			r.Post("/{id}/finalize", h.Finalize)
		})
	})

	return r
}
