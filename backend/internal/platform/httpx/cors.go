package httpx

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS configures cross-origin access for the web SPA. Because the SPA and API
// share a parent domain (*.swp.example.com) but live on different subdomains,
// requests are cross-origin: we must allow the specific origin(s) and set
// AllowCredentials so the SameSite=Lax refresh cookie is sent on /auth/refresh.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins, // explicit list; never "*" with credentials
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Idempotency-Key", "X-Request-Id", "Accept-Language"},
		ExposedHeaders:   []string{"X-Request-Id", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Location"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
