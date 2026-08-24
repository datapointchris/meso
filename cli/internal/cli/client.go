package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/datapointchris/goclilogin"
	"golang.org/x/oauth2"

	"github.com/datapointchris/meso/cli/internal/api"
	"github.com/datapointchris/meso/cli/internal/config"
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
	store := goclilogin.NewTokenStore(cfg.Login().KeyringService)

	source, err := goclilogin.TokenSource(ctx, cfg.Login(), store)
	if errors.Is(err, goclilogin.ErrNotLoggedIn) {
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
	// A refusal at the token endpoint arrives as the transport error of the
	// request that triggered the refresh, so the URL and the raw OAuth error
	// description are what reach the terminal unless they are named here.
	if goclilogin.IsSessionRejected(err) {
		return fmt.Errorf("session expired — run `meso auth login`")
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.Unauthorized() {
		return fmt.Errorf("session rejected by the API — run `meso auth login` to re-authenticate")
	}
	return err
}
