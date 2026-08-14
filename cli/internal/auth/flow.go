package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/datapointchris/meso/cli/internal/config"
)

// providerMetadata is the subset of the OIDC discovery document the CLI needs.
type providerMetadata struct {
	Issuer                      string `json:"issuer"`
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
}

// Discover fetches the OIDC discovery document for the issuer.
func Discover(ctx context.Context, issuer string) (providerMetadata, error) {
	discoURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoURL, nil)
	if err != nil {
		return providerMetadata{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return providerMetadata{}, fmt.Errorf("reach identity provider at %s: %w", discoURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return providerMetadata{}, fmt.Errorf("identity provider discovery returned %s", resp.Status)
	}
	var meta providerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return providerMetadata{}, err
	}
	return meta, nil
}

// oauthConfig builds the oauth2 config used for the device grant and refresh.
// AuthStyle is pinned to InParams because the client is public
// (token_endpoint_auth_method: none) — the client_id goes in the request body,
// and there is no secret to put in a Basic header.
func oauthConfig(cfg config.Config, meta providerMetadata) *oauth2.Config {
	return &oauth2.Config{
		ClientID: cfg.ClientID,
		Endpoint: oauth2.Endpoint{
			TokenURL:      meta.TokenEndpoint,
			DeviceAuthURL: meta.DeviceAuthorizationEndpoint,
			AuthStyle:     oauth2.AuthStyleInParams,
		},
		Scopes: config.Scopes,
	}
}

// Login runs the OAuth 2.0 device authorization grant (RFC 8628). It asks
// Authelia for a device code, prints the user code and verification URL, and
// polls until the request is approved or the code expires.
//
// Approval happens in any browser on any device, which is the point: a loopback
// redirect requires a browser that can reach a listener on this machine, and
// over SSH there is none. Progress is written to progress (stderr), never
// stdout, so `auth token` stays pipeable.
func Login(ctx context.Context, cfg config.Config, opener func(string) error, progress io.Writer) (*oauth2.Token, error) {
	meta, err := Discover(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	if meta.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("identity provider does not advertise a device_authorization_endpoint")
	}

	oauthCfg := oauthConfig(cfg, meta)
	deviceAuth, err := oauthCfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("request a device code: %w", err)
	}

	writeInstructions(progress, cfg.ClientID, deviceAuth)
	if url := verificationURL(deviceAuth); opener != nil {
		if openErr := opener(url); openErr != nil {
			_, _ = fmt.Fprintf(progress, "(could not open a browser here: %v)\n", openErr)
		}
	}

	token, err := oauthCfg.DeviceAccessToken(ctx, deviceAuth)
	if err != nil {
		return nil, fmt.Errorf("waiting for approval: %w", err)
	}
	return token, nil
}

// verificationURL prefers the URI with the code already embedded, so a browser
// that does open lands on an approval screen with nothing to type.
func verificationURL(d *oauth2.DeviceAuthResponse) string {
	if d.VerificationURIComplete != "" {
		return d.VerificationURIComplete
	}
	return d.VerificationURI
}

func writeInstructions(progress io.Writer, clientID string, d *oauth2.DeviceAuthResponse) {
	_, _ = fmt.Fprintf(progress, "\nSigning in as %s\n\n", clientID)
	_, _ = fmt.Fprintf(progress, "  Code:  %s\n", d.UserCode)
	_, _ = fmt.Fprintf(progress, "  Open:  %s\n\n", d.VerificationURI)
	_, _ = fmt.Fprintln(progress, "Open that URL in any browser, on any device, and enter the code.")
	_, _ = fmt.Fprintln(progress, "Waiting for approval...")
}

// persistingTokenSource wraps a refreshing oauth2.TokenSource and writes any
// newly-refreshed token back to the keychain, so a refresh performed on one
// invocation persists for the next.
type persistingTokenSource struct {
	base        oauth2.TokenSource
	store       *TokenStore
	clientID    string
	lastPersist string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != p.lastPersist {
		if saveErr := p.store.Save(p.clientID, tok); saveErr != nil {
			return tok, fmt.Errorf("persist refreshed token: %w", saveErr)
		}
		p.lastPersist = tok.AccessToken
	}
	return tok, nil
}

// TokenSource returns an auto-refreshing, keychain-persisting token source for
// the logged-in client, or ErrNotLoggedIn if there is no stored token.
func TokenSource(ctx context.Context, cfg config.Config, store *TokenStore) (oauth2.TokenSource, error) {
	tok, err := store.Load(cfg.ClientID)
	if err != nil {
		return nil, err
	}
	meta, err := Discover(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	oauthCfg := oauthConfig(cfg, meta)
	base := oauthCfg.TokenSource(ctx, tok)
	return &persistingTokenSource{
		base:        base,
		store:       store,
		clientID:    cfg.ClientID,
		lastPersist: tok.AccessToken,
	}, nil
}
