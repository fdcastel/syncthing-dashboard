package collector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"syncthing-dashboard/internal/model"
	"syncthing-dashboard/internal/syncthing"
)

// Collector keeps an in-memory snapshot that is refreshed on an interval.
type Collector struct {
	client       *syncthing.Client
	pollInterval time.Duration

	mu           sync.RWMutex
	snapshot     model.DashboardSnapshot
	hasSnapshot  bool
	lastGood     model.DashboardSnapshot
	hasLastGood  bool
	lastRateAt   time.Time
	lastInTotal  int64
	lastOutTotal int64
}

func New(client *syncthing.Client, pollInterval time.Duration) *Collector {
	return &Collector{
		client:       client,
		pollInterval: pollInterval,
	}
}

func (c *Collector) Start(ctx context.Context) {
	c.refresh(ctx)

	ticker := time.NewTicker(c.pollInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.refresh(ctx)
			}
		}
	}()
}

func (c *Collector) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hasSnapshot
}

func (c *Collector) Snapshot() (model.DashboardSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.hasSnapshot {
		return model.DashboardSnapshot{}, false
	}

	out := c.snapshot
	if !out.GeneratedAt.IsZero() && time.Since(out.GeneratedAt) > 2*c.pollInterval {
		out.Stale = true
	}
	if !out.SourceOnline {
		out.Stale = true
	}

	return out, true
}

func (c *Collector) refresh(ctx context.Context) {
	now := time.Now().UTC()
	snapshot, err := c.collect(ctx, now)
	if err == nil {
		snapshot.GeneratedAt = now
		snapshot.SourceOnline = true
		snapshot.SourceError = nil
		snapshot.Stale = false

		c.mu.Lock()
		c.snapshot = snapshot
		c.lastGood = snapshot
		c.hasSnapshot = true
		c.hasLastGood = true
		c.mu.Unlock()
		return
	}

	errText := err.Error()
	alert := model.Alert{
		Severity:  "critical",
		Code:      "SOURCE_UNREACHABLE",
		Message:   "Syncthing API is unreachable",
		SubjectID: "syncthing",
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hasLastGood {
		fallback := c.lastGood
		fallback.SourceOnline = false
		fallback.SourceError = &errText
		fallback.Stale = true
		fallback.Alerts = append([]model.Alert{alert}, fallback.Alerts...)
		c.snapshot = fallback
		c.hasSnapshot = true
		return
	}

	c.snapshot = model.DashboardSnapshot{
		GeneratedAt:  now,
		SourceOnline: false,
		SourceError:  &errText,
		Alerts:       []model.Alert{alert},
		Stale:        true,
	}
	c.hasSnapshot = true
}

func (c *Collector) collect(ctx context.Context, now time.Time) (model.DashboardSnapshot, error) {
	status, err := c.client.GetSystemStatus(ctx)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}
	version, err := c.client.GetSystemVersion(ctx)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}
	connections, err := c.client.GetSystemConnections(ctx)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}
	deviceStats, err := c.client.GetDeviceStats(ctx)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}
	folderStats, err := c.client.GetFolderStats(ctx)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}
	cfg, err := c.client.GetConfig(ctx)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}

	dbStatuses := make(map[string]syncthing.DBStatusResponse, len(cfg.Folders))
	for _, folder := range cfg.Folders {
		dbStatus, dbErr := c.client.GetDBStatus(ctx, folder.ID)
		if dbErr != nil {
			return model.DashboardSnapshot{}, fmt.Errorf("get db status for folder %s: %w", folder.ID, dbErr)
		}
		dbStatuses[folder.ID] = dbStatus
	}

	localDeviceID := status.MyID
	localDeviceName := localDeviceID
	for _, device := range cfg.Devices {
		if device.DeviceID == localDeviceID {
			if strings.TrimSpace(device.Name) != "" {
				localDeviceName = device.Name
			}
			break
		}
	}

	downloadBPS, uploadBPS := c.currentRates(connections.Total, now)

	device := model.DeviceStatus{
		Name:        localDeviceName,
		ID:          localDeviceID,
		Version:     strings.TrimSpace(strings.Join([]string{version.Version, version.OS, version.Arch}, " ")),
		UptimeS:     status.Uptime,
		DownloadBPS: downloadBPS,
		UploadBPS:   uploadBPS,
	}

	folders := make([]model.FolderStatus, 0, len(cfg.Folders))
	for _, folder := range cfg.Folders {
		dbStatus := dbStatuses[folder.ID]
		completion, completionErr := c.client.GetDBCompletion(ctx, folder.ID)
		if completionErr != nil {
			return model.DashboardSnapshot{}, fmt.Errorf("get db completion for folder %s: %w", folder.ID, completionErr)
		}

		state := strings.TrimSpace(dbStatus.State)
		if state == "" {
			state = "unknown"
		}
		if folder.Paused {
			state = "paused"
		}

		needItems := completion.NeedItems
		if needItems < 0 {
			needItems = 0
		}
		needBytes := completion.NeedBytes
		if needBytes < 0 {
			needBytes = 0
		}
		globalBytes := dbStatus.GlobalBytes
		if completion.GlobalBytes > globalBytes {
			globalBytes = completion.GlobalBytes
		}
		var completionPct *float64
		if completion.Completion >= 0 && completion.Completion <= 100 {
			value := completion.Completion
			completionPct = &value
		}

		var lastScan *time.Time
		if fs, ok := folderStats[folder.ID]; ok {
			parsed := parseSyncthingTime(fs.LastScan)
			lastScan = parsed
		}

		label := folder.Label
		if strings.TrimSpace(label) == "" {
			label = folder.ID
		}

		folders = append(folders, model.FolderStatus{
			ID:                folder.ID,
			Label:             label,
			Path:              folder.Path,
			State:             state,
			GlobalFiles:       dbStatus.GlobalFiles,
			LocalFiles:        dbStatus.LocalFiles,
			GlobalBytes:       globalBytes,
			LocalBytes:        dbStatus.LocalBytes,
			NeedItems:         needItems,
			NeedBytes:         needBytes,
			LocalChangesItems: dbStatus.ReceiveOnlyTotalItems,
			CompletionPct:     completionPct,
			LastScanAt:        lastScan,
		})
	}
	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Label < folders[j].Label
	})

	remotes := make([]model.RemoteDeviceStatus, 0, len(cfg.Devices))
	for _, deviceCfg := range cfg.Devices {
		if deviceCfg.DeviceID == localDeviceID {
			continue
		}
		conn := connections.Connections[deviceCfg.DeviceID]
		deviceStat := deviceStats[deviceCfg.DeviceID]

		name := deviceCfg.Name
		if strings.TrimSpace(name) == "" {
			name = deviceCfg.DeviceID
		}

		remotes = append(remotes, model.RemoteDeviceStatus{
			ID:            deviceCfg.DeviceID,
			Name:          name,
			Connected:     conn.Connected,
			Address:       conn.Address,
			LastSeenAt:    parseSyncthingTime(deviceStat.LastSeen),
			InBytesTotal:  conn.InBytesTotal,
			OutBytesTotal: conn.OutBytesTotal,
		})
	}
	sort.Slice(remotes, func(i, j int) bool {
		return remotes[i].Name < remotes[j].Name
	})

	var localFilesTotal int64
	var localDirsTotal int64
	var localBytesTotal int64
	for _, dbStatus := range dbStatuses {
		localFilesTotal += dbStatus.LocalFiles
		localDirsTotal += dbStatus.LocalDirectories
		localBytesTotal += dbStatus.LocalBytes
	}

	listenersOK, listenersTotal := serviceHealthCount(status.ConnectionServiceStatus)
	discoveryOK, discoveryTotal := discoveryHealthCount(status)
	device.LocalFilesTotal = localFilesTotal
	device.LocalDirsTotal = localDirsTotal
	device.LocalBytesTotal = localBytesTotal
	device.ListenersOK = listenersOK
	device.ListenersTotal = listenersTotal
	device.DiscoveryOK = discoveryOK
	device.DiscoveryTotal = discoveryTotal

	alerts := deriveAlerts(remotes, folders)

	return model.DashboardSnapshot{
		GeneratedAt:  now,
		SourceOnline: true,
		SourceError:  nil,
		Device:       device,
		Folders:      folders,
		Remotes:      remotes,
		Alerts:       alerts,
		Stale:        false,
	}, nil
}

func (c *Collector) currentRates(total syncthing.ConnectionTotals, now time.Time) (float64, float64) {
	if total.BitsPerSecondIn > 0 || total.BitsPerSecondOut > 0 {
		return total.BitsPerSecondIn / 8, total.BitsPerSecondOut / 8
	}

	if c.lastRateAt.IsZero() {
		c.lastRateAt = now
		c.lastInTotal = total.InBytesTotal
		c.lastOutTotal = total.OutBytesTotal
		return total.BitsPerSecondIn / 8, total.BitsPerSecondOut / 8
	}

	elapsed := now.Sub(c.lastRateAt).Seconds()
	if elapsed <= 0 {
		return total.BitsPerSecondIn / 8, total.BitsPerSecondOut / 8
	}

	inDelta := total.InBytesTotal - c.lastInTotal
	outDelta := total.OutBytesTotal - c.lastOutTotal
	c.lastRateAt = now
	c.lastInTotal = total.InBytesTotal
	c.lastOutTotal = total.OutBytesTotal

	if inDelta < 0 || outDelta < 0 {
		return total.BitsPerSecondIn / 8, total.BitsPerSecondOut / 8
	}

	return float64(inDelta) / elapsed, float64(outDelta) / elapsed
}

func serviceHealthCount(statusByKey map[string]syncthing.ServiceStatus) (int, int) {
	total := len(statusByKey)
	if total == 0 {
		return 0, 0
	}

	ok := 0
	for _, status := range statusByKey {
		if status.Error == nil || strings.TrimSpace(*status.Error) == "" {
			ok++
		}
	}
	return ok, total
}

func discoveryHealthCount(status syncthing.SystemStatusResponse) (int, int) {
	ok, total := serviceHealthCount(status.DiscoveryStatus)
	if total > 0 {
		return ok, total
	}

	if status.DiscoveryMethods <= 0 {
		return 0, 0
	}

	errorCount := len(status.DiscoveryErrors)
	if errorCount > status.DiscoveryMethods {
		errorCount = status.DiscoveryMethods
	}
	return status.DiscoveryMethods - errorCount, status.DiscoveryMethods
}

func deriveAlerts(remotes []model.RemoteDeviceStatus, folders []model.FolderStatus) []model.Alert {
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
		if folder.State == "error" {
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

func parseSyncthingTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z0700",
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}

	return nil
}
