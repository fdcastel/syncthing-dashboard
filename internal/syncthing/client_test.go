package syncthing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetJSONRejectsUnknownPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "token", 2*time.Second, false)

	var out map[string]any
	err := client.getJSON(context.Background(), "/rest/system/restart", nil, &out)
	if err == nil {
		t.Fatalf("expected error for blocked path")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected allowlist error, got %v", err)
	}
}

func TestGetDBStatusUsesAllowlistedPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/db/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Query().Get("folder") != "docs" {
			t.Fatalf("missing folder query")
		}
		if r.Header.Get("X-API-Key") != "secret" {
			t.Fatalf("missing api key header")
		}
		_, _ = w.Write([]byte(`{"globalFiles":11,"localFiles":10,"needFiles":1,"needDirectories":0,"needSymlinks":0,"needDeletes":0,"needBytes":1024,"state":"syncing"}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "secret", 2*time.Second, false)
	status, err := client.GetDBStatus(context.Background(), "docs")
	if err != nil {
		t.Fatalf("GetDBStatus failed: %v", err)
	}
	if status.NeedFiles != 1 {
		t.Fatalf("unexpected needFiles: %d", status.NeedFiles)
	}
}

func TestGetDBCompletionUsesAllowlistedPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/db/completion" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Query().Get("folder") != "docs" {
			t.Fatalf("missing folder query")
		}
		if r.Header.Get("X-API-Key") != "secret" {
			t.Fatalf("missing api key header")
		}
		_, _ = w.Write([]byte(`{"completion":25.5,"needBytes":1024,"needItems":5,"globalBytes":4096}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "secret", 2*time.Second, false)
	status, err := client.GetDBCompletion(context.Background(), "docs")
	if err != nil {
		t.Fatalf("GetDBCompletion failed: %v", err)
	}
	if status.Completion != 25.5 || status.NeedItems != 5 {
		t.Fatalf("unexpected completion payload: %+v", status)
	}
}
