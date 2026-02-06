package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syncthing-dashboard/internal/model"
)

type fakeReader struct {
	snapshot model.DashboardSnapshot
	ok       bool
	ready    bool
}

func (f fakeReader) Snapshot() (model.DashboardSnapshot, bool) {
	return f.snapshot, f.ok
}

func (f fakeReader) Ready() bool {
	return f.ready
}

func TestDashboardEndpointReturnsSnapshot(t *testing.T) {
	api := New(fakeReader{
		snapshot: model.DashboardSnapshot{
			GeneratedAt:  time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC),
			SourceOnline: true,
		},
		ok:    true,
		ready: true,
	}, "Syncthing", "Read-Only Dashboard", 5*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store cache header")
	}

	var payload struct {
		model.DashboardSnapshot
		PageTitle      string `json:"page_title"`
		PageSubtitle   string `json:"page_subtitle"`
		PollIntervalMS int64  `json:"poll_interval_ms"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if !payload.SourceOnline {
		t.Fatalf("expected source_online true")
	}
	if payload.PageTitle != "Syncthing" || payload.PageSubtitle != "Read-Only Dashboard" {
		t.Fatalf("unexpected page branding: %+v", payload)
	}
	if payload.PollIntervalMS != 5000 {
		t.Fatalf("unexpected poll interval ms: %d", payload.PollIntervalMS)
	}
}

func TestDashboardEndpointMethodNotAllowed(t *testing.T) {
	api := New(fakeReader{ok: true, ready: true}, "Syncthing", "Read-Only Dashboard", 5*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard", nil)
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestReadyz(t *testing.T) {
	readyAPI := New(fakeReader{ok: true, ready: true}, "Syncthing", "Read-Only Dashboard", 5*time.Second)
	notReadyAPI := New(fakeReader{ok: false, ready: false}, "Syncthing", "Read-Only Dashboard", 5*time.Second)

	r1 := httptest.NewRecorder()
	readyAPI.ServeHTTP(r1, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if r1.Code != http.StatusOK {
		t.Fatalf("expected ready status 200, got %d", r1.Code)
	}

	r2 := httptest.NewRecorder()
	notReadyAPI.ServeHTTP(r2, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if r2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected not ready status 503, got %d", r2.Code)
	}
}
