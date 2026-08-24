package config

import (
	"strings"
	"testing"
)

// Login is the seam between meso's settings and goclilogin's, so what it
// carries across is worth pinning. The keyring service is a deployed identifier
// that released versions already wrote under, and the lock belongs beside the
// tool's other state rather than under goclilogin's default.
//
// The scope set is goclilogin's DefaultScopes and is tested there. It must not
// carry authelia.bearer.authz, which Authelia forbids alongside the device
// grant.
func TestLoginCarriesTheDeployedIdentifiers(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdgstate")
	t.Setenv("MESO_OIDC_ISSUER", "https://auth.example.com")
	t.Setenv("MESO_CLIENT_ID", "meso-cli-somehost")

	login := Load().Login()
	if login.Issuer != "https://auth.example.com" {
		t.Errorf("Issuer = %q", login.Issuer)
	}
	if login.ClientID != "meso-cli-somehost" {
		t.Errorf("ClientID = %q", login.ClientID)
	}
	if login.KeyringService != "meso-cli" {
		t.Errorf("KeyringService = %q, want meso-cli", login.KeyringService)
	}
	if login.LockDir != "/tmp/xdgstate/meso" {
		t.Errorf("LockDir = %q, want the tool's own state directory", login.LockDir)
	}
}

// An env var set to the empty string falls through to the default rather than
// blanking the field — getEnv treats "" as unset.
func TestLoad_EmptyEnvFallsThroughToDefault(t *testing.T) {
	t.Setenv("MESO_API_BASE", "")
	if got := Load().APIBase; got != defaultAPIBase {
		t.Errorf("APIBase = %q, want the default %q", got, defaultAPIBase)
	}
}

// The client id spells the machine into it, so one machine's token is revocable
// without touching another's.
func TestDefaultClientIDIsPerMachine(t *testing.T) {
	t.Setenv("MESO_CLIENT_ID", "")
	got := Load().ClientID
	if !strings.HasPrefix(got, "meso-cli-") {
		t.Errorf("ClientID = %q, want a meso-cli-<host> spelling", got)
	}
}
