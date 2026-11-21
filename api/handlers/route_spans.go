package handlers

import (
	"net/http"
	"strings"

	"github.com/datapointchris/goapiauth"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// recordNames names the record a path wildcard addresses, by the collection
// before it, and for a nested record by its parent's collection too.
var recordNames = map[string]string{
	"movements":          "movement",
	"movements/related":  "related_movement",
	"workouts":           "workout",
	"workouts/movements": "workout_movement",
	"sessions":           "session",
	"sessions/movements": "session_movement",
	"movements/sets":     "session_set",
	"metrics":            "metric",
	"measurements":       "measurement",
	"log":                "log_entry",
	"cycles":             "cycle",
	"cycles/workouts":    "cycle_workout",
	"feedback":           "feedback",
}

// pathRecord is one wildcard in a route's path and the span attribute its value is set as.
type pathRecord struct {
	wildcard  string
	attribute string
}

// pathRecords lists the records path addresses, outermost first: for
// /api/v1/sessions/{id}/movements/{entryID} that is meso.session.id and
// meso.session_movement.id.
func pathRecords(path string) []pathRecord {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	var records []pathRecord
	for i, segment := range segments {
		wildcard, ok := strings.CutPrefix(segment, "{")
		if !ok || i == 0 {
			continue
		}
		wildcard = strings.TrimSuffix(wildcard, "}")
		collection := segments[i-1]
		name, known := "", false
		if i >= 3 && strings.HasPrefix(segments[i-2], "{") {
			name, known = recordNames[segments[i-3]+"/"+collection]
		}
		if !known {
			name, known = recordNames[collection]
		}
		if !known {
			name = collection
		}
		field := "id"
		if wildcard == "name" {
			field = "name"
		}
		records = append(records, pathRecord{wildcard: wildcard, attribute: "meso." + name + "." + field})
	}
	return records
}

// withRouteSpan names the request's server span for the route that answered it,
// as `{method} {route}`, and sets on it the route, each record the path
// addresses, and who called. The span is started outside the authentication
// middleware, which hands the mux a copy of the request, so the route the mux
// matched is set from here rather than read back by the span's starter.
func withRouteSpan(pattern string, handle http.HandlerFunc) http.HandlerFunc {
	route := pattern
	if _, path, hasMethod := strings.Cut(pattern, " "); hasMethod {
		route = path
	}
	records := pathRecords(route)
	return func(w http.ResponseWriter, r *http.Request) {
		if span := trace.SpanFromContext(r.Context()); span.IsRecording() {
			span.SetName(r.Method + " " + route)
			attributes := []attribute.KeyValue{semconv.HTTPRoute(route)}
			for _, record := range records {
				attributes = append(attributes, attribute.String(record.attribute, r.PathValue(record.wildcard)))
			}
			if identity, ok := goapiauth.IdentityFrom(r.Context()); ok {
				if identity.Subject != "" {
					attributes = append(attributes, semconv.EnduserID(identity.Subject))
				}
				if identity.ClientID != "" {
					attributes = append(attributes, attribute.String("meso.oauth.client_id", identity.ClientID))
				}
			}
			span.SetAttributes(attributes...)
		}
		handle(w, r)
	}
}
