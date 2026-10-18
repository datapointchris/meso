package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/datapointchris/gobrowserlogin"

	"meso/api/auth"
)

// TokenVerifier is the seam the middleware depends on, so its routing can be
// tested without a live identity provider.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (auth.Identity, error)
}

// SessionAuthenticator is the same seam for the browser's session.
type SessionAuthenticator interface {
	Authenticate(r *http.Request) (gobrowserlogin.Identity, error)
}

// The browser login's routes, which a browser reaches before it holds a session.
const (
	LoginPath    = "/api/auth/login"
	CallbackPath = "/api/auth/callback"
)

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
// The browser SPA sends the session cookie this API set when it logged the
// browser in, and the CSRF header gobrowserlogin requires beside it. No header
// a client can set alone identifies anyone, so a request sent straight to the
// container, past the edge, proves nothing it did not prove there. A request
// carrying any bearer credential never reaches the session: falling through
// would let a caller pair a junk token with a stolen cookie.
//
// Each route puts the caller's groups on the request for RequireGroup: a JWT's
// groups claim, or the groups the provider's userinfo named at the session's
// login. The service token names no user and carries no group.
//
// /health is exempt: docker-compose healthchecks and k8s liveness/readiness
// probes hit this endpoint without credentials, and gating it would cause
// orchestrators to mark a healthy service as unhealthy. The RFC 9728 metadata
// is exempt too, because a CLI reads it to learn where to get its first token,
// and so are the login and its callback, which a browser reaches before it
// holds a session.
func RequireAuth(verifier TokenVerifier, serviceToken string, sessions SessionAuthenticator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/health", auth.ProtectedResourcePath, LoginPath, CallbackPath:
				next.ServeHTTP(w, r)
				return
			}

			if raw := auth.BearerToken(r.Header.Get("Authorization")); raw != "" {
				if auth.LooksLikeJWT(raw) {
					identity, err := verifier.Verify(r.Context(), raw)
					if errors.Is(err, auth.ErrProviderUnavailable) {
						logger.ErrorContext(r.Context(), "identity provider unreachable", "error", err, "path", r.URL.Path)
						refuse(w, http.StatusServiceUnavailable, "the identity provider is unavailable")
						return
					}
					if err != nil {
						logger.WarnContext(r.Context(), "bearer token rejected", "error", err, "path", r.URL.Path)
						refuse(w, http.StatusUnauthorized, noCredential)
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
				refuse(w, http.StatusUnauthorized, noCredential)
				return
			}

			identity, err := sessions.Authenticate(r)
			switch {
			case errors.Is(err, gobrowserlogin.ErrNoSession):
				refuse(w, http.StatusUnauthorized, noCredential)
				return
			case errors.Is(err, gobrowserlogin.ErrNoCSRFHeader):
				refuse(w, http.StatusForbidden, "a request carrying the session cookie must send "+gobrowserlogin.CSRFHeader)
				return
			case err != nil:
				logger.ErrorContext(r.Context(), "session store unreachable", "error", err, "path", r.URL.Path)
				refuse(w, http.StatusServiceUnavailable, "the session store is unavailable")
				return
			}
			ctx := context.WithValue(r.Context(), identityKey, auth.Identity{Subject: identity.Subject, Groups: identity.Groups})
			ctx = context.WithValue(ctx, groupsKey, identity.Groups)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// noCredential is every 401's message. It is one sentence whichever check
// failed, so a refusal tells a caller nothing about which credential came close.
const noCredential = "the request carries no credential this API accepts"

// refuse writes the {"error","message"} envelope every handler writes, which is
// the one shape a client decodes a reason from.
func refuse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": http.StatusText(status), "message": message})
}

// RequireGroup refuses with 403 a caller RequireAuth did not place in group.
func RequireGroup(group string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groups, _ := r.Context().Value(groupsKey).([]string)
		if !slices.Contains(groups, group) {
			refuse(w, http.StatusForbidden, "this needs membership in the "+group+" group")
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

// IdentityFrom returns the identity a bearer token or a browser session
// established. A session's carries no ClientID, and a request authorized by
// the service token carries none, because that token names no user.
func IdentityFrom(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityKey).(auth.Identity)
	return identity, ok
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
