package auth

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

// keyringService namespaces meso's entries in the OS keychain. Tokens are keyed
// within it by client_id so multiple machines/apps sharing a synced keychain do
// not collide.
const keyringService = "meso-cli"

// ErrNotLoggedIn is returned when no token is stored for the client.
var ErrNotLoggedIn = errors.New("not logged in")

// storedToken is the on-disk shape. oauth2.Token keeps the id_token only in its
// Extra map, which does not survive a JSON round-trip, so it is persisted
// explicitly alongside the standard fields.
type storedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	IDToken      string    `json:"id_token,omitempty"`
}

func (s storedToken) toOAuth2() *oauth2.Token {
	tok := &oauth2.Token{
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		TokenType:    s.TokenType,
		Expiry:       s.Expiry,
	}
	if s.IDToken != "" {
		tok = tok.WithExtra(map[string]any{"id_token": s.IDToken})
	}
	return tok
}

func fromOAuth2(tok *oauth2.Token) storedToken {
	s := storedToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
	}
	if id, ok := tok.Extra("id_token").(string); ok {
		s.IDToken = id
	}
	return s
}

// keyringStore is the seam that lets tests swap the OS keychain for an in-memory
// fake — go-keyring's MockInit is process-global and racy under parallel tests,
// so the store depends on this interface instead.
type keyringStore interface {
	Set(service, user, password string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
}

type osKeyring struct{}

func (osKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (osKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (osKeyring) Delete(service, user string) error        { return keyring.Delete(service, user) }

// TokenStore persists OAuth tokens in the OS keychain.
type TokenStore struct {
	backend keyringStore
}

func NewTokenStore() *TokenStore { return &TokenStore{backend: osKeyring{}} }

func (t *TokenStore) Save(clientID string, tok *oauth2.Token) error {
	data, err := json.Marshal(fromOAuth2(tok))
	if err != nil {
		return err
	}
	return t.backend.Set(keyringService, clientID, string(data))
}

func (t *TokenStore) Load(clientID string) (*oauth2.Token, error) {
	raw, err := t.backend.Get(keyringService, clientID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	var s storedToken
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	return s.toOAuth2(), nil
}

func (t *TokenStore) Delete(clientID string) error {
	err := t.backend.Delete(keyringService, clientID)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotLoggedIn
	}
	return err
}
