package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"meso/api/auth"
)

const testServiceToken = "s3cr3t-service-token"

// next is a tiny stand-in handler used to verify whether the middleware
// passed the request through. It records that it was called and writes 200.
func next() (http.Handler, *bool) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &called
}

type fakeVerifier struct {
	identity auth.Identity
	err      error
	calls    int
}

func (f *fakeVerifier) Verify(context.Context, string) (auth.Identity, error) {
	f.calls++
	return f.identity, f.err
}

func middlewareWith(v TokenVerifier, handler http.Handler) http.Handler {
	return middlewareWithServiceToken(v, "", handler)
}

func middlewareWithServiceToken(v TokenVerifier, serviceToken string, handler http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return RequireAuth(v, serviceToken, logger)(handler)
}

func TestRequireAuth_HealthIsExempt(t *testing.T) {
	handler, called := next()
	mw := middlewareWith(&fakeVerifier{err: auth.ErrUnauthorized}, handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !*called {
		t.Fatal("/health should bypass auth, but the next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("/health should return 200 even without auth, got %d", rec.Code)
	}
}

func TestRequireAuth_RejectsUnauthenticated(t *testing.T) {
	handler, called := next()
	mw := middlewareWith(&fakeVerifier{err: auth.ErrUnauthorized}, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("unauthenticated request should not reach the next handler")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuth_AcceptsAutheliaHeader(t *testing.T) {
	handler, called := next()
	mw := middlewareWith(&fakeVerifier{err: auth.ErrUnauthorized}, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Remote-User", "chris")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !*called {
		t.Fatal("Remote-User header should authenticate the request")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireAuth_AcceptsVerifiedBearerAndExposesIdentity(t *testing.T) {
	want := auth.Identity{Subject: "user-uuid", ClientID: "meso-cli-macmini"}
	var got auth.Identity
	var ok bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = IdentityFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := middlewareWith(&fakeVerifier{identity: want}, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer some.jwt.value")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !ok {
		t.Fatal("handler could not read the identity from the request context")
	}
	if got != want {
		t.Fatalf("identity = %+v, want %+v", got, want)
	}
}

// A rejected token must fail the request outright. Falling through to the
// Remote-User path would let a caller pair a junk token with a forged header.
func TestRequireAuth_RejectedBearerDoesNotFallBackToHeader(t *testing.T) {
	handler, called := next()
	mw := middlewareWith(&fakeVerifier{err: errors.New("bad signature")}, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer forged.jwt.value")
	req.Header.Set("Remote-User", "chris")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("a rejected bearer token must not fall through to Remote-User")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuth_AcceptsServiceToken(t *testing.T) {
	handler, called := next()
	verifier := &fakeVerifier{err: auth.ErrUnauthorized}
	mw := middlewareWithServiceToken(verifier, testServiceToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer "+testServiceToken)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !*called {
		t.Fatal("the service token should authorize the request")
	}
	if verifier.calls != 0 {
		t.Fatalf("the service token must not be graded against JWKS, got %d verifier calls", verifier.calls)
	}
}

// The service token authorizes no user, so nothing may read an identity from a
// request it authorized.
func TestRequireAuth_ServiceTokenCarriesNoIdentity(t *testing.T) {
	var ok bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = IdentityFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := middlewareWithServiceToken(&fakeVerifier{err: auth.ErrUnauthorized}, testServiceToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer "+testServiceToken)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ok {
		t.Fatal("a service-token request must carry no identity")
	}
}

func TestRequireAuth_RejectsWrongServiceToken(t *testing.T) {
	handler, called := next()
	mw := middlewareWithServiceToken(&fakeVerifier{err: auth.ErrUnauthorized}, testServiceToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer wrong-service-token")
	req.Header.Set("Remote-User", "chris")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("a wrong service token must not authorize, nor fall through to Remote-User")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// With no service token configured, the shared-secret path is closed and an
// opaque bearer credential is simply rejected.
func TestRequireAuth_RejectsOpaqueBearerWhenNoServiceTokenConfigured(t *testing.T) {
	handler, called := next()
	verifier := &fakeVerifier{identity: auth.Identity{Subject: "user-uuid", ClientID: "meso-cli-macmini"}}
	mw := middlewareWithServiceToken(verifier, "", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer opaque-value")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("an opaque bearer credential must not authorize when no service token is set")
	}
	if verifier.calls != 0 {
		t.Fatalf("a non-JWT credential must never reach the verifier, got %d calls", verifier.calls)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// A JWT-shaped credential is judged by JWKS alone, so it can never be accepted
// by matching the shared secret.
func TestRequireAuth_JWTShapedServiceTokenGoesToTheVerifier(t *testing.T) {
	handler, called := next()
	const jwtShaped = "aaa.bbb.ccc"
	verifier := &fakeVerifier{err: auth.ErrUnauthorized}
	mw := middlewareWithServiceToken(verifier, jwtShaped, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer "+jwtShaped)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("a JWT-shaped credential must not be authorized by the secret comparison")
	}
	if verifier.calls != 1 {
		t.Fatalf("expected the verifier to judge a JWT-shaped credential, got %d calls", verifier.calls)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
