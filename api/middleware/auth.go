package middleware

import (
	"net/http"
	"os"
	"strings"
)

// RequireAuthelia gates external traffic in production. Authelia authorizes
// requests at the Traefik ForwardAuth edge (browser cookie SSO or CLI opaque
// Bearer token, both by audience), so an authorized request reaches the API
// already carrying Authelia's forwarded identity in Remote-User. The API does
// no token validation of its own — the meso CLI's Authorization: Bearer token
// is opaque and is verified upstream at the edge, never here.
//
// A MESO_SERVICE_TOKEN Bearer path is kept for any internal service->service
// call on the docker network that bypasses the edge (none in v1, but the hook
// mirrors nomad so future internal callers have a path).
//
// /health is exempt: docker-compose healthchecks and k8s liveness/readiness
// probes hit it without credentials, and gating it would make orchestrators
// mark a healthy service unhealthy.
func RequireAuthelia(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if token := os.Getenv("MESO_SERVICE_TOKEN"); token != "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == token {
				next.ServeHTTP(w, r)
				return
			}
		}
		if r.Header.Get("Remote-User") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// DevCORS adds permissive CORS headers for the Vite dev server (port 3000).
// Only applied in development; in production Caddy and Authelia handle this.
func DevCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
