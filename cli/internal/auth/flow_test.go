package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"meso/cli/internal/config"
)

// mockIDP is a minimal Authelia stand-in implementing the native bearer.authz
// flow: discovery, PAR, an authorization endpoint that plays the browser by
// form_post-ing the code back to the loopback, and a PKCE-checked token
// endpoint.
type mockIDP struct {
	server *httptest.Server

	mu       sync.Mutex
	parByURI map[string]url.Values // request_uri -> pushed params
	codes    map[string]codeRecord // code -> pkce challenge + redirect
}

type codeRecord struct {
	codeChallenge string
	redirectURI   string
}

func newMockIDP(t *testing.T) *mockIDP {
	t.Helper()
	idp := &mockIDP{
		parByURI: map[string]url.Values{},
		codes:    map[string]codeRecord{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("/par", idp.par)
	mux.HandleFunc("/authorize", idp.authorize)
	mux.HandleFunc("/token", idp.token)
	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

func (m *mockIDP) discovery(w http.ResponseWriter, _ *http.Request) {
	base := m.server.URL
	_ = json.NewEncoder(w).Encode(map[string]string{
		"issuer":                                base,
		"authorization_endpoint":                base + "/authorize",
		"token_endpoint":                        base + "/token",
		"pushed_authorization_request_endpoint": base + "/par",
	})
}

func (m *mockIDP) par(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.FormValue("response_mode") != "form_post" {
		http.Error(w, "expected form_post", http.StatusBadRequest)
		return
	}
	if r.FormValue("code_challenge_method") != "S256" || r.FormValue("code_challenge") == "" {
		http.Error(w, "expected PKCE S256", http.StatusBadRequest)
		return
	}
	requestURI := "urn:ietf:params:oauth:request_uri:" + r.FormValue("state")
	m.mu.Lock()
	m.parByURI[requestURI] = r.Form
	m.mu.Unlock()

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"request_uri": requestURI, "expires_in": 60})
}

// authorize plays the browser: it resolves the pushed request and form_posts
// the authorization code to the client's loopback redirect_uri.
func (m *mockIDP) authorize(w http.ResponseWriter, r *http.Request) {
	requestURI := r.URL.Query().Get("request_uri")
	m.mu.Lock()
	params, ok := m.parByURI[requestURI]
	m.mu.Unlock()
	if !ok {
		http.Error(w, "unknown request_uri", http.StatusBadRequest)
		return
	}

	code := "auth-code-123"
	m.mu.Lock()
	m.codes[code] = codeRecord{codeChallenge: params.Get("code_challenge"), redirectURI: params.Get("redirect_uri")}
	m.mu.Unlock()

	resp, err := http.PostForm(params.Get("redirect_uri"), url.Values{
		"code":  {code},
		"state": {params.Get("state")},
	})
	if err != nil {
		http.Error(w, "callback failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	resp.Body.Close()
	w.WriteHeader(http.StatusOK)
}

func (m *mockIDP) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	code := r.FormValue("code")
	m.mu.Lock()
	rec, ok := m.codes[code]
	m.mu.Unlock()
	if !ok {
		http.Error(w, "unknown code", http.StatusBadRequest)
		return
	}
	// Verify PKCE: base64url(sha256(verifier)) must equal the pushed challenge.
	sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != rec.codeChallenge {
		http.Error(w, "PKCE verification failed", http.StatusBadRequest)
		return
	}
	if r.FormValue("redirect_uri") != rec.redirectURI {
		http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "authelia_at_opaque_value",
		"refresh_token": "authelia_rt_opaque_value",
		"token_type":    "bearer",
		"expires_in":    3600,
	})
}

func TestLogin_FullPARFormPostFlow(t *testing.T) {
	idp := newMockIDP(t)
	cfg := config.Config{
		Issuer:   idp.server.URL,
		ClientID: "meso-cli-macmini",
		Audience: "https://meso.ichrisbirch.com",
	}

	// The opener plays the human clicking the link: fetching the authorization
	// URL triggers the mock to form_post the code to the loopback listener.
	opener := func(authURL string) error {
		resp, err := http.Get(authURL) //nolint:noctx // test helper
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}

	token, err := Login(context.Background(), cfg, opener, io.Discard)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token.AccessToken != "authelia_at_opaque_value" {
		t.Fatalf("unexpected access token: %q", token.AccessToken)
	}
	if token.RefreshToken != "authelia_rt_opaque_value" {
		t.Fatalf("unexpected refresh token: %q", token.RefreshToken)
	}
}

func TestCallbackHandler_Success(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := callbackHandler("the-state", codeCh, errCh)

	rec := httptest.NewRecorder()
	body := url.Values{"code": {"abc"}, "state": {"the-state"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	select {
	case code := <-codeCh:
		if code != "abc" {
			t.Fatalf("got code %q", code)
		}
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallbackHandler_StateMismatch(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := callbackHandler("expected-state", codeCh, errCh)

	rec := httptest.NewRecorder()
	body := url.Values{"code": {"abc"}, "state": {"attacker-state"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	select {
	case <-codeCh:
		t.Fatal("code accepted despite state mismatch")
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a state-mismatch error")
		}
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCallbackHandler_AuthorizationError(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := callbackHandler("s", codeCh, errCh)

	rec := httptest.NewRecorder()
	body := url.Values{"error": {"access_denied"}, "error_description": {"user said no"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	select {
	case <-codeCh:
		t.Fatal("code delivered despite authorization error")
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected an authorization error")
		}
	}
}

func TestBindLoopback_PicksFreePort(t *testing.T) {
	listener, port, err := bindLoopback(config.LoopbackPorts)
	if err != nil {
		t.Fatalf("bindLoopback: %v", err)
	}
	defer listener.Close()
	if port != config.LoopbackPorts[0] {
		t.Fatalf("expected first free port %d, got %d", config.LoopbackPorts[0], port)
	}
	if got := listener.Addr().String(); got != fmt.Sprintf("127.0.0.1:%d", port) {
		t.Fatalf("unexpected listener addr %q", got)
	}
}
