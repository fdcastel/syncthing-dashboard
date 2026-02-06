package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"syncthing-dashboard/internal/model"
	"syncthing-dashboard/internal/syncthing"
)

func TestCollectorMapsSnapshotAndAlerts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/system/status":
			_, _ = w.Write([]byte(`{"myID":"LOCAL-1","uptime":120,"connectionServiceStatus":{"tcp://0.0.0.0:22000":{"error":null},"quic://0.0.0.0:22000":{"error":"bind failed"}},"discoveryStatus":{"global":{"error":null},"local":{"error":"disabled"}}}`))
		case "/rest/system/version":
			_, _ = w.Write([]byte(`{"version":"v2.0.1","os":"linux","arch":"amd64"}`))
		case "/rest/system/connections":
			_, _ = w.Write([]byte(`{"total":{"bitsPerSecondIn":8000,"bitsPerSecondOut":4000},"connections":{"REMOTE-1":{"address":"tcp://10.0.0.5:22000","connected":false,"inBytesTotal":100,"outBytesTotal":200}}}`))
		case "/rest/stats/device":
			_, _ = w.Write([]byte(`{"REMOTE-1":{"lastSeen":"2026-02-05T20:00:00Z"}}`))
		case "/rest/stats/folder":
			_, _ = w.Write([]byte(`{"app":{"lastScan":"2026-02-05T20:10:00Z"}}`))
		case "/rest/config":
			_, _ = w.Write([]byte(`{"devices":[{"deviceID":"LOCAL-1","name":"vault"},{"deviceID":"REMOTE-1","name":"BHS-HOST40"}],"folders":[{"id":"app","label":"app","path":"/mnt/vault/app","paused":false}]}`))
		case "/rest/db/status":
			if r.URL.Query().Get("folder") != "app" {
				t.Fatalf("expected folder=app")
			}
			_, _ = w.Write([]byte(`{"globalFiles":30,"localFiles":20,"localDirectories":7,"globalBytes":4096,"localBytes":2048,"needFiles":10,"needDirectories":0,"needSymlinks":0,"needDeletes":0,"needBytes":2048,"receiveOnlyTotalItems":3,"state":"syncing"}`))
		case "/rest/db/completion":
			if r.URL.Query().Get("folder") != "app" {
				t.Fatalf("expected folder=app")
			}
			_, _ = w.Write([]byte(`{"completion":8.1,"needBytes":3072,"needItems":12,"globalBytes":4096}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	client := syncthing.NewClient(ts.URL, "key", 2*time.Second, false)
	c := New(client, 5*time.Second)
	c.refresh(context.Background())

	snapshot, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected snapshot to be available")
	}

	if !snapshot.SourceOnline {
		t.Fatalf("expected source to be online")
	}
	if snapshot.Device.Name != "vault" {
		t.Fatalf("unexpected device name: %s", snapshot.Device.Name)
	}
	if snapshot.Device.DownloadBPS != 1000 {
		t.Fatalf("unexpected download rate: %f", snapshot.Device.DownloadBPS)
	}
	if snapshot.Device.LocalFilesTotal != 20 || snapshot.Device.LocalDirsTotal != 7 || snapshot.Device.LocalBytesTotal != 2048 {
		t.Fatalf("unexpected local state totals: %+v", snapshot.Device)
	}
	if snapshot.Device.ListenersOK != 1 || snapshot.Device.ListenersTotal != 2 {
		t.Fatalf("unexpected listeners health: %+v", snapshot.Device)
	}
	if snapshot.Device.DiscoveryOK != 1 || snapshot.Device.DiscoveryTotal != 2 {
		t.Fatalf("unexpected discovery health: %+v", snapshot.Device)
	}
	if len(snapshot.Folders) != 1 {
		t.Fatalf("expected 1 folder")
	}
	if snapshot.Folders[0].GlobalBytes != 4096 || snapshot.Folders[0].LocalBytes != 2048 {
		t.Fatalf("expected folder byte totals to be mapped")
	}
	if snapshot.Folders[0].NeedItems != 12 || snapshot.Folders[0].NeedBytes != 3072 {
		t.Fatalf("expected completion endpoint to refine need values")
	}
	if snapshot.Folders[0].LocalChangesItems != 3 {
		t.Fatalf("expected receive-only local changes to be mapped")
	}
	if snapshot.Folders[0].CompletionPct == nil || *snapshot.Folders[0].CompletionPct != 8.1 {
		t.Fatalf("expected completion_pct to be populated")
	}
	if len(snapshot.Remotes) != 1 || snapshot.Remotes[0].Connected {
		t.Fatalf("expected disconnected remote")
	}

	hasRemoteAlert := false
	hasFolderAlert := false
	for _, alert := range snapshot.Alerts {
		if alert.Code == "REMOTE_DISCONNECTED" {
			hasRemoteAlert = true
		}
		if alert.Code == "FOLDER_OUT_OF_SYNC" {
			hasFolderAlert = true
		}
	}
	if !hasRemoteAlert || !hasFolderAlert {
		t.Fatalf("expected both remote and folder alerts, got %+v", snapshot.Alerts)
	}
}

func TestCollectorSourceUnreachableAddsCriticalAlert(t *testing.T) {
	client := syncthing.NewClient("http://127.0.0.1:1", "key", 100*time.Millisecond, false)
	c := New(client, 5*time.Second)

	c.refresh(context.Background())
	snapshot, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected snapshot")
	}
	if snapshot.SourceOnline {
		t.Fatalf("expected source to be offline")
	}
	if len(snapshot.Alerts) == 0 || snapshot.Alerts[0].Code != "SOURCE_UNREACHABLE" {
		t.Fatalf("expected SOURCE_UNREACHABLE alert, got %+v", snapshot.Alerts)
	}
	if !snapshot.Stale {
		t.Fatalf("expected stale snapshot when source is unreachable")
	}
}

func TestSnapshotBecomesStaleByAge(t *testing.T) {
	c := &Collector{pollInterval: 5 * time.Second}
	c.snapshot = model.DashboardSnapshot{
		GeneratedAt:  time.Now().UTC().Add(-11 * time.Second),
		SourceOnline: true,
		Stale:        false,
	}
	c.hasSnapshot = true

	snapshot, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected snapshot")
	}
	if !snapshot.Stale {
		t.Fatalf("expected stale snapshot because it is older than 2*poll interval")
	}
}

func TestCollectorComputesRatesFromConnectionTotals(t *testing.T) {
	var connectionCalls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/system/status":
			_, _ = w.Write([]byte(`{"myID":"LOCAL-1","uptime":120}`))
		case "/rest/system/version":
			_, _ = w.Write([]byte(`{"version":"v2.0.1","os":"linux","arch":"amd64"}`))
		case "/rest/system/connections":
			currentCall := connectionCalls.Add(1)
			if currentCall == 1 {
				_, _ = w.Write([]byte(`{"total":{"inBytesTotal":1000,"outBytesTotal":2000},"connections":{}}`))
			} else {
				_, _ = w.Write([]byte(`{"total":{"inBytesTotal":1600,"outBytesTotal":2600},"connections":{}}`))
			}
		case "/rest/stats/device":
			_, _ = w.Write([]byte(`{}`))
		case "/rest/stats/folder":
			_, _ = w.Write([]byte(`{"app":{"lastScan":"2026-02-05T20:10:00Z"}}`))
		case "/rest/config":
			_, _ = w.Write([]byte(`{"devices":[{"deviceID":"LOCAL-1","name":"vault"}],"folders":[{"id":"app","label":"app","path":"/mnt/vault/app","paused":false}]}`))
		case "/rest/db/status":
			_, _ = w.Write([]byte(`{"globalFiles":1,"localFiles":1,"localDirectories":1,"globalBytes":1000,"localBytes":1000,"needFiles":0,"needDirectories":0,"needSymlinks":0,"needDeletes":0,"needBytes":0,"state":"idle"}`))
		case "/rest/db/completion":
			_, _ = w.Write([]byte(`{"completion":100,"needBytes":0,"needItems":0,"globalBytes":1000}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	client := syncthing.NewClient(ts.URL, "key", 2*time.Second, false)
	c := New(client, 5*time.Second)
	c.refresh(context.Background())
	time.Sleep(120 * time.Millisecond)
	c.refresh(context.Background())

	snapshot, ok := c.Snapshot()
	if !ok {
		t.Fatalf("expected snapshot")
	}
	if snapshot.Device.DownloadBPS <= 0 || snapshot.Device.UploadBPS <= 0 {
		t.Fatalf("expected positive rates from total byte deltas, got down=%f up=%f", snapshot.Device.DownloadBPS, snapshot.Device.UploadBPS)
	}
}
