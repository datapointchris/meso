package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/datapointchris/goclilogin"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/config"
)

// loginTimeout is a backstop only — the device code carries its own expiry and
// the poll stops there first.
const loginTimeout = 15 * time.Minute

func newAuthCommand() *cobra.Command {
	cmd := asNamespace(&cobra.Command{
		Use:   "auth",
		Short: "Log in and out of the meso API",
		Long: "Authenticate this machine against Authelia using the OAuth 2.0 device\n" +
			"authorization grant. The CLI prints a code and a URL; approve it in any\n" +
			"browser on any device, including from a different machine over SSH.\n" +
			"\n" +
			"The token is kept in the OS keychain, or in a file only this user can read\n" +
			"on a host with no keychain. `meso auth status` says which.\n" +
			"\n" +
			"Where the CLI is pointed comes from one config file. `meso auth status`\n" +
			"prints its path and its api, issuer and client_id, and `meso auth login\n" +
			"<url>` fills in whatever it lacks.",
	})
	cmd.AddCommand(newAuthLoginCommand(), newAuthLogoutCommand(), newAuthStatusCommand(), newAuthTokenCommand())
	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	var clientID string
	cmd := &cobra.Command{
		Use:   "login [url]",
		Short: "Log in by approving a code in any browser",
		Long: "Log in to the meso API at url, or to the config file's api when url is\n" +
			"omitted. Whatever the config file lacks is filled in: api from url, the\n" +
			"issuer from the API's RFC 9728 metadata, and the client id from\n" +
			"--client-id or this machine's short hostname. Each is printed with where it\n" +
			"came from before the login starts, and written to the file once it succeeds.\n" +
			"A value already in the file is never rewritten.",
		Example: "  meso auth login\n  meso auth login https://meso.example.com\n" +
			"  meso auth login https://meso.example.com --client-id meso-cli-laptop",
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := apiOverride
			if len(args) == 1 {
				if apiOverride != "" && apiOverride != args[0] {
					return usageError{fmt.Errorf("--api %s and the url %s name different APIs; give one", apiOverride, args[0])}
				}
				url = args[0]
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
			defer cancel()

			deployment, err := config.ForLogin(ctx, url, clientID)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Logging in to:")
			goclilogin.WriteDeployment(cmd.ErrOrStderr(), deployment)
			_, _ = fmt.Fprintln(cmd.ErrOrStderr())

			login := config.Login(deployment)
			store := goclilogin.NewTokenStore(login)

			// pkg/browser hands the launcher subprocess os.Stdout and os.Stderr,
			// which puts whatever the browser writes at startup — GPU warnings,
			// favicon decode failures, a dbus complaint — in the middle of the
			// user code and the verification URL. A launcher that fails is
			// reported by OpenURL's error instead, so nothing diagnostic is lost
			// by dropping the stream. Package-level vars are the library's only
			// knob for it.
			browser.Stdout = io.Discard
			browser.Stderr = io.Discard

			token, err := goclilogin.Login(ctx, login, func(p goclilogin.DevicePrompt) {
				goclilogin.WriteInstructions(cmd.ErrOrStderr(), login.ClientID, p)
				if openErr := browser.OpenURL(p.BrowserURL()); openErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "(could not open a browser here: %v)\n", openErr)
				}
			})
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("timed out waiting for approval after %s; run `meso auth login` again", loginTimeout)
				}
				return err
			}
			backend, err := store.Save(login.ClientID, token)
			if err != nil {
				return fmt.Errorf("save token: %w", err)
			}
			if backend == goclilogin.BackendFile {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
					"\nNo OS keyring on this host — the token is in %s, readable by this user only.\n",
					store.FilePath())
			}
			if err := deployment.Save(); err != nil {
				return fmt.Errorf("logged in as %s, but the config file was not written: %w", login.ClientID, err)
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\n✓ Logged in (%s)\n", login.ClientID)
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "",
		"Log in as this OAuth client when the config file names none (default meso-cli-<short hostname>)")
	return cmd
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Remove the stored token from this machine's keychain",
		Example: "  meso auth logout",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Only the client id is needed to find the token, so a file that
			// lacks the other two keys still logs out. A file naming no client
			// falls back to the hostname's, which every release before the
			// config file logged in as, so its token cannot outlive the logout.
			deployment, err := config.Load(cmd.Context(), apiOverride)
			if err != nil && !errors.Is(err, goclilogin.ErrNotConfigured) {
				return err
			}
			if deployment.ClientID.Value == "" {
				deployment.ClientID.Value = config.DefaultClientID()
			}
			clientID := deployment.ClientID.Value
			store := goclilogin.NewTokenStore(config.Login(deployment))

			if err := store.Delete(clientID); err != nil {
				if errors.Is(err, goclilogin.ErrNotLoggedIn) {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Not logged in — nothing to remove.")
					return nil
				}
				return fmt.Errorf("remove token from keychain: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Logged out (%s).\n", clientID)
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
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			deployment, err := config.Load(cmd.Context(), apiOverride)
			if err != nil {
				return err
			}
			login := config.Login(deployment)
			store := goclilogin.NewTokenStore(login)

			source, err := goclilogin.TokenSource(cmd.Context(), login, store)
			if errors.Is(err, goclilogin.ErrNotLoggedIn) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Not logged in as %s. Run `meso auth login`.\n", login.ClientID)
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

// statusReport is the stable JSON schema for `auth status --json`. There is no
// audience field because Authelia leaves `aud` empty through the device grant —
// reporting one would be a claim the token does not carry.
type statusReport struct {
	LoggedIn bool `json:"logged_in"`
	// Configured is false when the config file lacks a value, which ConfigFile
	// names and the empty fields below show.
	Configured bool   `json:"configured"`
	ConfigFile string `json:"config_file"`
	API        string `json:"api"`
	ClientID   string `json:"client_id"`
	Issuer     string `json:"issuer"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	Expired    bool   `json:"expired"`

	// Backend is where the token is kept. A host with no OS keyring stores it
	// in a mode-600 file instead, and that is worth saying out loud.
	Backend goclilogin.Backend `json:"backend,omitempty"`
}

func newAuthStatusCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show where the CLI is pointed and whether it is logged in",
		Long: "Show the api, issuer and client_id this machine uses, where each came\n" +
			"from, the config file's path, and whether a token is stored for the client.\n" +
			"Exits 1 when the file lacks a value or no token is stored.",
		Example: "  meso auth status\n  meso auth status --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			deployment, loadErr := config.Load(cmd.Context(), apiOverride)
			if loadErr != nil && !errors.Is(loadErr, goclilogin.ErrNotConfigured) {
				return loadErr
			}
			report := statusReport{
				Configured: loadErr == nil,
				ConfigFile: deployment.Path,
				API:        deployment.API.Value,
				ClientID:   deployment.ClientID.Value,
				Issuer:     deployment.Issuer.Value,
			}

			if report.ClientID != "" {
				store := goclilogin.NewTokenStore(config.Login(deployment))
				token, backend, err := store.Load(report.ClientID)
				switch {
				case errors.Is(err, goclilogin.ErrNotLoggedIn):
					// logged out is a valid state, reported on stdout, exit 1 (like gh)
				case err != nil:
					return fmt.Errorf("read token from keychain: %w", err)
				default:
					report.LoggedIn = true
					if !token.Expiry.IsZero() {
						report.ExpiresAt = token.Expiry.Format(time.RFC3339)
						report.Expired = time.Now().After(token.Expiry)
						report.Backend = backend
					}
				}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printStatus(cmd.OutOrStdout(), report, deployment, loadErr)
			}

			if !report.Configured || !report.LoggedIn {
				return exitCode(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output status as JSON to stdout")
	return cmd
}

// printStatus leads with the verdict on a line of its own, then prints where
// the CLI is pointed. A script checking a login reads the first line alone.
func printStatus(out io.Writer, r statusReport, d goclilogin.Deployment, loadErr error) {
	switch {
	case !r.Configured:
		_, _ = fmt.Fprintf(out, "%v\n", loadErr)
	case !r.LoggedIn:
		_, _ = fmt.Fprintf(out, "Not logged in as %s. Run `meso auth login` to authenticate.\n", r.ClientID)
	default:
		_, _ = fmt.Fprintln(out, "Logged in")
	}
	goclilogin.WriteDeployment(out, d)
	// WriteDeployment's widest key is client_id, padded by two, so its values
	// start in the column these rows pad to.
	if r.ExpiresAt != "" {
		state := "valid"
		if r.Expired {
			state = "expired — will refresh on next use"
		}
		_, _ = fmt.Fprintf(out, "  %-11s%s (expires %s)\n", "token", state, r.ExpiresAt)
	}
	if r.Backend == goclilogin.BackendFile {
		_, _ = fmt.Fprintf(out, "  %-11s%s — no OS keyring on this host\n", "storage", r.Backend)
	}
}
