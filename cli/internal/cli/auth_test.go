package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datapointchris/goclilogin"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"

	"github.com/datapointchris/meso/cli/internal/config"
)

// isolateAuth points the config file, the state directory and the keychain at
// test-owned places, so no test reads or writes this machine's login.
func isolateAuth(t *testing.T) (configHome string) {
	t.Helper()
	keyring.MockInit()
	configHome = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return configHome
}

func runMeso(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), err
}

// A machine with no config file gets the file's path and the command that
// writes it, and exits 1 without consulting the keychain.
func TestAuthStatus_WithNoConfigFileNamesItAndTheLogin(t *testing.T) {
	configHome := isolateAuth(t)

	stdout, err := runMeso(t, "auth", "status")
	var ec exitCode
	if !errors.As(err, &ec) || ec != 1 {
		t.Fatalf("want exitCode(1), got %T: %v", err, err)
	}
	for _, want := range []string{filepath.Join(configHome, "meso", "config.toml"), "meso auth login <url>", "(not set)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status missing %q:\n%s", want, stdout)
		}
	}
}

// Where every command is pointed is meso's own file under XDG_CONFIG_HOME.
func TestAuthStatus_PrintsWhatTheConfigFileSays(t *testing.T) {
	configHome := isolateAuth(t)
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

	stdout, _ := runMeso(t, "auth", "status")
	for _, want := range []string{"https://meso.example.com", "https://auth.example.com", "meso-cli-workstation", path} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status missing %q:\n%s", want, stdout)
		}
	}
}

// A machine logged in by a release from before the config file holds its token
// under the hostname's client id, with no file naming one. Logging out there
// removes that token rather than reporting there is nothing to remove.
func TestAuthLogout_RemovesATokenStoredBeforeTheConfigFileExisted(t *testing.T) {
	isolateAuth(t)
	login := config.Login(goclilogin.Deployment{ClientID: goclilogin.Setting{Value: config.DefaultClientID()}})
	store := goclilogin.NewTokenStore(login)
	token := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}
	if _, err := store.Save(login.ClientID, token); err != nil {
		t.Fatal(err)
	}

	if _, err := runMeso(t, "auth", "logout"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load(login.ClientID); !errors.Is(err, goclilogin.ErrNotLoggedIn) {
		t.Errorf("after logout, Load = %v, want ErrNotLoggedIn", err)
	}
}

// The url and --api both name the API to log in to, so two different ones are
// a usage mistake rather than a choice the CLI makes for the caller.
func TestAuthLogin_RefusesAURLDisagreeingWithAPI(t *testing.T) {
	isolateAuth(t)

	_, err := runMeso(t, "auth", "login", "--api", "https://one.example.com", "https://two.example.com")
	var usage usageError
	if !errors.As(err, &usage) {
		t.Fatalf("want a usageError, got %T: %v", err, err)
	}
}
