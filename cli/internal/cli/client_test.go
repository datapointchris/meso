package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datapointchris/goclilogin"
	"golang.org/x/oauth2"

	"github.com/datapointchris/meso/cli/internal/api"
	"github.com/datapointchris/meso/cli/internal/config"
)

// A CLI started under a trace sends it on each API request unchanged, so the
// API's span for the request is the child of whatever ran the CLI.
func TestAnAPIRequestCarriesTheTraceTheCLIWasStartedUnder(t *testing.T) {
	configHome := isolateAuth(t)
	var sent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get("traceparent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	path := filepath.Join(configHome, "meso", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "api = \"" + server.URL + "\"\nissuer = \"https://auth.example.com\"\nclient_id = \"meso-cli-test\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	login := config.Login(goclilogin.Deployment{
		Issuer:   goclilogin.Setting{Value: "https://auth.example.com"},
		ClientID: goclilogin.Setting{Value: "meso-cli-test"},
	})
	token := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}
	if _, err := goclilogin.NewTokenStore(login).Save(login.ClientID, token); err != nil {
		t.Fatal(err)
	}

	const traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	t.Setenv("TRACEPARENT", traceparent)
	if err := runLine(t, "stats", "--json"); err != nil {
		t.Fatal(err)
	}
	if sent != traceparent {
		t.Errorf("the API was sent traceparent %q, want %q", sent, traceparent)
	}
}

// An admin command refused by the group gate prints the API's own reason, which
// the CLI dropped while the gate answered in plain text.
func TestHandleAPIErrorPrintsTheAPIsReasonForA403(t *testing.T) {
	t.Parallel()

	refusal := &api.APIError{
		StatusCode: http.StatusForbidden, Status: "403 Forbidden",
		Message: "this needs membership in the admin group",
	}
	got := handleAPIError(refusal).Error()
	for _, want := range []string{"this needs membership in the admin group", "meso auth login"} {
		if !strings.Contains(got, want) {
			t.Errorf("handleAPIError(403) = %q, missing %q", got, want)
		}
	}
}
