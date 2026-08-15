package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

const testKeyID = "main"

type testIDP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newTestIDP(t *testing.T) *testIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	idp := &testIDP{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   idp.server.URL,
			"jwks_uri": idp.server.URL + "/jwks.json",
		})
	})
	mux.HandleFunc("/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       key.Public(),
			KeyID:     testKeyID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}}}
		_ = json.NewEncoder(w).Encode(set)
	})
	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

// sign mints a token with the given header type and claims, signed by the IDP's
// key unless a different one is supplied.
func (i *testIDP) sign(t *testing.T, typ string, claims map[string]any, key *rsa.PrivateKey) string {
	t.Helper()
	if key == nil {
		key = i.key
	}
	opts := (&jose.SignerOptions{}).WithType(jose.ContentType(typ))
	opts.WithHeader("kid", testKeyID)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, opts)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	raw, err := obj.CompactSerialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	return raw
}

func (i *testIDP) validClaims() map[string]any {
	return map[string]any{
		"iss":       i.server.URL,
		"sub":       "user-uuid",
		"client_id": "meso-cli-macmini",
		"iat":       time.Now().Add(-time.Minute).Unix(),
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
}

func newTestVerifier(t *testing.T, idp *testIDP) *Verifier {
	t.Helper()
	v, err := NewVerifier(context.Background(), idp.server.URL, "meso-cli-")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

func TestVerify_AcceptsValidAccessToken(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	identity, err := v.Verify(context.Background(), idp.sign(t, "at+jwt", idp.validClaims(), nil))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if identity.Subject != "user-uuid" {
		t.Fatalf("Subject = %q", identity.Subject)
	}
	if identity.ClientID != "meso-cli-macmini" {
		t.Fatalf("ClientID = %q", identity.ClientID)
	}
}

// An id_token is signed by the same key and carries the same issuer, and unlike
// the access token it is handed to the browser. Only the header type separates
// them.
func TestVerify_RejectsIDToken(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	claims := idp.validClaims()
	claims["aud"] = []string{"meso-cli-macmini"}

	_, err := v.Verify(context.Background(), idp.sign(t, "JWT", claims, nil))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for an id_token, got %v", err)
	}
}

func TestVerify_RejectsForeignSigningKey(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	attacker, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate attacker key: %v", err)
	}

	_, err = v.Verify(context.Background(), idp.sign(t, "at+jwt", idp.validClaims(), attacker))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for a foreign key, got %v", err)
	}
}

func TestVerify_RejectsExpired(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	claims := idp.validClaims()
	claims["exp"] = time.Now().Add(-time.Minute).Unix()

	_, err := v.Verify(context.Background(), idp.sign(t, "at+jwt", claims, nil))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for an expired token, got %v", err)
	}
}

func TestVerify_RejectsWrongIssuer(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	claims := idp.validClaims()
	claims["iss"] = "https://evil.example.com"

	_, err := v.Verify(context.Background(), idp.sign(t, "at+jwt", claims, nil))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for a wrong issuer, got %v", err)
	}
}

// Authelia leaves `aud` empty in the device grant, so cross-product isolation
// rests entirely on this check.
func TestVerify_RejectsAnotherProductsClient(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	claims := idp.validClaims()
	claims["client_id"] = "nomad-cli-macmini"

	_, err := v.Verify(context.Background(), idp.sign(t, "at+jwt", claims, nil))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for another product's client, got %v", err)
	}
}

func TestVerify_RejectsMissingSubject(t *testing.T) {
	idp := newTestIDP(t)
	v := newTestVerifier(t, idp)

	claims := idp.validClaims()
	delete(claims, "sub")

	_, err := v.Verify(context.Background(), idp.sign(t, "at+jwt", claims, nil))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for a token with no subject, got %v", err)
	}
}

func TestBearerToken(t *testing.T) {
	cases := map[string]string{
		"Bearer abc.def.ghi": "abc.def.ghi",
		"bearer abc.def.ghi": "abc.def.ghi",
		"Basic dXNlcjpwdw==": "",
		"":                   "",
		"Bearer":             "",
	}
	for header, want := range cases {
		if got := BearerToken(header); got != want {
			t.Errorf("BearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}

// The shape test is what keeps the JWT and shared-secret paths disjoint, so a
// service token must never be mistaken for a JWT and vice versa.
func TestLooksLikeJWT(t *testing.T) {
	cases := map[string]bool{
		"abc.def.ghi":                true,
		"s3cr3t-service-token":       false,
		"abc.def":                    false,
		"abc.def.ghi.jkl":            false,
		"":                           false,
		".def.ghi":                   false,
		"abc..ghi":                   false,
		"abc.def.":                   false,
		"not.a.jwt.but-close":        false,
		"dotted.service.token-value": true,
	}
	for raw, want := range cases {
		if got := LooksLikeJWT(raw); got != want {
			t.Errorf("LooksLikeJWT(%q) = %v, want %v", raw, got, want)
		}
	}
}

// Cloudflare fronts the real issuer and refuses an unidentified client, so the agent has to be
// set on the JWKS refreshes too, not only on the discovery fetch at startup.
func TestVerify_IdentifiesItselfToTheIssuer(t *testing.T) {
	var mu sync.Mutex
	var agents []string

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	mux := http.NewServeMux()
	var base string
	record := func(r *http.Request) {
		mu.Lock()
		agents = append(agents, r.UserAgent())
		mu.Unlock()
	}
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "jwks_uri": base + "/jwks.json"})
	})
	mux.HandleFunc("/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key: key.Public(), KeyID: testKeyID, Algorithm: string(jose.RS256), Use: "sig",
		}}}
		_ = json.NewEncoder(w).Encode(set)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	base = server.URL

	v, err := NewVerifier(context.Background(), base, "cli-")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	// A well-formed at+jwt reaches the key set; a malformed one fails the type check first and
	// would leave the JWKS request untested.
	opts := (&jose.SignerOptions{}).WithType("at+jwt")
	opts.WithHeader("kid", testKeyID)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, opts)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	obj, err := signer.Sign([]byte(`{"iss":"x","sub":"s","client_id":"cli-x","exp":1}`))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	raw, err := obj.CompactSerialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	_, _ = v.Verify(context.Background(), raw)

	mu.Lock()
	defer mu.Unlock()
	if len(agents) < 2 {
		t.Fatalf("expected discovery and a JWKS fetch, got %d request(s)", len(agents))
	}
	for _, agent := range agents {
		if agent != userAgent {
			t.Fatalf("User-Agent = %q, want %q", agent, userAgent)
		}
	}
}
