package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datapointchris/goclilogin"
)

// Login is the seam between meso's settings and goclilogin's, so what it
// carries across is worth pinning. The keyring service is a deployed identifier
// that released versions already wrote under, and the lock belongs beside the
// tool's other state rather than under goclilogin's default.
//
// The scope set is goclilogin's DefaultScopes plus groups, which the API reads to
// admit the admin group. It must not carry authelia.bearer.authz, which Authelia
// forbids alongside the device grant.
func TestLoginCarriesTheDeployedIdentifiers(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdgstate")

	login := Login(goclilogin.Deployment{
		Issuer:   goclilogin.Setting{Value: "https://auth.example.com"},
		ClientID: goclilogin.Setting{Value: "meso-cli-somehost"},
	})
	if login.Issuer != "https://auth.example.com" {
		t.Errorf("Issuer = %q", login.Issuer)
	}
	if login.ClientID != "meso-cli-somehost" {
		t.Errorf("ClientID = %q", login.ClientID)
	}
	if login.KeyringService != "meso-cli" {
		t.Errorf("KeyringService = %q, want meso-cli", login.KeyringService)
	}
	if login.StateDir != "/tmp/xdgstate/meso" {
		t.Errorf("StateDir = %q, want the tool's own state directory", login.StateDir)
	}
	if got := strings.Join(login.Scopes, " "); got != "openid profile offline_access groups" {
		t.Errorf("Scopes = %q", got)
	}
}

// The file is the only source. A variable an earlier release read is ignored,
// so a stale export cannot point the CLI somewhere the file does not say.
func TestLoadReadsMesosOwnFileAndNoVariable(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("MESO_API_BASE", "https://stale.example.com")
	t.Setenv("MESO_OIDC_ISSUER", "https://stale-auth.example.com")
	t.Setenv("MESO_CLIENT_ID", "meso-cli-stale")

	path := filepath.Join(configHome, "meso", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "api = \"https://meso.example.com\"\n" +
		"issuer = \"https://auth.example.com\"\n" +
		"client_id = \"meso-cli-workstation\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := Load(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Path != path || d.API.Value != "https://meso.example.com" ||
		d.Issuer.Value != "https://auth.example.com" || d.ClientID.Value != "meso-cli-workstation" {
		t.Errorf("Load() = %+v", d)
	}
}

// With no file there is nothing to fall back to, and the error names the
// command that writes one.
func TestLoadWithNoFileNamesTheLogin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := Load(context.Background(), "")
	if !errors.Is(err, goclilogin.ErrNotConfigured) || !strings.Contains(err.Error(), "meso auth login <url>") {
		t.Errorf("Load() error = %v, want ErrNotConfigured naming `meso auth login <url>`", err)
	}
}
