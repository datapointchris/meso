package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// staticTokenClient mimics the oauth2 client the CLI injects: an http.Client whose
// transport stamps a fixed bearer token on every request.
func staticTokenClient(token string) *http.Client {
	return &http.Client{Transport: bearerTransport{token: token}}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(req)
}

// captured records what the fake server received, so tests can assert on the
// method, path+query, and decoded JSON body.
type captured struct {
	method string
	path   string
	query  string
	body   map[string]any
}

// recordingServer returns a server that replies with status+respBody and captures
// the request it saw.
func recordingServer(t *testing.T, status int, respBody string) (*httptest.Server, *captured) {
	t.Helper()
	got := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.query = r.URL.RawQuery
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &got.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func TestListMovements_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":1,"name":"Deadlift","movement_kind":"exercise","favorite":true,"tags":["strength"],"equipment":["barbell"],
		 "muscles":[{"muscle":"hamstrings","region":"posterior","role":"primary"}]}
	]`)
	client := New(srv.URL, staticTokenClient("t"))

	fav := true
	movements, err := client.ListMovements(context.Background(), MovementFilter{Kind: "exercise", Region: "posterior", Favorite: &fav})
	if err != nil {
		t.Fatalf("ListMovements: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/movements" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"kind=exercise", "region=posterior", "favorite=true"} {
		if !containsParam(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
	if len(movements) != 1 || movements[0].Name != "Deadlift" {
		t.Fatalf("decoded = %+v", movements)
	}
	if len(movements[0].PrimaryMuscles()) != 1 || movements[0].PrimaryMuscles()[0] != "hamstrings" {
		t.Errorf("primary muscles = %v", movements[0].PrimaryMuscles())
	}
}

func TestListMovements_NoFilterHasNoQuery(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[]`)
	client := New(srv.URL, staticTokenClient("t"))
	if _, err := client.ListMovements(context.Background(), MovementFilter{}); err != nil {
		t.Fatalf("ListMovements: %v", err)
	}
	if got.query != "" {
		t.Errorf("expected no query string, got %q", got.query)
	}
}

func TestGetMovement_NotFound(t *testing.T) {
	srv, _ := recordingServer(t, http.StatusNotFound, `{"error":"Not Found","message":"movement 9 not found"}`)
	client := New(srv.URL, staticTokenClient("t"))

	_, err := client.GetMovement(context.Background(), 9)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.NotFound() {
		t.Errorf("expected 404 APIError, got %v", err)
	}
}

func TestCreateMovement_SendsBodyOmitsUnsetNullables(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated, `{"id":5,"name":"Front Squat","movement_kind":"exercise"}`)
	client := New(srv.URL, staticTokenClient("t"))

	m, err := client.CreateMovement(context.Background(), MovementCreate{
		Name: "Front Squat", MovementKind: "exercise", Tags: []string{"legs"},
		Muscles: []MuscleInput{{Muscle: "quads", Role: "primary"}},
	})
	if err != nil {
		t.Fatalf("CreateMovement: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/movements" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["name"] != "Front Squat" || got.body["movement_kind"] != "exercise" {
		t.Errorf("body = %v", got.body)
	}
	// omitempty: unset nullable pointers must not appear.
	if _, ok := got.body["rating"]; ok {
		t.Errorf("unset rating should be omitted, body = %v", got.body)
	}
	if m.ID != 5 {
		t.Errorf("decoded = %+v", m)
	}
}

func TestUpdateMovement_SendsOnlyPatch(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":3,"name":"Deadlift","favorite":true}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.UpdateMovement(context.Background(), 3, map[string]any{"favorite": true}); err != nil {
		t.Fatalf("UpdateMovement: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/api/v1/movements/3" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if len(got.body) != 1 || got.body["favorite"] != true {
		t.Errorf("only favorite should be sent, got %v", got.body)
	}
}

func TestDeleteMovement_Conflict(t *testing.T) {
	srv, got := recordingServer(t, http.StatusConflict, `{"error":"Conflict","message":"referenced"}`)
	client := New(srv.URL, staticTokenClient("t"))

	err := client.DeleteMovement(context.Background(), 3)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 APIError, got %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/movements/3" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

// containsParam reports whether an encoded query string contains the given
// key=value pair (order-independent).
func containsParam(query, pair string) bool {
	for _, p := range splitAmp(query) {
		if p == pair {
			return true
		}
	}
	return false
}

func splitAmp(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '&' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
