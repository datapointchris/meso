package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/datapointchris/meso/cli/internal/api"
)

// An admin command refused by the group gate prints the API's own reason, which
// the CLI dropped while the gate answered in plain text.
func TestHandleAPIErrorPrintsTheAPIsReasonForA403(t *testing.T) {
	t.Parallel()

	refusal := &api.APIError{
		StatusCode: http.StatusForbidden, Status: "403 Forbidden",
		Message: "this needs membership in the admin group",
	}
	got := handleAPIError(refusal).Error()
	for _, want := range []string{"this needs membership in the admin group", "meso auth login"} {
		if !strings.Contains(got, want) {
			t.Errorf("handleAPIError(403) = %q, missing %q", got, want)
		}
	}
}
