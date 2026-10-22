package middleware

import (
	"net/http"

	"github.com/datapointchris/goapiauth"
	"github.com/datapointchris/gobrowserlogin"
)

// The browser login's routes, which a browser reaches before it holds a session.
const (
	LoginPath    = "/api/auth/login"
	CallbackPath = "/api/auth/callback"
)

// DevGroups places every request in groups, as the developer. Development runs
// no identity provider, so goapiauth.RequireGroup reads this identity instead.
func DevGroups(next http.Handler, groups ...string) http.Handler {
	developer := goapiauth.Identity{Subject: "developer", Groups: groups}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(goapiauth.WithIdentity(r.Context(), developer)))
	})
}

// DevCORS adds permissive CORS headers for the Vite dev server, on the port
// web/vite.config.ts serves it from. Only applied in development; in production
// Caddy serves the SPA and the API from one origin.
func DevCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3001")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+gobrowserlogin.CSRFHeader)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
