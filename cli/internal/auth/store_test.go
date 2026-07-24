package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

// fakeKeyring is an in-memory stand-in for the OS keychain. go-keyring's own
// MockInit is process-global and unsafe under parallel tests, so the store
// depends on the keyringStore seam and tests inject this.
type fakeKeyring struct {
	entries map[string]string
}

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{entries: map[string]string{}} }

func key(service, user string) string { return service + "\x00" + user }

func (f *fakeKeyring) Set(service, user, password string) error {
	f.entries[key(service, user)] = password
	return nil
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
	v, ok := f.entries[key(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if _, ok := f.entries[key(service, user)]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.entries, key(service, user))
	return nil
}

func storeWithFake() *TokenStore { return &TokenStore{backend: newFakeKeyring()} }

func TestTokenStore_RoundTrip(t *testing.T) {
	store := storeWithFake()
	want := &oauth2.Token{
		AccessToken:  "authelia_at_opaque",
		RefreshToken: "authelia_rt_opaque",
		TokenType:    "bearer",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second),
	}

	if err := store.Save("meso-cli-macmini", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Load("meso-cli-macmini")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("token mismatch: got %+v want %+v", got, want)
	}
	if !got.Expiry.Equal(want.Expiry) {
		t.Fatalf("expiry mismatch: got %v want %v", got.Expiry, want.Expiry)
	}
}

func TestTokenStore_LoadMissingIsNotLoggedIn(t *testing.T) {
	store := storeWithFake()
	_, err := store.Load("meso-cli-nobody")
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

func TestTokenStore_DeleteMissingIsNotLoggedIn(t *testing.T) {
	store := storeWithFake()
	if err := store.Delete("meso-cli-nobody"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

func TestTokenStore_DeleteRemoves(t *testing.T) {
	store := storeWithFake()
	_ = store.Save("meso-cli-macmini", &oauth2.Token{AccessToken: "x"})
	if err := store.Delete("meso-cli-macmini"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Load("meso-cli-macmini"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn after delete, got %v", err)
	}
}

func TestTokenStore_NamespacedByClientID(t *testing.T) {
	store := storeWithFake()
	_ = store.Save("meso-cli-mbp", &oauth2.Token{AccessToken: "mbp-token"})
	_ = store.Save("meso-cli-macmini", &oauth2.Token{AccessToken: "macmini-token"})

	mbp, _ := store.Load("meso-cli-mbp")
	macmini, _ := store.Load("meso-cli-macmini")
	if mbp.AccessToken != "mbp-token" || macmini.AccessToken != "macmini-token" {
		t.Fatalf("tokens collided across client ids: mbp=%q macmini=%q", mbp.AccessToken, macmini.AccessToken)
	}
}
