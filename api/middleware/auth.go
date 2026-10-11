package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"meso/api/auth"
)

// TokenVerifier is the seam the middleware depends on, so its routing can be
// tested without a live identity provider.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (auth.Identity, error)
}

type contextKey string

const (
	identityKey contextKey = "identity"
	groupsKey   contextKey = "groups"
)

// RequireAuth authorizes a request by one of three routes.
//
// The meso CLI sends an RFC 9068 JWT access token, which is verified here
// against Authelia's JWKS — signature, issuer, expiry, and a client_id
// belonging to this product. Nothing outside the API is trusted for that path.
//
// An internal service-to-service caller on the docker network sends the
// MESO_SERVICE_TOKEN shared secret, compared byte-for-byte in constant time.
// Which of the two a bearer credential is judged as comes from its shape alone,
// decided before either check runs, so a caller cannot have one credential
// graded by the other's rules. A service token given a JWT shape would be sent
// to the verifier and rejected there.
//
// The browser SPA authenticates with an Authelia session cookie and arrives
// with a Remote-User header injected by ForwardAuth at the Traefik edge. That
// path trusts the header, which is only sound for requests that actually
// traverse Traefik. A request carrying any bearer credential never reaches it:
// falling through would let a caller pair a junk token with a forged header.
//
// Each route puts the caller's groups on the request for RequireGroup: a JWT's
// groups claim, or the edge's Remote-Groups header, which the edge strips from
// every inbound request before ForwardAuth sets it. The service token names no
// user and carries no group.
//
// /health is exempt: docker-compose healthchecks and k8s liveness/readiness
// probes hit this endpoint without credentials, and gating it would cause
// orchestrators to mark a healthy service as unhealthy.
func RequireAuth(verifier TokenVerifier, serviceToken string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			if raw := auth.BearerToken(r.Header.Get("Authorization")); raw != "" {
				if auth.LooksLikeJWT(raw) {
					identity, err := verifier.Verify(r.Context(), raw)
					if errors.Is(err, auth.ErrProviderUnavailable) {
						logger.ErrorContext(r.Context(), "identity provider unreachable", "error", err, "path", r.URL.Path)
						http.Error(w, "identity provider unavailable", http.StatusServiceUnavailable)
						return
					}
					if err != nil {
						logger.WarnContext(r.Context(), "bearer token rejected", "error", err, "path", r.URL.Path)
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					ctx := context.WithValue(r.Context(), identityKey, identity)
					ctx = context.WithValue(ctx, groupsKey, identity.Groups)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				if serviceToken != "" && subtle.ConstantTimeCompare([]byte(raw), []byte(serviceToken)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
				logger.WarnContext(r.Context(), "service token rejected", "path", r.URL.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if r.Header.Get("Remote-User") == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), groupsKey, edgeGroups(r.Header.Get("Remote-Groups")))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// edgeGroups splits the comma-separated Remote-Groups header ForwardAuth sets.
func edgeGroups(header string) []string {
	var groups []string
	for group := range strings.SplitSeq(header, ",") {
		if group = strings.TrimSpace(group); group != "" {
			groups = append(groups, group)
		}
	}
	return groups
}

// RequireGroup refuses with 403 a caller RequireAuth did not place in group.
func RequireGroup(group string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groups, _ := r.Context().Value(groupsKey).([]string)
		if !slices.Contains(groups, group) {
			http.Error(w, "forbidden: this needs membership in the "+group+" group", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// DevGroups places every request in groups. Development runs no identity
// provider and no RequireAuth, so every caller is the developer.
func DevGroups(next http.Handler, groups ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), groupsKey, groups)))
	})
}

// IdentityFrom returns the identity established by a verified bearer token.
// Requests authorized by the edge or by the service token carry no identity
// here — the edge carries Remote-User instead, and the service token names no
// user.
func IdentityFrom(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityKey).(auth.Identity)
	return identity, ok
}

// DevCORS adds permissive CORS headers for the Vite dev server (port 3000).
// Only applied in development; in production Caddy and Authelia handle this.
func DevCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
