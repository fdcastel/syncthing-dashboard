package model

import "time"

// DashboardSnapshot is the API payload returned to dashboard clients.
type DashboardSnapshot struct {
	GeneratedAt  time.Time            `json:"generated_at"`
	SourceOnline bool                 `json:"source_online"`
	SourceError  *string              `json:"source_error"`
	Device       DeviceStatus         `json:"device"`
	Folders      []FolderStatus       `json:"folders"`
	Remotes      []RemoteDeviceStatus `json:"remotes"`
	Alerts       []Alert              `json:"alerts"`
	Stale        bool                 `json:"stale"`
}

type DeviceStatus struct {
	Name            string  `json:"name"`
	ID              string  `json:"id"`
	Version         string  `json:"version"`
	UptimeS         int64   `json:"uptime_s"`
	DownloadBPS     float64 `json:"download_bps"`
	UploadBPS       float64 `json:"upload_bps"`
	LocalFilesTotal int64   `json:"local_files_total"`
	LocalDirsTotal  int64   `json:"local_dirs_total"`
	LocalBytesTotal int64   `json:"local_bytes_total"`
	ListenersOK     int     `json:"listeners_ok"`
	ListenersTotal  int     `json:"listeners_total"`
	DiscoveryOK     int     `json:"discovery_ok"`
	DiscoveryTotal  int     `json:"discovery_total"`
}

type FolderStatus struct {
	ID                string     `json:"id"`
	Label             string     `json:"label"`
	Path              string     `json:"path"`
	State             string     `json:"state"`
	GlobalFiles       int64      `json:"global_files"`
	LocalFiles        int64      `json:"local_files"`
	GlobalBytes       int64      `json:"global_bytes"`
	LocalBytes        int64      `json:"local_bytes"`
	NeedItems         int64      `json:"need_items"`
	NeedBytes         int64      `json:"need_bytes"`
	LocalChangesItems int64      `json:"local_changes_items"`
	CompletionPct     *float64   `json:"completion_pct"`
	LastScanAt        *time.Time `json:"last_scan_at"`
}

type RemoteDeviceStatus struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Connected     bool       `json:"connected"`
	Address       string     `json:"address"`
	LastSeenAt    *time.Time `json:"last_seen_at"`
	InBytesTotal  int64      `json:"in_bytes_total"`
	OutBytesTotal int64      `json:"out_bytes_total"`
}

type Alert struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	SubjectID string `json:"subject_id"`
}
