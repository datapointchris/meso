package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/oauth2"

	"github.com/datapointchris/meso/cli/internal/config"
)

// mockIDP is a minimal Authelia stand-in for the device authorization grant:
// discovery, a device-authorization endpoint, and a token endpoint that reports
// authorization_pending until the request is "approved".
type mockIDP struct {
	server *httptest.Server

	mu             sync.Mutex
	pollsBefore    int
	polls          int
	omitDeviceAuth bool
}

func newMockIDP(t *testing.T, pollsBefore int) *mockIDP {
	t.Helper()
	idp := &mockIDP{pollsBefore: pollsBefore}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("/device", idp.deviceAuth)
	mux.HandleFunc("/token", idp.token)
	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

func (m *mockIDP) discovery(w http.ResponseWriter, _ *http.Request) {
	base := m.server.URL
	doc := map[string]string{
		"issuer":         base,
		"token_endpoint": base + "/token",
	}
	if !m.omitDeviceAuth {
		doc["device_authorization_endpoint"] = base + "/device"
	}
	_ = json.NewEncoder(w).Encode(doc)
}

func (m *mockIDP) deviceAuth(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.FormValue("client_id") == "" {
		http.Error(w, "missing client_id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"device_code":               "device-code-123",
		"user_code":                 "JSMTZRTB",
		"verification_uri":          "https://auth.example.com/device",
		"verification_uri_complete": "https://auth.example.com/device?user_code=JSMTZRTB",
		"expires_in":                600,
		"interval":                  1,
	})
}

func (m *mockIDP) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if got := r.FormValue("grant_type"); got != "urn:ietf:params:oauth:grant-type:device_code" {
		http.Error(w, "unexpected grant_type "+got, http.StatusBadRequest)
		return
	}
	if r.FormValue("device_code") != "device-code-123" {
		http.Error(w, "unknown device_code", http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	m.polls++
	pending := m.polls <= m.pollsBefore
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if pending {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "authelia_at_jwt_value",
		"refresh_token": "authelia_rt_value",
		"token_type":    "bearer",
		"expires_in":    3600,
	})
}

func TestLogin_DeviceFlowPollsUntilApproved(t *testing.T) {
	idp := newMockIDP(t, 1)
	cfg := config.Config{Issuer: idp.server.URL, ClientID: "meso-cli-macmini"}

	var opened string
	opener := func(url string) error {
		opened = url
		return nil
	}

	token, err := Login(context.Background(), cfg, opener, io.Discard)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token.AccessToken != "authelia_at_jwt_value" {
		t.Fatalf("unexpected access token: %q", token.AccessToken)
	}
	if token.RefreshToken != "authelia_rt_value" {
		t.Fatalf("unexpected refresh token: %q", token.RefreshToken)
	}
	if opened != "https://auth.example.com/device?user_code=JSMTZRTB" {
		t.Fatalf("opener got %q, expected the complete verification URI", opened)
	}
	idp.mu.Lock()
	defer idp.mu.Unlock()
	if idp.polls < 2 {
		t.Fatalf("expected the CLI to poll past authorization_pending, got %d polls", idp.polls)
	}
}

// The user code has to reach the user even when no browser can be opened, which
// is the whole point of the grant over SSH.
func TestLogin_WritesUserCodeWhenBrowserFails(t *testing.T) {
	idp := newMockIDP(t, 0)
	cfg := config.Config{Issuer: idp.server.URL, ClientID: "meso-cli-macmini"}

	var progress strings.Builder
	opener := func(string) error { return errors.New("no browser here") }

	if _, err := Login(context.Background(), cfg, opener, &progress); err != nil {
		t.Fatalf("Login: %v", err)
	}
	out := progress.String()
	if !strings.Contains(out, "JSMTZRTB") {
		t.Fatalf("user code missing from progress output: %q", out)
	}
	if !strings.Contains(out, "https://auth.example.com/device") {
		t.Fatalf("verification URI missing from progress output: %q", out)
	}
}

func TestLogin_ErrorsWhenProviderHasNoDeviceEndpoint(t *testing.T) {
	idp := newMockIDP(t, 0)
	idp.omitDeviceAuth = true
	cfg := config.Config{Issuer: idp.server.URL, ClientID: "meso-cli-macmini"}

	_, err := Login(context.Background(), cfg, nil, io.Discard)
	if err == nil {
		t.Fatal("expected an error when the provider advertises no device endpoint")
	}
	if !strings.Contains(err.Error(), "device_authorization_endpoint") {
		t.Fatalf("error should name the missing endpoint, got: %v", err)
	}
}

func TestVerificationURL_PrefersCompleteURI(t *testing.T) {
	withComplete := &oauth2.DeviceAuthResponse{
		VerificationURI:         "https://auth.example.com/device",
		VerificationURIComplete: "https://auth.example.com/device?user_code=ABCD",
	}
	if got := verificationURL(withComplete); got != withComplete.VerificationURIComplete {
		t.Fatalf("got %q, want the complete URI", got)
	}

	bare := &oauth2.DeviceAuthResponse{VerificationURI: "https://auth.example.com/device"}
	if got := verificationURL(bare); got != bare.VerificationURI {
		t.Fatalf("got %q, want the bare URI", got)
	}
}
