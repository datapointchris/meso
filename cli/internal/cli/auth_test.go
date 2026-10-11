package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A machine with no config file gets the file's path and the command that
// writes it, and exits 1 without consulting the keychain.
func TestAuthStatus_WithNoConfigFileNamesItAndTheLogin(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	var stdout bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"auth", "status"})

	err := root.Execute()
	var ec exitCode
	if !errors.As(err, &ec) || ec != 1 {
		t.Fatalf("want exitCode(1), got %T: %v", err, err)
	}
	for _, want := range []string{filepath.Join(configHome, "meso", "config.toml"), "meso auth login <url>", "(not set)"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("status missing %q:\n%s", want, stdout.String())
		}
	}
}

// The url and --api both name the API to log in to, so two different ones are
// a usage mistake rather than a choice the CLI makes for the caller.
func TestAuthLogin_RefusesAURLDisagreeingWithAPI(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--api", "https://one.example.com", "auth", "login", "https://two.example.com"})

	var usage usageError
	if err := root.Execute(); !errors.As(err, &usage) {
		t.Fatalf("want a usageError, got %T: %v", err, err)
	}
}
