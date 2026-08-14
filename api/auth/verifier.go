// Package auth verifies the RFC 9068 JWT access tokens Authelia issues to the
// meso CLI, so authorization does not depend on a header the API cannot check.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// ErrUnauthorized is returned for every rejection. The reason is logged rather
// than returned to the caller, so a probe cannot use the error text to learn
// which check it failed.
var ErrUnauthorized = errors.New("unauthorized")

// ErrProviderUnavailable separates "the identity provider did not answer" from
// "this token is bad". Folding the two together tells a caller holding a
// perfectly good token to log in again, which cannot fix a network failure.
var ErrProviderUnavailable = errors.New("identity provider unavailable")

// discoveryTimeout bounds the startup fetch. Without it a provider that accepts
// the connection and never replies wedges the process before it binds a port,
// so the healthcheck never passes and nothing restarts a server that has not
// exited.
const discoveryTimeout = 10 * time.Second

// Identity is what a verified token establishes about the caller.
type Identity struct {
	Subject  string
	ClientID string
}

// Verifier checks token signatures against the issuer's JWKS and the claims
// against this API's expectations.
type Verifier struct {
	issuer         string
	clientIDPrefix string
	keySet         *oidc.RemoteKeySet
	now            func() time.Time
}

type discoveryDocument struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// NewVerifier resolves the issuer's JWKS endpoint from its discovery document.
// clientIDPrefix is the per-app half of the CLI client naming
// (`meso-cli-` matching `meso-cli-macmini`), and is what keeps a token minted
// for another product from being accepted here: Authelia does not populate the
// audience claim through the device authorization grant, so `aud` is empty and
// cannot carry that isolation.
func NewVerifier(ctx context.Context, issuer, clientIDPrefix string) (*Verifier, error) {
	if issuer == "" || clientIDPrefix == "" {
		return nil, errors.New("auth: issuer and clientIDPrefix are both required")
	}
	doc, err := fetchDiscovery(ctx, issuer)
	if err != nil {
		return nil, err
	}
	if doc.Issuer != issuer {
		return nil, fmt.Errorf("auth: issuer %q advertises itself as %q", issuer, doc.Issuer)
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("auth: issuer %q advertises no jwks_uri", issuer)
	}
	return &Verifier{
		issuer:         issuer,
		clientIDPrefix: clientIDPrefix,
		keySet:         oidc.NewRemoteKeySet(ctx, doc.JWKSURI),
		now:            time.Now,
	}, nil
}

func fetchDiscovery(ctx context.Context, issuer string) (discoveryDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return discoveryDocument{}, err
	}
	client := &http.Client{Timeout: discoveryTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("auth: reach %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return discoveryDocument{}, fmt.Errorf("auth: %s returned %s", url, resp.Status)
	}
	var doc discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return discoveryDocument{}, fmt.Errorf("auth: decode %s: %w", url, err)
	}
	return doc, nil
}

type accessTokenClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	ClientID string `json:"client_id"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
}

// Verify checks the signature and then every claim this API relies on. It
// returns ErrUnauthorized for all failures, with the reason in the wrapped
// error for logging.
func (v *Verifier) Verify(ctx context.Context, raw string) (Identity, error) {
	if err := requireAccessTokenType(raw); err != nil {
		return Identity{}, err
	}

	payload, err := v.keySet.VerifySignature(ctx, raw)
	if err != nil {
		if isRetrievalFailure(err) {
			return Identity{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
		}
		return Identity{}, fmt.Errorf("%w: signature: %v", ErrUnauthorized, err)
	}

	var claims accessTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Identity{}, fmt.Errorf("%w: claims are not JSON: %v", ErrUnauthorized, err)
	}
	if claims.Issuer != v.issuer {
		return Identity{}, fmt.Errorf("%w: issuer %q", ErrUnauthorized, claims.Issuer)
	}
	if claims.Subject == "" {
		return Identity{}, fmt.Errorf("%w: no subject", ErrUnauthorized)
	}
	if !strings.HasPrefix(claims.ClientID, v.clientIDPrefix) {
		return Identity{}, fmt.Errorf("%w: client %q is not a %s* client", ErrUnauthorized, claims.ClientID, v.clientIDPrefix)
	}
	if claims.Expiry == 0 || v.now().After(time.Unix(claims.Expiry, 0)) {
		return Identity{}, fmt.Errorf("%w: expired", ErrUnauthorized)
	}

	return Identity{Subject: claims.Subject, ClientID: claims.ClientID}, nil
}

// requireAccessTokenType rejects anything not typed as an RFC 9068 access
// token. An id_token from the same issuer carries a valid signature and issuer,
// so without this check it would authenticate — and it is handed to the client
// rather than kept secret from it.
func requireAccessTokenType(raw string) error {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return fmt.Errorf("%w: not a JWT", ErrUnauthorized)
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("%w: header is not base64url", ErrUnauthorized)
	}
	var header struct {
		Type string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return fmt.Errorf("%w: header is not JSON", ErrUnauthorized)
	}
	if !strings.EqualFold(header.Type, "at+jwt") {
		return fmt.Errorf("%w: token type %q is not at+jwt", ErrUnauthorized, header.Type)
	}
	return nil
}

// BearerToken extracts the credential from an Authorization header, returning
// "" when the header is absent or uses another scheme.
func BearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// LooksLikeJWT reports whether a credential has the three-part compact-JWS
// shape. The middleware routes on it so the two bearer credentials meso accepts
// stay disjoint: a JWT-shaped token is only ever graded against JWKS, and the
// service secret is only ever compared byte-for-byte. Without the split, a
// forged JWT would be offered to the secret comparison and a leaked secret to
// the verifier, and each check would be answering about the wrong credential.
func LooksLikeJWT(raw string) bool {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
	}
	return true
}

// isRetrievalFailure reports whether the key set could not be fetched, as
// opposed to the token failing against keys that were fetched. go-oidc does not
// type these separately, so the message is all there is to go on.
func isRetrievalFailure(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "fetching keys") || strings.Contains(msg, "oidc: get keys failed")
}
