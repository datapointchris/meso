package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A CLI logging in reads the issuer from authorization_servers, and refuses the
// document unless resource is the URL it asked about.
func TestProtectedResourceMetadata_NamesTheResourceAndItsIssuer(t *testing.T) {
	rec := httptest.NewRecorder()
	ProtectedResourceMetadata("https://meso.example.com", "https://auth.example.com").
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ProtectedResourcePath, nil))

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got protectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Resource != "https://meso.example.com" ||
		len(got.AuthorizationServers) != 1 || got.AuthorizationServers[0] != "https://auth.example.com" {
		t.Errorf("metadata = %+v", got)
	}
}
