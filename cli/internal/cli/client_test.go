package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/datapointchris/meso/cli/internal/api"
)

func TestHandleAPIErrorNamesTheAdminGroupAndTheLoginOnA403(t *testing.T) {
	t.Parallel()

	got := handleAPIError(&api.APIError{StatusCode: http.StatusForbidden, Status: "403 Forbidden"}).Error()
	for _, want := range []string{"admin group", "meso auth login"} {
		if !strings.Contains(got, want) {
			t.Errorf("handleAPIError(403) = %q, missing %q", got, want)
		}
	}
}
