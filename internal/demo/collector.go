package demo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"syncthing-dashboard/internal/model"
)

const (
	kib = 1024
	mib = 1024 * kib
	gib = 1024 * mib
)

// Collector produces rich synthetic snapshots for demonstration mode.
type Collector struct {
	pollInterval time.Duration

	mu       sync.RWMutex
	snapshot model.DashboardSnapshot
	ready    bool
	tick     int
	startAt  time.Time
}

func NewCollector(pollInterval time.Duration) *Collector {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Collector{
		pollInterval: pollInterval,
		startAt:      time.Now().UTC().Add(-73 * time.Hour),
	}
}

func (c *Collector) Start(ctx context.Context) {
	c.refresh()

	ticker := time.NewTicker(c.pollInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.refresh()
			}
		}
	}()
}

func (c *Collector) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

func (c *Collector) Snapshot() (model.DashboardSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.ready {
		return model.DashboardSnapshot{}, false
	}

	out := c.snapshot
	if time.Since(out.GeneratedAt) > 2*c.pollInterval {
		out.Stale = true
	}
	return out, true
}

func (c *Collector) refresh() {
	now := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.snapshot = buildSnapshot(now, c.tick, c.startAt, c.pollInterval)
	c.snapshot.GeneratedAt = now
	c.ready = true
	c.tick++
}

type folderSeed struct {
	ID           string
	Label        string
	Path         string
	Mode         string
	GlobalFiles  int64
	GlobalBytes  int64
	BaseProgress float64
	Speed        int
	LocalChanges int64
}

func buildSnapshot(now time.Time, tick int, startAt time.Time, pollInterval time.Duration) model.DashboardSnapshot {
	folders := buildFolders(now, tick)
	remotes := buildRemotes(now, tick)
	device := buildDevice(now, tick, startAt, pollInterval, folders)
	alerts := buildAlerts(remotes, folders)

	return model.DashboardSnapshot{
		GeneratedAt:  now,
		SourceOnline: true,
		SourceError:  nil,
		Device:       device,
		Folders:      folders,
		Remotes:      remotes,
		Alerts:       alerts,
		Stale:        false,
	}
}

func buildDevice(now time.Time, tick int, startAt time.Time, pollInterval time.Duration, folders []model.FolderStatus) model.DeviceStatus {
	var totalFiles int64
	var totalDirs int64
	var totalBytes int64
	for _, folder := range folders {
		totalFiles += folder.LocalFiles
		totalDirs += maxInt64(1, folder.LocalFiles/2)
		totalBytes += folder.LocalBytes
	}

	downloadBPS := (2.3 + float64((tick*3)%10)/10.0) * mib
	uploadBPS := (145 + float64((tick*17)%115)) * kib
	uptime := now.Sub(startAt).Seconds() + float64(tick)*pollInterval.Seconds()

	listenersTotal := 2
	listenersOK := 2
	if tick%23 >= 19 {
		listenersOK = 1
	}

	discoveryTotal := 5
	discoveryOK := 4
	if tick%19 == 0 {
		discoveryOK = 3
	}

	return model.DeviceStatus{
		Name:            "Homelab",
		ID:              "HOMELAB-DEMO-A4M9QY7-TK2N6PT-MV7R2FD-GQ9Y1LK-R8SN4WU-CP6E2JD-7YQ4HTA",
		Version:         "v2.0.12 linux amd64",
		UptimeS:         int64(uptime),
		DownloadBPS:     downloadBPS,
		UploadBPS:       uploadBPS,
		LocalFilesTotal: totalFiles,
		LocalDirsTotal:  totalDirs,
		LocalBytesTotal: totalBytes,
		ListenersOK:     listenersOK,
		ListenersTotal:  listenersTotal,
		DiscoveryOK:     discoveryOK,
		DiscoveryTotal:  discoveryTotal,
	}
}

func buildFolders(now time.Time, tick int) []model.FolderStatus {
	seeds := []folderSeed{
		{"folder-pictures", "Pictures", "/sync/Pictures", "local", 182, 136 * gib, 100, 0, 9},
		{"folder-documents", "Documents", "/sync/Documents", "idle", 96, 42 * gib, 100, 0, 0},
		{"folder-media", "Media", "/sync/Media", "syncing", 214, 328 * gib, 35, 3, 0},
		{"folder-music", "Music", "/sync/Music", "syncing", 484, 78 * gib, 64, 4, 0},
		{"folder-videos", "Videos", "/sync/Videos", "error", 33, 512 * gib, 73, 0, 0},
		{"folder-downloads", "Downloads", "/sync/Downloads", "scanning", 127, 58 * gib, 100, 0, 0},
		{"folder-projects", "Projects", "/sync/Projects", "syncing", 71, 24 * gib, 12, 5, 0},
		{"folder-backups", "Backups", "/sync/Backups", "paused", 65, 910 * gib, 100, 0, 0},
		{"folder-books", "Books", "/sync/Books", "local", 143, 19 * gib, 100, 0, 3},
		{"folder-taxes", "Taxes", "/sync/Taxes", "idle", 22, 4 * gib, 100, 0, 0},
	}

	folders := make([]model.FolderStatus, 0, len(seeds))
	for idx, seed := range seeds {
		state := "idle"
		needItems := int64(0)
		needBytes := int64(0)
		localChanges := seed.LocalChanges
		globalBytes := seed.GlobalBytes
		localBytes := seed.GlobalBytes
		localFiles := seed.GlobalFiles
		completion := 100.0

		switch seed.Mode {
		case "syncing":
			state = "syncing"
			progress := seed.BaseProgress + float64((tick*seed.Speed+idx)%19)
			if progress > 96 {
				progress = 96 - float64((tick+idx)%7)
			}
			if progress < 1 {
				progress = 1
			}
			completion = progress
			needBytes = int64(float64(seed.GlobalBytes) * ((100 - progress) / 100))
			if needBytes < 64*mib {
				needBytes = 64 * mib
			}
			localBytes = maxInt64(0, seed.GlobalBytes-needBytes)
			needItems = maxInt64(1, int64(float64(seed.GlobalFiles)*(100-progress)/100))
			localFiles = maxInt64(0, seed.GlobalFiles-needItems/2)
			if tick%11 == 0 && idx%2 == 0 {
				state = "scan-waiting"
			}
		case "local":
			state = "idle"
			localChanges = maxInt64(1, seed.LocalChanges+int64(tick%3))
		case "paused":
			state = "paused"
		case "scanning":
			state = "scan-waiting"
		case "error":
			state = "error"
			completion = 72
			needBytes = int64(float64(seed.GlobalBytes) * 0.28)
			needItems = maxInt64(3, seed.GlobalFiles/4)
			localBytes = maxInt64(0, seed.GlobalBytes-needBytes)
		}

		lastScan := now.Add(-time.Duration((idx*13+tick)%170) * time.Minute).UTC()
		completionCopy := completion
		folders = append(folders, model.FolderStatus{
			ID:                seed.ID,
			Label:             seed.Label,
			Path:              seed.Path,
			State:             state,
			GlobalFiles:       seed.GlobalFiles,
			LocalFiles:        localFiles,
			GlobalBytes:       globalBytes,
			LocalBytes:        localBytes,
			NeedItems:         needItems,
			NeedBytes:         needBytes,
			LocalChangesItems: localChanges,
			CompletionPct:     &completionCopy,
			LastScanAt:        &lastScan,
		})
	}

	return folders
}

type remoteSeed struct {
	ID      string
	Name    string
	Address string
	Mode    string
}

func buildRemotes(now time.Time, tick int) []model.RemoteDeviceStatus {
	seeds := []remoteSeed{
		{"ATTIC-DEMO-J24XQXQ-HC2SY5M-NUQ6R7L-W7K6WTV-J5Z62DW-ZZQKAMA-2YBDAQH", "Attic", "192.168.10.24:22000", "up"},
		{"DESK-DEMO-J24XQXQ-HC2SY5M-NUQ6R7L-W7K6WTV-J5Z62DW-ZZQKAMA-2YBDAQH", "Desk", "192.168.10.42:22000", "up"},
		{"BACKPACK-DEMO-J24XQXQ-HC2SY5M-NUQ6R7L-W7K6WTV-J5Z62DW-ZZQKAMA-2YBDAQH", "Backpack", "100.88.14.7:22000", "flap"},
		{"KEYRING-DEMO-J24XQXQ-HC2SY5M-NUQ6R7L-W7K6WTV-J5Z62DW-ZZQKAMA-2YBDAQH", "Keyring", "10.8.0.18:22000", "down"},
	}

	remotes := make([]model.RemoteDeviceStatus, 0, len(seeds))
	for idx, seed := range seeds {
		connected := true
		switch seed.Mode {
		case "down":
			connected = false
		case "flap":
			connected = tick%7 != 0 && tick%7 != 1
		}

		lastSeen := now.Add(-time.Duration((idx+1)*(tick%5+1)) * time.Minute).UTC()
		inBytes := int64((120+idx*14)*gib) + int64(tick*idx*41*mib)
		outBytes := int64((3+idx)*gib) + int64(tick*idx*11*mib)

		remotes = append(remotes, model.RemoteDeviceStatus{
			ID:            seed.ID,
			Name:          seed.Name,
			Connected:     connected,
			Address:       seed.Address,
			LastSeenAt:    &lastSeen,
			InBytesTotal:  inBytes,
			OutBytesTotal: outBytes,
		})
	}

	return remotes
}

func buildAlerts(remotes []model.RemoteDeviceStatus, folders []model.FolderStatus) []model.Alert {
	alerts := make([]model.Alert, 0)

	for _, remote := range remotes {
		if remote.Connected {
			continue
		}
		alerts = append(alerts, model.Alert{
			Severity:  "critical",
			Code:      "REMOTE_DISCONNECTED",
			Message:   fmt.Sprintf("Remote device %s is disconnected", remote.Name),
			SubjectID: remote.ID,
		})
	}

	for _, folder := range folders {
		if strings.EqualFold(folder.State, "error") {
			alerts = append(alerts, model.Alert{
				Severity:  "critical",
				Code:      "FOLDER_ERROR",
				Message:   fmt.Sprintf("Folder %s reports error state", folder.Label),
				SubjectID: folder.ID,
			})
			continue
		}

		if folder.NeedItems > 0 || folder.NeedBytes > 0 {
			alerts = append(alerts, model.Alert{
				Severity:  "warn",
				Code:      "FOLDER_OUT_OF_SYNC",
				Message:   fmt.Sprintf("Folder %s has pending sync items", folder.Label),
				SubjectID: folder.ID,
			})
		}
	}

	return alerts
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
