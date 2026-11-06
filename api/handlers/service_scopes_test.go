package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/datapointchris/goapiauth/authtest"
	"github.com/datapointchris/goapiauth/clientid"
	"github.com/datapointchris/gobrowserlogin"
	"github.com/stretchr/testify/assert"

	"meso/api/handlers"
	"meso/api/middleware"
)

// noSessions holds no browser session. Every request here carries a bearer
// token, which is judged on the token alone.
type noSessions struct{}

func (noSessions) Authenticate(*http.Request) (gobrowserlogin.Identity, error) {
	return gobrowserlogin.Identity{}, gobrowserlogin.ErrNoSession
}

// A service token reaches the routes its scopes list and gets 403 on every
// other one, through the auth production wraps around the route table.
func TestAServiceTokenReachesOnlyTheRoutesItsScopesList(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	created := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{"name": "Read by a service"}).Body)
	cyclePath := "/api/v1/cycles/" + strconv.FormatInt(created.ID, 10)

	provider := authtest.NewProvider(t)
	verifier := provider.Verifier(t, clientid.PersonPrefix("meso"), clientid.ServicePrefix("meso"))
	authed := middleware.RequireAuth(verifier, noSessions{}, mux, handlers.ServiceRoutes)
	scoped := func(scopes ...string) string {
		return provider.Sign(t, "at+jwt", map[string]any{"client_id": "meso-svc-dashboard", "scp": scopes})
	}
	person := provider.Person(t, "meso-cli-test", "user")
	cycles := scoped("meso.cycles.read")
	review := scoped("meso.review.read")
	feedback := scoped("meso.feedback.read")

	cases := []struct {
		token, method, path string
		want                int
	}{
		{cycles, http.MethodGet, "/api/v1/cycles", http.StatusOK},
		{cycles, http.MethodGet, cyclePath, http.StatusOK},
		{cycles, http.MethodGet, "/api/v1/review", http.StatusForbidden},
		{cycles, http.MethodPost, "/api/v1/cycles", http.StatusForbidden},
		{cycles, http.MethodDelete, cyclePath, http.StatusForbidden},
		{review, http.MethodGet, "/api/v1/review?since=2026-01-01T00:00:00Z", http.StatusOK},
		{review, http.MethodGet, "/api/v1/cycles", http.StatusForbidden},
		{review, http.MethodGet, "/api/v1/movements", http.StatusForbidden},
		{person, http.MethodGet, "/api/v1/movements", http.StatusOK},
		{feedback, http.MethodGet, "/api/v1/feedback?status=open", http.StatusOK},
		{feedback, http.MethodPost, "/api/v1/feedback", http.StatusForbidden},
		{cycles, http.MethodGet, "/api/v1/feedback", http.StatusForbidden},
		{person, http.MethodGet, "/api/v1/feedback", http.StatusForbidden},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, http.NoBody)
		req.Header.Set("Authorization", "Bearer "+c.token)
		rr := httptest.NewRecorder()
		authed.ServeHTTP(rr, req)
		assert.Equal(t, c.want, rr.Code, "%s %s: %s", c.method, c.path, rr.Body)
	}
}
