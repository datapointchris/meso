package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/datapointchris/gobrowserlogin"

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

type fakeSessions struct {
	identity gobrowserlogin.Identity
	err      error
}

func (f fakeSessions) Authenticate(*http.Request) (gobrowserlogin.Identity, error) {
	return f.identity, f.err
}

var noSession = fakeSessions{err: gobrowserlogin.ErrNoSession}

func sessionIn(groups ...string) fakeSessions {
	return fakeSessions{identity: gobrowserlogin.Identity{Subject: "user-uuid", Groups: groups}}
}

func middlewareWith(v TokenVerifier, handler http.Handler) http.Handler {
	return middlewareWithSessions(v, "", noSession, handler)
}

func middlewareWithServiceToken(v TokenVerifier, serviceToken string, handler http.Handler) http.Handler {
	return middlewareWithSessions(v, serviceToken, noSession, handler)
}

func middlewareWithSessions(v TokenVerifier, serviceToken string, s SessionAuthenticator, handler http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return RequireAuth(v, serviceToken, s, logger)(handler)
}

func TestRequireGroup_AdmitsOnlyACallerRequireAuthPlacedInTheGroup(t *testing.T) {
	cases := []struct {
		name       string
		verifier   TokenVerifier
		sessions   SessionAuthenticator
		headers    map[string]string
		wantStatus int
	}{
		{"session member", nil, sessionIn("dev", "admin"), nil, http.StatusOK},
		{"session non-member", nil, sessionIn("dev", "administrators"), nil, http.StatusForbidden},
		{
			"JWT member", &fakeVerifier{identity: auth.Identity{Subject: "chris", Groups: []string{"admin"}}}, noSession,
			map[string]string{"Authorization": "Bearer a.b.c"},
			http.StatusOK,
		},
		{"service token", nil, noSession, map[string]string{"Authorization": "Bearer " + testServiceToken}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, _ := next()
			mw := middlewareWithSessions(tc.verifier, testServiceToken, tc.sessions, RequireGroup("admin", handler))
			req := httptest.NewRequest(http.MethodGet, "/api/v1/feedback", nil)
			for name, value := range tc.headers {
				req.Header.Set(name, value)
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// Every CLI decodes a reason only from the {"error","message"} envelope, so an
// admin command refused in plain text printed a bare 403.
func TestRequireGroup_RefusesInTheEnvelopeEveryCLIDecodes(t *testing.T) {
	handler, _ := next()
	mw := middlewareWithSessions(nil, "", sessionIn("dev"), RequireGroup("admin", handler))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feedback", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("refusal body %q is not the JSON envelope: %v", rec.Body.String(), err)
	}
	if rec.Code != http.StatusForbidden || envelope.Message != "this needs membership in the admin group" {
		t.Fatalf("status %d, envelope %+v", rec.Code, envelope)
	}
}

func TestDevGroups_PlacesEveryRequestInTheNamedGroups(t *testing.T) {
	handler, called := next()
	rec := httptest.NewRecorder()
	DevGroups(RequireGroup("admin", handler), "admin").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feedback", nil))
	if !*called || rec.Code != http.StatusOK {
		t.Fatalf("development request was refused: status %d", rec.Code)
	}
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

func TestRequireAuth_ProtectedResourceMetadataIsExempt(t *testing.T) {
	handler, called := next()
	mw := middlewareWith(&fakeVerifier{err: auth.ErrUnauthorized}, handler)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, auth.ProtectedResourcePath, nil))

	if !*called || rec.Code != http.StatusOK {
		t.Fatalf("a CLI with no token yet was refused the metadata: called %v, status %d", *called, rec.Code)
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

func TestRequireAuth_LoginRoutesAreExempt(t *testing.T) {
	for _, path := range []string{LoginPath, CallbackPath} {
		handler, called := next()
		mw := middlewareWith(&fakeVerifier{err: auth.ErrUnauthorized}, handler)

		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))

		if !*called {
			t.Errorf("%s was refused to a browser with no session", path)
		}
	}
}

// The SPA answers a 401 by sending the browser to log in. A missing CSRF header
// or a store outage is neither fixed by logging in, so neither may be a 401.
func TestRequireAuth_SessionFailuresAnswerWhatFixesThem(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{gobrowserlogin.ErrNoSession, http.StatusUnauthorized},
		{gobrowserlogin.ErrNoCSRFHeader, http.StatusForbidden},
		{errors.New("connection refused"), http.StatusServiceUnavailable},
	} {
		handler, called := next()
		mw := middlewareWithSessions(&fakeVerifier{err: auth.ErrUnauthorized}, "", fakeSessions{err: tc.err}, handler)

		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/movements", http.NoBody))

		if *called || rec.Code != tc.want {
			t.Errorf("%v: called %v, status %d, want %d", tc.err, *called, rec.Code, tc.want)
		}
	}
}

func TestRequireAuth_AcceptsVerifiedBearerAndExposesIdentity(t *testing.T) {
	want := auth.Identity{Subject: "user-uuid", ClientID: "meso-cli-laptop"}
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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identity = %+v, want %+v", got, want)
	}
}

// A rejected token must fail the request outright, rather than fall through to
// whatever session the same request carries.
func TestRequireAuth_RejectedBearerDoesNotFallBackToASession(t *testing.T) {
	handler, called := next()
	mw := middlewareWithSessions(&fakeVerifier{err: errors.New("bad signature")}, "", sessionIn("admin"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer forged.jwt.value")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("a rejected bearer token must not fall through to the session")
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
	mw := middlewareWithSessions(&fakeVerifier{err: auth.ErrUnauthorized}, testServiceToken, sessionIn("admin"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	req.Header.Set("Authorization", "Bearer wrong-service-token")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if *called {
		t.Fatal("a wrong service token must not authorize, nor fall through to the session")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// With no service token configured, the shared-secret path is closed and an
// opaque bearer credential is simply rejected.
func TestRequireAuth_RejectsOpaqueBearerWhenNoServiceTokenConfigured(t *testing.T) {
	handler, called := next()
	verifier := &fakeVerifier{identity: auth.Identity{Subject: "user-uuid", ClientID: "meso-cli-laptop"}}
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
