// Package client implements the shared davctl Management API client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"davdeck.dev/davdeck/core/internal/diagnostics"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

// User is the secret-free user representation returned by davd.
type User struct {
	ID        domain.ID        `json:"id"`
	Username  string           `json:"username"`
	Enabled   bool             `json:"enabled"`
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

// Share is the filesystem metadata representation returned by davd.
type Share struct {
	ID        domain.ID        `json:"id"`
	Name      string           `json:"name"`
	Slug      string           `json:"slug"`
	Path      string           `json:"path"`
	Enabled   bool             `json:"enabled"`
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

type ShareUpdate struct {
	Name    *string `json:"name,omitempty"`
	Slug    *string `json:"slug,omitempty"`
	Path    *string `json:"path,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

type PermissionEntry struct {
	ShareID    domain.ID         `json:"share_id"`
	UserID     domain.ID         `json:"user_id"`
	Username   string            `json:"username"`
	Permission domain.Permission `json:"permission"`
}

type TLSUpdate struct {
	Mode            domain.TLSMode `json:"mode"`
	Hostname        string         `json:"hostname"`
	CertificatePath string         `json:"certificate_path,omitempty"`
	PrivateKeyPath  string         `json:"private_key_path,omitempty"`
}

type TLSCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type TLSCheckResult struct {
	Ready  bool       `json:"ready"`
	Checks []TLSCheck `json:"checks"`
}

type Revision struct {
	ID               domain.ID                       `json:"id"`
	Number           uint64                          `json:"number"`
	CreatedAt        domain.Timestamp                `json:"created_at"`
	ConfigHash       string                          `json:"config_hash"`
	ValidationStatus domain.RevisionValidationStatus `json:"validation_status"`
	ApplyStatus      domain.RevisionApplyStatus      `json:"apply_status"`
	AppVersion       string                          `json:"app_version"`
	ErrorCode        string                          `json:"error_code,omitempty"`
	ErrorSummary     string                          `json:"error_summary,omitempty"`
}

type RevisionState struct {
	DesiredRevision *uint64 `json:"desired_revision"`
	ActiveRevision  *uint64 `json:"active_revision"`
	Dirty           bool    `json:"dirty"`
	Pending         bool    `json:"pending"`
}

type ConfigValidation struct {
	Valid      bool     `json:"valid"`
	ConfigHash string   `json:"config_hash"`
	Warnings   []string `json:"warnings,omitempty"`
}

type LogRecord struct {
	ID        uint64         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

type LogPage struct {
	Records    []LogRecord `json:"records"`
	NextCursor uint64      `json:"next_cursor,omitempty"`
	HasMore    bool        `json:"has_more"`
}

type LogQuery struct {
	Limit     int
	Cursor    uint64
	Since     *time.Time
	Level     string
	Component string
}

type ServerStatus struct {
	Caddy string `json:"caddy"`
}

type ServerSettings struct {
	HTTPPort  int `json:"http_port"`
	HTTPSPort int `json:"https_port"`
}

type ServiceStatus struct {
	Installed bool   `json:"installed"`
	State     string `json:"state"`
}

type ConfigImportResult struct {
	UsersCreated          int      `json:"users_created"`
	UsersUpdated          int      `json:"users_updated"`
	SharesCreated         int      `json:"shares_created"`
	SharesUpdated         int      `json:"shares_updated"`
	PermissionsUpserted   int      `json:"permissions_upserted"`
	TLSUpdated            bool     `json:"tls_updated"`
	ServerUpdated         bool     `json:"server_updated"`
	PasswordResetRequired []string `json:"password_reset_required"`
	PendingApply          bool     `json:"pending_apply"`
}

// APIError is a typed safe error returned by davd.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    map[string]any
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Client talks only to davd's loopback Management API.
type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

// New validates the endpoint before creating a client.
func New(endpoint, token string) (*Client, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("management endpoint must be loopback HTTP")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() || parsed.Port() == "" {
		return nil, fmt.Errorf("management endpoint must be loopback HTTP")
	}
	if token == "" {
		return nil, fmt.Errorf("management token is empty")
	}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), token: token, http: &http.Client{Timeout: 10 * time.Second}}, nil
}

// Status fetches the daemon status snapshot.
func (c *Client) Status(ctx context.Context) (status.Snapshot, error) {
	var snapshot status.Snapshot
	if err := c.do(ctx, http.MethodGet, "/api/v1/status", nil, &snapshot); err != nil {
		return status.Snapshot{}, err
	}
	return snapshot, nil
}

func (c *Client) ServerStatus(ctx context.Context) (ServerStatus, error) {
	var result ServerStatus
	err := c.do(ctx, http.MethodGet, "/api/v1/server/status", nil, &result)
	return result, err
}

func (c *Client) StartServer(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/server/start", nil, nil)
}
func (c *Client) StopServer(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/server/stop", nil, nil)
}
func (c *Client) RestartServer(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/server/restart", nil, nil)
}

func (c *Client) ServerSettings(ctx context.Context) (ServerSettings, error) {
	var result ServerSettings
	err := c.do(ctx, http.MethodGet, "/api/v1/server/settings", nil, &result)
	return result, err
}

func (c *Client) UpdateServerPorts(ctx context.Context, httpPort, httpsPort int) (ServerSettings, error) {
	var result ServerSettings
	err := c.do(ctx, http.MethodPut, "/api/v1/server/settings", ServerSettings{HTTPPort: httpPort, HTTPSPort: httpsPort}, &result)
	return result, err
}

func (c *Client) ServiceInstall(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/service/install", nil, nil)
}

func (c *Client) ServiceUninstall(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/service/uninstall", nil, nil)
}

func (c *Client) ServiceStart(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/service/start", nil, nil)
}

func (c *Client) ServiceStop(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v1/service/stop", nil, nil)
}

func (c *Client) ServiceStatus(ctx context.Context) (ServiceStatus, error) {
	var result ServiceStatus
	err := c.do(ctx, http.MethodGet, "/api/v1/service/status", nil, &result)
	return result, err
}

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := c.do(ctx, http.MethodGet, "/api/v1/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) CreateUser(ctx context.Context, username, password string) (User, error) {
	var user User
	err := c.do(ctx, http.MethodPost, "/api/v1/users", map[string]string{"username": username, "password": password}, &user)
	return user, err
}

func (c *Client) DeleteUser(ctx context.Context, id domain.ID) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/users/"+string(id), nil, nil)
}

func (c *Client) SetUserEnabled(ctx context.Context, id domain.ID, enabled bool) (User, error) {
	var user User
	err := c.do(ctx, http.MethodPatch, "/api/v1/users/"+string(id), map[string]bool{"enabled": enabled}, &user)
	return user, err
}

func (c *Client) ChangeUserPassword(ctx context.Context, id domain.ID, password string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/users/"+string(id)+"/password", map[string]string{"password": password}, nil)
}

func (c *Client) ListShares(ctx context.Context) ([]Share, error) {
	var shares []Share
	if err := c.do(ctx, http.MethodGet, "/api/v1/shares", nil, &shares); err != nil {
		return nil, err
	}
	return shares, nil
}

func (c *Client) CreateShare(ctx context.Context, name, slug, path string) (Share, error) {
	var share Share
	err := c.do(ctx, http.MethodPost, "/api/v1/shares", map[string]string{"name": name, "slug": slug, "path": path}, &share)
	return share, err
}

func (c *Client) UpdateShare(ctx context.Context, id domain.ID, update ShareUpdate) (Share, error) {
	var share Share
	err := c.do(ctx, http.MethodPatch, "/api/v1/shares/"+string(id), update, &share)
	return share, err
}

func (c *Client) DeleteShare(ctx context.Context, id domain.ID) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/shares/"+string(id), nil, nil)
}

func (c *Client) ListPermissions(ctx context.Context, shareID domain.ID) ([]PermissionEntry, error) {
	var entries []PermissionEntry
	if err := c.do(ctx, http.MethodGet, "/api/v1/shares/"+string(shareID)+"/permissions", nil, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Client) SetPermission(ctx context.Context, shareID, userID domain.ID, permission domain.Permission) (PermissionEntry, error) {
	var entry PermissionEntry
	err := c.do(ctx, http.MethodPut, "/api/v1/shares/"+string(shareID)+"/permissions/"+string(userID), map[string]domain.Permission{"permission": permission}, &entry)
	return entry, err
}

func (c *Client) GetTLS(ctx context.Context) (*domain.TLSProfile, error) {
	var profile *domain.TLSProfile
	if err := c.do(ctx, http.MethodGet, "/api/v1/tls", nil, &profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (c *Client) UpdateTLS(ctx context.Context, update TLSUpdate) (domain.TLSProfile, error) {
	var profile domain.TLSProfile
	err := c.do(ctx, http.MethodPut, "/api/v1/tls", update, &profile)
	return profile, err
}

func (c *Client) CheckTLS(ctx context.Context) (TLSCheckResult, error) {
	var result TLSCheckResult
	err := c.do(ctx, http.MethodPost, "/api/v1/tls/check", nil, &result)
	return result, err
}

func (c *Client) RunDiagnostics(ctx context.Context) (diagnostics.Report, error) {
	var report diagnostics.Report
	err := c.do(ctx, http.MethodPost, "/api/v1/diagnostics/run", nil, &report)
	return report, err
}

func (c *Client) ExportConfig(ctx context.Context) (string, error) {
	var result struct {
		Format          string `json:"format"`
		Content         string `json:"content"`
		ContainsSecrets bool   `json:"contains_secrets"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/config/export", nil, &result); err != nil {
		return "", err
	}
	if result.Format != "yaml" || result.ContainsSecrets {
		return "", fmt.Errorf("davd returned an unsafe configuration export")
	}
	return result.Content, nil
}

func (c *Client) ImportConfig(ctx context.Context, body []byte) (ConfigImportResult, error) {
	var result ConfigImportResult
	err := c.doBody(ctx, http.MethodPost, "/api/v1/config/import", body, "application/yaml", &result)
	return result, err
}

func (c *Client) ApplyConfig(ctx context.Context) (Revision, error) {
	var revision Revision
	err := c.do(ctx, http.MethodPost, "/api/v1/config/apply", nil, &revision)
	return revision, err
}

func (c *Client) ValidateConfig(ctx context.Context) (ConfigValidation, error) {
	var result ConfigValidation
	err := c.do(ctx, http.MethodPost, "/api/v1/config/validate", nil, &result)
	return result, err
}

func (c *Client) ConfigState(ctx context.Context) (RevisionState, error) {
	var state RevisionState
	err := c.do(ctx, http.MethodGet, "/api/v1/config/state", nil, &state)
	return state, err
}
func (c *Client) ListRevisions(ctx context.Context) ([]Revision, error) {
	var revisions []Revision
	if err := c.do(ctx, http.MethodGet, "/api/v1/revisions", nil, &revisions); err != nil {
		return nil, err
	}
	return revisions, nil
}

func (c *Client) RestoreRevision(ctx context.Context, id domain.ID) (Revision, error) {
	var result Revision
	err := c.do(ctx, http.MethodPost, "/api/v1/revisions/"+string(id)+"/restore", nil, &result)
	return result, err
}

func (c *Client) DeleteRevision(ctx context.Context, id domain.ID) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/revisions/"+string(id), nil, nil)
}

func (c *Client) Logs(ctx context.Context, query LogQuery) (LogPage, error) {
	values := url.Values{}
	if query.Limit != 0 {
		values.Set("limit", fmt.Sprint(query.Limit))
	}
	if query.Cursor != 0 {
		values.Set("cursor", fmt.Sprint(query.Cursor))
	}
	if query.Since != nil {
		values.Set("since", query.Since.UTC().Format(time.RFC3339Nano))
	}
	if query.Level != "" {
		values.Set("level", query.Level)
	}
	if query.Component != "" {
		values.Set("component", query.Component)
	}
	path := "/api/v1/logs"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var result LogPage
	err := c.do(ctx, http.MethodGet, path, nil, &result)
	return result, err
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	var body []byte
	contentType := ""
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode davd request: %w", err)
		}
		body, contentType = encoded, "application/json"
	}
	return c.doBody(ctx, method, path, body, contentType, output)
}

func (c *Client) doBody(ctx context.Context, method, path string, body []byte, contentType string, output any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reader)
	if err != nil {
		return fmt.Errorf("create davd request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	result, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("connect to davd: %w", err)
	}
	defer result.Body.Close()
	var payload response
	decoder := json.NewDecoder(io.LimitReader(result.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decode davd response: %w", err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 || !payload.Success {
		if payload.Error != nil {
			return &APIError{StatusCode: result.StatusCode, Code: payload.Error.Code, Message: payload.Error.Message, Details: payload.Error.Details}
		}
		return fmt.Errorf("davd returned HTTP %d", result.StatusCode)
	}
	if output != nil {
		if err := json.Unmarshal(payload.Data, output); err != nil {
			return fmt.Errorf("decode davd response data: %w", err)
		}
	}
	return nil
}
