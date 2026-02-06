package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"syncthing-dashboard/internal/model"
)

type snapshotReader interface {
	Snapshot() (model.DashboardSnapshot, bool)
	Ready() bool
}

// API hosts the read-only dashboard endpoints and static UI.
type API struct {
	reader       snapshotReader
	pageTitle    string
	pageSubtitle string
	pollInterval time.Duration
	mux          *http.ServeMux
}

func New(reader snapshotReader, pageTitle, pageSubtitle string, pollInterval time.Duration) *API {
	api := &API{
		reader:       reader,
		pageTitle:    pageTitle,
		pageSubtitle: pageSubtitle,
		pollInterval: pollInterval,
		mux:          http.NewServeMux(),
	}

	api.mux.HandleFunc("/api/v1/dashboard", api.handleDashboard)
	api.mux.HandleFunc("/healthz", api.handleHealthz)
	api.mux.HandleFunc("/readyz", api.handleReadyz)
	api.mux.Handle("/", http.FileServer(http.Dir("web")))

	return api
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *API) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	snapshot, ok := a.reader.Snapshot()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "snapshot unavailable"})
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, dashboardResponse{
		DashboardSnapshot: snapshot,
		PageTitle:         a.pageTitle,
		PageSubtitle:      a.pageSubtitle,
		PollIntervalMS:    a.pollInterval.Milliseconds(),
	})
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !a.reader.Ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]bool{"ready": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ready": true})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type dashboardResponse struct {
	model.DashboardSnapshot
	PageTitle      string `json:"page_title"`
	PageSubtitle   string `json:"page_subtitle"`
	PollIntervalMS int64  `json:"poll_interval_ms"`
}
