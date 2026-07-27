package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/datapointchris/meso/cli/internal/config"
)

// providerMetadata is the subset of the OIDC discovery document the CLI needs.
// Authelia's native bearer.authz flow requires a Pushed Authorization Request
// (RFC 9126), so the PAR endpoint is part of the contract.
type providerMetadata struct {
	Issuer                             string `json:"issuer"`
	AuthorizationEndpoint              string `json:"authorization_endpoint"`
	TokenEndpoint                      string `json:"token_endpoint"`
	PushedAuthorizationRequestEndpoint string `json:"pushed_authorization_request_endpoint"`
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return providerMetadata{}, fmt.Errorf("identity provider discovery returned %s", resp.Status)
	}
	var meta providerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return providerMetadata{}, err
	}
	return meta, nil
}

// oauthConfig builds the oauth2 config used for the token exchange and refresh.
// AuthStyle is pinned to InParams because the client is public
// (token_endpoint_auth_method: none) — the client_id goes in the request body,
// and there is no secret to put in a Basic header.
func oauthConfig(cfg config.Config, meta providerMetadata, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID: cfg.ClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:   meta.AuthorizationEndpoint,
			TokenURL:  meta.TokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: redirectURL,
		Scopes:      config.Scopes,
	}
}

// Login runs Authelia's native OAuth 2.0 Bearer Authorization: Authorization
// Code + PKCE (RFC 7636) + Pushed Authorization Request (RFC 9126) with a
// loopback redirect (RFC 8252) and response_mode=form_post. It binds the first
// free port from config.LoopbackPorts, pushes the request to Authelia's PAR
// endpoint, opens the browser to a short authorization URL, and blocks until
// Authelia POSTs the code back to the loopback callback or ctx is cancelled.
// Progress is written to progress (stderr), never stdout.
func Login(ctx context.Context, cfg config.Config, opener func(string) error, progress io.Writer) (*oauth2.Token, error) {
	meta, err := Discover(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	if meta.PushedAuthorizationRequestEndpoint == "" {
		return nil, fmt.Errorf("identity provider does not advertise a pushed_authorization_request_endpoint")
	}

	listener, port, err := bindLoopback(config.LoopbackPorts)
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	redirectURL := fmt.Sprintf("http://%s:%d/callback", config.RedirectHost, port)

	verifier := oauth2.GenerateVerifier()
	state, err := randomToken()
	if err != nil {
		return nil, err
	}

	// Push the authorization request; get back a one-time request_uri.
	requestURI, err := pushAuthorizationRequest(ctx, cfg, meta, redirectURL, state, verifier)
	if err != nil {
		return nil, err
	}
	authURL := fmt.Sprintf(
		"%s?client_id=%s&request_uri=%s",
		meta.AuthorizationEndpoint,
		url.QueryEscape(cfg.ClientID),
		url.QueryEscape(requestURI),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: callbackHandler(state, codeCh, errCh)}
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	fmt.Fprintf(progress, "Opening your browser to sign in as %s...\n", cfg.ClientID)
	fmt.Fprintf(progress, "Approve the consent screen. If the browser does not open, visit:\n\n  %s\n\n", authURL)
	if openErr := opener(authURL); openErr != nil {
		fmt.Fprintf(progress, "(could not open a browser automatically: %v)\n", openErr)
	}

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	oauthCfg := oauthConfig(cfg, meta, redirectURL)
	token, err := oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code for token: %w", err)
	}
	return token, nil
}

// pushAuthorizationRequest POSTs the full authorization request to the PAR
// endpoint and returns the resulting request_uri. The client is public, so the
// client_id travels in the body and there is no client authentication.
func pushAuthorizationRequest(ctx context.Context, cfg config.Config, meta providerMetadata, redirectURL, state, verifier string) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("response_type", "code")
	form.Set("response_mode", "form_post")
	form.Set("redirect_uri", redirectURL)
	form.Set("scope", strings.Join(config.Scopes, " "))
	form.Set("audience", cfg.Audience)
	form.Set("state", state)
	form.Set("code_challenge", oauth2.S256ChallengeFromVerifier(verifier))
	form.Set("code_challenge_method", "S256")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.PushedAuthorizationRequestEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("push authorization request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pushed authorization request rejected (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var par struct {
		RequestURI string `json:"request_uri"`
		ExpiresIn  int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &par); err != nil {
		return "", fmt.Errorf("decode PAR response: %w", err)
	}
	if par.RequestURI == "" {
		return "", fmt.Errorf("PAR response carried no request_uri")
	}
	return par.RequestURI, nil
}

// callbackHandler validates the state parameter and captures the authorization
// code. With response_mode=form_post, Authelia POSTs the parameters as a form
// body; FormValue transparently reads them (and query params as a fallback).
func callbackHandler(wantState string, codeCh chan<- string, errCh chan<- error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeResultPage(w, false, "could not parse the callback parameters")
			errCh <- fmt.Errorf("parse callback form: %w", err)
			return
		}
		if oauthErr := r.FormValue("error"); oauthErr != "" {
			detail := oauthErr + ": " + r.FormValue("error_description")
			writeResultPage(w, false, detail)
			errCh <- fmt.Errorf("authorization failed: %s", detail)
			return
		}
		if r.FormValue("state") != wantState {
			writeResultPage(w, false, "state mismatch — possible CSRF, login aborted")
			errCh <- fmt.Errorf("state mismatch: the callback state did not match the request")
			return
		}
		code := r.FormValue("code")
		if code == "" {
			writeResultPage(w, false, "no authorization code in callback")
			errCh <- fmt.Errorf("callback carried no authorization code")
			return
		}
		writeResultPage(w, true, "")
		codeCh <- code
	})
	return mux
}

func writeResultPage(w http.ResponseWriter, ok bool, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	heading := "You're signed in to meso"
	body := "You can close this tab and return to the terminal."
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		heading = "Sign-in failed"
		body = detail
	}
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>meso</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:6rem auto;padding:0 1rem;color:#1a1a1a}
h1{font-size:1.4rem}p{color:#555}</style></head>
<body><h1>%s</h1><p>%s</p></body></html>`, heading, body)
}

// bindLoopback binds the first available port from the candidate set on
// 127.0.0.1. Returns the listener and the port that was bound.
func bindLoopback(ports []int) (net.Listener, int, error) {
	var lastErr error
	for _, port := range ports {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return listener, port, nil
		}
		lastErr = err
	}
	return nil, 0, fmt.Errorf("no loopback port free in %v: %w", ports, lastErr)
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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
	oauthCfg := oauthConfig(cfg, meta, "")
	base := oauthCfg.TokenSource(ctx, tok)
	return &persistingTokenSource{
		base:        base,
		store:       store,
		clientID:    cfg.ClientID,
		lastPersist: tok.AccessToken,
	}, nil
}
