package cli

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"

	"meso/cli/internal/api"
	"meso/cli/internal/auth"
	"meso/cli/internal/config"
)

// errNeedsLogin is returned by newAPIClient when there is no stored token. The
// resource commands print the login hint and exit 1; it is a normal state, not a
// crash, so it is kept distinct from a runtime error.
var errNeedsLogin = errors.New("not logged in")

// newAPIClient builds an authenticated API client for the logged-in machine. The
// oauth2 client injects (and refreshes) the bearer token on every request via the
// persisting token source, so resource commands never touch tokens directly.
func newAPIClient(ctx context.Context) (*api.Client, error) {
	cfg := config.Load()
	store := auth.NewTokenStore()

	source, err := auth.TokenSource(ctx, cfg, store)
	if errors.Is(err, auth.ErrNotLoggedIn) {
		return nil, errNeedsLogin
	}
	if err != nil {
		return nil, fmt.Errorf("prepare API client: %w", err)
	}

	httpClient := oauth2.NewClient(ctx, source)
	return api.New(cfg.APIBase, httpClient), nil
}

// handleAPIError maps an error from a resource command to a message and exit
// code: not-logged-in and 401 both point at `meso auth login`; everything else is
// a runtime error. Returns the error to return from RunE.
func handleAPIError(err error) error {
	if errors.Is(err, errNeedsLogin) {
		return fmt.Errorf("not logged in — run `meso auth login`")
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.Unauthorized() {
		return fmt.Errorf("session rejected by the API — run `meso auth login` to re-authenticate")
	}
	return err
}
