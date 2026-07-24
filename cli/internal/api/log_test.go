package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListLog_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":"018f-a","entry_date":"2026-07-25","body":"mobility","tags":["mobility"],"mood":null,"created_at":"","updated_at":""},
		{"id":"018f-b","entry_date":"2026-07-20","body":"deadlifts","tags":["strength","knee"],"mood":"focused","created_at":"","updated_at":""}
	]`)
	client := New(srv.URL, staticTokenClient("t"))

	entries, err := client.ListLog(context.Background(), LogFilter{From: "2026-07-01", Tag: "strength"})
	if err != nil {
		t.Fatalf("ListLog: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/log" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"from=2026-07-01", "tag=strength"} {
		if !containsParam(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
	if len(entries) != 2 || entries[1].Mood == nil || *entries[1].Mood != "focused" {
		t.Fatalf("decoded = %+v", entries)
	}
	if len(entries[1].Tags) != 2 || entries[1].Tags[1] != "knee" {
		t.Errorf("tags = %v", entries[1].Tags)
	}
}

func TestCreateLogEntry_SendsBody(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated,
		`{"id":"018f-c","entry_date":"2026-07-24","body":"note","tags":["rest"],"mood":"tired","created_at":"","updated_at":""}`)
	client := New(srv.URL, staticTokenClient("t"))

	mood := "tired"
	if _, err := client.CreateLogEntry(context.Background(),
		LogEntryCreate{Body: "note", EntryDate: "2026-07-24", Tags: []string{"rest"}, Mood: &mood}); err != nil {
		t.Fatalf("CreateLogEntry: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/log" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["body"] != "note" || got.body["mood"] != "tired" {
		t.Errorf("body = %v", got.body)
	}
}

func TestUpdateLogEntry_SendsPatch(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK,
		`{"id":"018f-c","entry_date":"2026-07-24","body":"revised","tags":[],"mood":null,"created_at":"","updated_at":""}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.UpdateLogEntry(context.Background(), "018f-c", map[string]any{"body": "revised"}); err != nil {
		t.Fatalf("UpdateLogEntry: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/api/v1/log/018f-c" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["body"] != "revised" {
		t.Errorf("body = %v", got.body)
	}
}

func TestDeleteLogEntry(t *testing.T) {
	srv, got := recordingServer(t, http.StatusNoContent, "")
	client := New(srv.URL, staticTokenClient("t"))

	if err := client.DeleteLogEntry(context.Background(), "018f-c"); err != nil {
		t.Fatalf("DeleteLogEntry: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/log/018f-c" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}
