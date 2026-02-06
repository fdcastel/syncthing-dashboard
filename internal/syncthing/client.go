package syncthing

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var allowedReadPaths = map[string]struct{}{
	"/rest/system/status":      {},
	"/rest/system/version":     {},
	"/rest/system/connections": {},
	"/rest/stats/device":       {},
	"/rest/stats/folder":       {},
	"/rest/config":             {},
	"/rest/db/status":          {},
	"/rest/db/completion":      {},
}

// Client is a strict read-only Syncthing API client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string, timeout time.Duration, insecureSkipVerify bool) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

func (c *Client) GetSystemStatus(ctx context.Context) (SystemStatusResponse, error) {
	var out SystemStatusResponse
	if err := c.getJSON(ctx, "/rest/system/status", nil, &out); err != nil {
		return SystemStatusResponse{}, err
	}
	return out, nil
}

func (c *Client) GetSystemVersion(ctx context.Context) (SystemVersionResponse, error) {
	var out SystemVersionResponse
	if err := c.getJSON(ctx, "/rest/system/version", nil, &out); err != nil {
		return SystemVersionResponse{}, err
	}
	return out, nil
}

func (c *Client) GetSystemConnections(ctx context.Context) (SystemConnectionsResponse, error) {
	var out SystemConnectionsResponse
	if err := c.getJSON(ctx, "/rest/system/connections", nil, &out); err != nil {
		return SystemConnectionsResponse{}, err
	}
	return out, nil
}

func (c *Client) GetDeviceStats(ctx context.Context) (map[string]DeviceStats, error) {
	var out map[string]DeviceStats
	if err := c.getJSON(ctx, "/rest/stats/device", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetFolderStats(ctx context.Context) (map[string]FolderStats, error) {
	var out map[string]FolderStats
	if err := c.getJSON(ctx, "/rest/stats/folder", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetConfig(ctx context.Context) (ConfigResponse, error) {
	var out ConfigResponse
	if err := c.getJSON(ctx, "/rest/config", nil, &out); err != nil {
		return ConfigResponse{}, err
	}
	return out, nil
}

func (c *Client) GetDBStatus(ctx context.Context, folderID string) (DBStatusResponse, error) {
	var out DBStatusResponse
	query := url.Values{}
	query.Set("folder", folderID)
	if err := c.getJSON(ctx, "/rest/db/status", query, &out); err != nil {
		return DBStatusResponse{}, err
	}
	return out, nil
}

func (c *Client) GetDBCompletion(ctx context.Context, folderID string) (DBCompletionResponse, error) {
	var out DBCompletionResponse
	query := url.Values{}
	query.Set("folder", folderID)
	if err := c.getJSON(ctx, "/rest/db/completion", query, &out); err != nil {
		return DBCompletionResponse{}, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	if _, ok := allowedReadPaths[path]; !ok {
		return fmt.Errorf("path %q is not allowed in read-only mode", path)
	}

	endpoint := c.baseURL + path
	if query != nil {
		endpoint = endpoint + "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("request %s failed with status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response %s: %w", path, err)
	}

	return nil
}

type SystemStatusResponse struct {
	MyID                    string                   `json:"myID"`
	Uptime                  int64                    `json:"uptime"`
	ConnectionServiceStatus map[string]ServiceStatus `json:"connectionServiceStatus"`
	DiscoveryStatus         map[string]ServiceStatus `json:"discoveryStatus"`
	DiscoveryMethods        int                      `json:"discoveryMethods"`
	DiscoveryErrors         map[string]string        `json:"discoveryErrors"`
}

type ServiceStatus struct {
	Error *string `json:"error"`
}

type SystemVersionResponse struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

type SystemConnectionsResponse struct {
	Total       ConnectionTotals             `json:"total"`
	Connections map[string]ConnectionDetails `json:"connections"`
}

type ConnectionTotals struct {
	InBytesTotal     int64   `json:"inBytesTotal"`
	OutBytesTotal    int64   `json:"outBytesTotal"`
	BitsPerSecondIn  float64 `json:"bitsPerSecondIn"`
	BitsPerSecondOut float64 `json:"bitsPerSecondOut"`
}

type ConnectionDetails struct {
	Address       string `json:"address"`
	Connected     bool   `json:"connected"`
	InBytesTotal  int64  `json:"inBytesTotal"`
	OutBytesTotal int64  `json:"outBytesTotal"`
}

type DeviceStats struct {
	LastSeen string `json:"lastSeen"`
}

type FolderStats struct {
	LastScan string `json:"lastScan"`
}

type ConfigResponse struct {
	Devices []ConfigDevice `json:"devices"`
	Folders []ConfigFolder `json:"folders"`
}

type ConfigDevice struct {
	DeviceID string `json:"deviceID"`
	Name     string `json:"name"`
}

type ConfigFolder struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Paused bool   `json:"paused"`
}

type DBStatusResponse struct {
	GlobalFiles             int64  `json:"globalFiles"`
	LocalFiles              int64  `json:"localFiles"`
	LocalDirectories        int64  `json:"localDirectories"`
	GlobalBytes             int64  `json:"globalBytes"`
	LocalBytes              int64  `json:"localBytes"`
	NeedFiles               int64  `json:"needFiles"`
	NeedDirectories         int64  `json:"needDirectories"`
	NeedSymlinks            int64  `json:"needSymlinks"`
	NeedDeletes             int64  `json:"needDeletes"`
	NeedBytes               int64  `json:"needBytes"`
	NeedTotalItems          int64  `json:"needTotalItems"`
	ReceiveOnlyTotalItems   int64  `json:"receiveOnlyTotalItems"`
	ReceiveOnlyChangedBytes int64  `json:"receiveOnlyChangedBytes"`
	State                   string `json:"state"`
}

type DBCompletionResponse struct {
	Completion  float64 `json:"completion"`
	NeedBytes   int64   `json:"needBytes"`
	NeedItems   int64   `json:"needItems"`
	GlobalBytes int64   `json:"globalBytes"`
}
