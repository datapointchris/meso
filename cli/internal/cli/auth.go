package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/auth"
	"github.com/datapointchris/meso/cli/internal/config"
)

// loginTimeout bounds how long the CLI waits for the browser round-trip before
// giving up and freeing the loopback port.
const loginTimeout = 5 * time.Minute

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in and out of the meso API",
		Long: "Authenticate this machine against Authelia using the OAuth 2.0\n" +
			"Authorization Code flow with PKCE, a pushed authorization request, and a\n" +
			"loopback redirect. The resulting bearer token is authorized at the edge\n" +
			"and stored in the OS keychain, never on disk.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(newAuthLoginCommand(), newAuthLogoutCommand(), newAuthStatusCommand(), newAuthTokenCommand())
	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "login",
		Short:   "Log in via the browser",
		Example: "  meso auth login",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load()
			store := auth.NewTokenStore()

			ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
			defer cancel()

			token, err := auth.Login(ctx, cfg, browser.OpenURL, cmd.ErrOrStderr())
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("timed out waiting for browser sign-in after %s; run `meso auth login` again", loginTimeout)
				}
				return err
			}
			if err := store.Save(cfg.ClientID, token); err != nil {
				return fmt.Errorf("save token to keychain: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\n✓ Logged in (%s)\n", cfg.ClientID)
			return nil
		},
	}
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Remove the stored token from this machine's keychain",
		Example: "  meso auth logout",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load()
			store := auth.NewTokenStore()

			if err := store.Delete(cfg.ClientID); err != nil {
				if errors.Is(err, auth.ErrNotLoggedIn) {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Not logged in — nothing to remove.")
					return nil
				}
				return fmt.Errorf("remove token from keychain: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Logged out (%s).\n", cfg.ClientID)
			return nil
		},
	}
}

func newAuthTokenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the current access token to stdout",
		Long: "Print a valid access token to stdout, refreshing it first if it has\n" +
			"expired. Intended for scripting, e.g. `curl -H \"Authorization: Bearer\n" +
			"$(meso auth token)\" …`. Exits non-zero if not logged in.",
		Example: "  meso auth token",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load()
			store := auth.NewTokenStore()

			source, err := auth.TokenSource(cmd.Context(), cfg, store)
			if errors.Is(err, auth.ErrNotLoggedIn) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Not logged in as %s. Run `meso auth login`.\n", cfg.ClientID)
				return exitCode(1)
			}
			if err != nil {
				return err
			}
			token, err := source.Token()
			if err != nil {
				return fmt.Errorf("obtain a valid token (refresh may have failed — try `meso auth login`): %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), token.AccessToken)
			return nil
		},
	}
}

// statusReport is the stable JSON schema for `auth status --json`. The token is
// opaque (edge-authorized), so there are no identity claims to surface — only
// whether a token is stored and whether it has expired.
type statusReport struct {
	LoggedIn  bool   `json:"logged_in"`
	ClientID  string `json:"client_id"`
	Issuer    string `json:"issuer"`
	Audience  string `json:"audience"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Expired   bool   `json:"expired"`
}

func newAuthStatusCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show whether the CLI is logged in",
		Example: "  meso auth status\n  meso auth status --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load()
			store := auth.NewTokenStore()

			report := statusReport{ClientID: cfg.ClientID, Issuer: cfg.Issuer, Audience: cfg.Audience}

			token, err := store.Load(cfg.ClientID)
			switch {
			case errors.Is(err, auth.ErrNotLoggedIn):
				// logged out is a valid state, reported on stdout, exit 1 (like gh)
			case err != nil:
				return fmt.Errorf("read token from keychain: %w", err)
			default:
				report.LoggedIn = true
				if !token.Expiry.IsZero() {
					report.ExpiresAt = token.Expiry.Format(time.RFC3339)
					report.Expired = time.Now().After(token.Expiry)
				}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printStatus(cmd, report)
			}

			if !report.LoggedIn {
				return exitCode(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output status as JSON to stdout")
	return cmd
}

func printStatus(cmd *cobra.Command, r statusReport) {
	out := cmd.OutOrStdout()
	if !r.LoggedIn {
		_, _ = fmt.Fprintf(out, "Not logged in as %s.\nRun `meso auth login` to authenticate.\n", r.ClientID)
		return
	}
	_, _ = fmt.Fprintf(out, "Logged in\n")
	_, _ = fmt.Fprintf(out, "  client:   %s\n", r.ClientID)
	_, _ = fmt.Fprintf(out, "  issuer:   %s\n", r.Issuer)
	_, _ = fmt.Fprintf(out, "  audience: %s\n", r.Audience)
	if r.ExpiresAt != "" {
		state := "valid"
		if r.Expired {
			state = "expired — will refresh on next use"
		}
		_, _ = fmt.Fprintf(out, "  token:    %s (expires %s)\n", state, r.ExpiresAt)
	}
}
