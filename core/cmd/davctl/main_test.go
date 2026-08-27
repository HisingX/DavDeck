package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/client"
	"davdeck.dev/davdeck/core/internal/diagnostics"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/platform"
	"davdeck.dev/davdeck/core/internal/status"
)

type fakeStatusClient struct {
	snapshot       status.Snapshot
	err            error
	users          []client.User
	created        client.User
	password       string
	deleted        domain.ID
	enabled        *bool
	shares         []client.Share
	createdShare   client.Share
	shareUpdate    client.ShareUpdate
	deletedShare   domain.ID
	permissions    []client.PermissionEntry
	setPermission  domain.Permission
	revision       client.Revision
	validation     client.ConfigValidation
	revisionState  client.RevisionState
	revisions      []client.Revision
	restored       domain.ID
	logPage        client.LogPage
	logQuery       client.LogQuery
	tlsProfile     *domain.TLSProfile
	tlsUpdate      client.TLSUpdate
	tlsCheck       client.TLSCheckResult
	diagnostics    diagnostics.Report
	exportedConfig string
	importedConfig []byte
	importResult   client.ConfigImportResult
	serverState    client.ServerStatus
	serverSettings client.ServerSettings
	serviceState   client.ServiceStatus
	serviceCalls   []string
}

func (f fakeStatusClient) Status(context.Context) (status.Snapshot, error) {
	return f.snapshot, f.err
}
func (f *fakeStatusClient) ServerStatus(context.Context) (client.ServerStatus, error) {
	return f.serverState, f.err
}
func (f *fakeStatusClient) StartServer(context.Context) error   { return f.err }
func (f *fakeStatusClient) StopServer(context.Context) error    { return f.err }
func (f *fakeStatusClient) RestartServer(context.Context) error { return f.err }
func (f *fakeStatusClient) ServerSettings(context.Context) (client.ServerSettings, error) {
	return f.serverSettings, f.err
}
func (f *fakeStatusClient) UpdateServerPorts(_ context.Context, httpPort, httpsPort int) (client.ServerSettings, error) {
	f.serverSettings.HTTPPort, f.serverSettings.HTTPSPort = httpPort, httpsPort
	return f.serverSettings, f.err
}
func (f *fakeStatusClient) ServiceInstall(context.Context) error {
	f.serviceCalls = append(f.serviceCalls, "install")
	return f.err
}
func (f *fakeStatusClient) ServiceUninstall(context.Context) error {
	f.serviceCalls = append(f.serviceCalls, "uninstall")
	return f.err
}
func (f *fakeStatusClient) ServiceStart(context.Context) error {
	f.serviceCalls = append(f.serviceCalls, "start")
	return f.err
}
func (f *fakeStatusClient) ServiceStop(context.Context) error {
	f.serviceCalls = append(f.serviceCalls, "stop")
	return f.err
}
func (f *fakeStatusClient) ServiceStatus(context.Context) (client.ServiceStatus, error) {
	f.serviceCalls = append(f.serviceCalls, "status")
	return f.serviceState, f.err
}

func (f *fakeStatusClient) ListUsers(context.Context) ([]client.User, error) { return f.users, f.err }
func (f *fakeStatusClient) CreateUser(_ context.Context, username, password string) (client.User, error) {
	f.password = password
	if f.created.ID == "" {
		f.created = client.User{ID: "11111111-1111-4111-8111-111111111111", Username: username, Enabled: true}
	}
	return f.created, f.err
}
func (f *fakeStatusClient) DeleteUser(_ context.Context, id domain.ID) error {
	f.deleted = id
	return f.err
}
func (f *fakeStatusClient) SetUserEnabled(_ context.Context, _ domain.ID, enabled bool) (client.User, error) {
	f.enabled = &enabled
	user := f.users[0]
	user.Enabled = enabled
	return user, f.err
}
func (f *fakeStatusClient) ChangeUserPassword(_ context.Context, _ domain.ID, password string) error {
	f.password = password
	return f.err
}
func (f *fakeStatusClient) ListShares(context.Context) ([]client.Share, error) {
	return f.shares, f.err
}
func (f *fakeStatusClient) CreateShare(_ context.Context, name, slug, path string) (client.Share, error) {
	if f.createdShare.ID == "" {
		f.createdShare = client.Share{ID: "22222222-2222-4222-8222-222222222222", Name: name, Slug: slug, Path: path, Enabled: true}
	}
	return f.createdShare, f.err
}
func (f *fakeStatusClient) UpdateShare(_ context.Context, _ domain.ID, update client.ShareUpdate) (client.Share, error) {
	f.shareUpdate = update
	share := f.shares[0]
	if update.Name != nil {
		share.Name = *update.Name
	}
	if update.Slug != nil {
		share.Slug = *update.Slug
	}
	if update.Path != nil {
		share.Path = *update.Path
	}
	if update.Enabled != nil {
		share.Enabled = *update.Enabled
	}
	return share, f.err
}
func (f *fakeStatusClient) DeleteShare(_ context.Context, id domain.ID) error {
	f.deletedShare = id
	return f.err
}
func (f *fakeStatusClient) ListPermissions(context.Context, domain.ID) ([]client.PermissionEntry, error) {
	return f.permissions, f.err
}
func (f *fakeStatusClient) SetPermission(_ context.Context, shareID, userID domain.ID, permission domain.Permission) (client.PermissionEntry, error) {
	f.setPermission = permission
	return client.PermissionEntry{ShareID: shareID, UserID: userID, Username: "Alice", Permission: permission}, f.err
}
func (f *fakeStatusClient) ApplyConfig(context.Context) (client.Revision, error) {
	return f.revision, f.err
}
func (f *fakeStatusClient) ValidateConfig(context.Context) (client.ConfigValidation, error) {
	return f.validation, f.err
}
func (f *fakeStatusClient) ConfigState(context.Context) (client.RevisionState, error) {
	return f.revisionState, f.err
}
func (f *fakeStatusClient) ListRevisions(context.Context) ([]client.Revision, error) {
	return f.revisions, f.err
}
func (f *fakeStatusClient) RestoreRevision(_ context.Context, id domain.ID) (client.Revision, error) {
	f.restored = id
	return f.revision, f.err
}
func (f *fakeStatusClient) DeleteRevision(_ context.Context, id domain.ID) error {
	f.restored = id
	return f.err
}
func (f *fakeStatusClient) Logs(_ context.Context, query client.LogQuery) (client.LogPage, error) {
	f.logQuery = query
	return f.logPage, f.err
}
func (f *fakeStatusClient) GetTLS(context.Context) (*domain.TLSProfile, error) {
	return f.tlsProfile, f.err
}
func (f *fakeStatusClient) UpdateTLS(_ context.Context, update client.TLSUpdate) (domain.TLSProfile, error) {
	f.tlsUpdate = update
	return domain.TLSProfile{Mode: update.Mode, Hostname: update.Hostname, CertificatePath: update.CertificatePath, PrivateKeyPath: update.PrivateKeyPath}, f.err
}
func (f *fakeStatusClient) CheckTLS(context.Context) (client.TLSCheckResult, error) {
	return f.tlsCheck, f.err
}
func (f *fakeStatusClient) RunDiagnostics(context.Context) (diagnostics.Report, error) {
	return f.diagnostics, f.err
}
func (f *fakeStatusClient) ExportConfig(context.Context) (string, error) {
	return f.exportedConfig, f.err
}
func (f *fakeStatusClient) ImportConfig(_ context.Context, body []byte) (client.ConfigImportResult, error) {
	f.importedConfig = append([]byte(nil), body...)
	return f.importResult, f.err
}

func testDependencies(apiClient managementClient) (dependencies, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	paths := platform.Paths{ConfigDir: "/config", RuntimeDir: "/runtime"}
	return dependencies{
		stdin:  strings.NewReader(""),
		stdout: stdout,
		stderr: stderr,
		paths:  paths,
		readFile: func(path string) ([]byte, error) {
			switch path {
			case paths.EndpointPath():
				return []byte("http://127.0.0.1:8080\n"), nil
			case paths.TokenPath():
				return []byte("token\n"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		writeFile:    func(string, []byte) error { return nil },
		readPassword: func(string) ([]byte, error) { return []byte("interactive password"), nil },
		newClient: func(endpoint, token string) (managementClient, error) {
			if endpoint != "http://127.0.0.1:8080" || token != "token" {
				return nil, errors.New("unexpected discovery values")
			}
			return apiClient, nil
		},
	}, stdout, stderr
}

func TestStatusHumanAndJSONOutput(t *testing.T) {
	t.Parallel()
	snapshot := status.Snapshot{
		Name: "DavDeck", Version: "test", Daemon: status.DaemonRunning, Database: status.DatabaseReady, SchemaVersion: 3,
		Caddy: "RUNNING", WebDAV: "RUNNING", Service: status.ServiceStatus{Installed: true, State: "RUNNING", StartsAtBoot: true},
	}
	for _, testCase := range []struct {
		name string
		args []string
		want string
	}{
		{"human", []string{"status"}, "Caddy:    RUNNING"},
		{"json after command", []string{"status", "--json"}, `"caddy":"RUNNING"`},
		{"json before command", []string{"--json", "status"}, `"service":{"installed":true`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			deps, stdout, stderr := testDependencies(&fakeStatusClient{snapshot: snapshot})
			if code := run(testCase.args, deps); code != exitSuccess {
				t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), testCase.want) || stderr.Len() != 0 {
				t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestVersionDoesNotRequireDaemonDiscovery(t *testing.T) {
	deps, stdout, stderr := testDependencies(&fakeStatusClient{})
	deps.readFile = func(string) ([]byte, error) { return nil, errors.New("must not be called") }
	if code := run([]string{"version"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "DavDeck dev") || !strings.Contains(stdout.String(), "Target:") {
		t.Fatalf("exit output = %q, stderr = %q", stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(&fakeStatusClient{})
	deps.readFile = func(string) ([]byte, error) { return nil, errors.New("must not be called") }
	if code := run([]string{"--json", "version"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), `"product":"DavDeck"`) || !strings.Contains(stdout.String(), `"target_os":`) {
		t.Fatalf("json output = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestStatusJSONErrorAndExitCode(t *testing.T) {
	t.Parallel()
	deps, stdout, stderr := testDependencies(&fakeStatusClient{err: &client.APIError{StatusCode: 401, Code: "UNAUTHORIZED", Message: "Authentication required"}})
	if code := run([]string{"status", "--json"}, deps); code != exitConnection {
		t.Fatalf("exit code = %d", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestServiceCommandsSupportLifecycleAndJSON(t *testing.T) {
	clientValue := &fakeStatusClient{serviceState: client.ServiceStatus{Installed: true, State: "RUNNING"}}
	deps, stdout, stderr := testDependencies(clientValue)
	if code := run([]string{"service", "status"}, deps); code != exitSuccess {
		t.Fatalf("status exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Installed: true") || !strings.Contains(stdout.String(), "State: RUNNING") {
		t.Fatalf("status output = %q", stdout.String())
	}
	for _, operation := range []string{"install", "uninstall", "start", "stop"} {
		deps, _, stderr = testDependencies(clientValue)
		if code := run([]string{"service", operation}, deps); code != exitSuccess {
			t.Fatalf("%s exit = %d, stderr = %s", operation, code, stderr.String())
		}
	}
	deps, stdout, stderr = testDependencies(clientValue)
	if code := run([]string{"--json", "service", "status"}, deps); code != exitSuccess {
		t.Fatalf("json status exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"installed":true`) || !strings.Contains(stdout.String(), `"state":"RUNNING"`) {
		t.Fatalf("json status output = %q", stdout.String())
	}
	if strings.Join(clientValue.serviceCalls, ",") != "status,install,uninstall,start,stop,status" {
		t.Fatalf("service calls = %v", clientValue.serviceCalls)
	}
}

func TestServicePrivilegeErrorUsesOperationalExitCode(t *testing.T) {
	deps, _, stderr := testDependencies(&fakeStatusClient{err: &client.APIError{StatusCode: 403, Code: "PRIVILEGE_REQUIRED", Message: "Administrator privileges are required"}})
	if code := run([]string{"service", "install"}, deps); code != exitOperational {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "PRIVILEGE_REQUIRED") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestStatusDiscoveryAndUsageFailures(t *testing.T) {
	t.Parallel()
	deps, _, stderr := testDependencies(&fakeStatusClient{})
	deps.readFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	if code := run([]string{"status"}, deps); code != exitConnection || !strings.Contains(stderr.String(), "DAEMON_DISCOVERY_FAILED") {
		t.Fatalf("exit code or stderr mismatch: %d %q", code, stderr.String())
	}
	deps, _, stderr = testDependencies(&fakeStatusClient{})
	if code := run([]string{"unknown"}, deps); code != exitUsage || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("exit code or stderr mismatch: %d %q", code, stderr.String())
	}
}

func TestUserAddReadsPasswordFromStdinWithoutPrintingIt(t *testing.T) {
	apiClient := &fakeStatusClient{}
	deps, stdout, stderr := testDependencies(apiClient)
	deps.stdin = strings.NewReader("secret password\n")
	if code := run([]string{"user", "add", "Alice", "--password-stdin"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if apiClient.password != "secret password" {
		t.Fatalf("password = %q", apiClient.password)
	}
	if strings.Contains(stdout.String()+stderr.String(), "secret password") {
		t.Fatal("password leaked to output")
	}
}

func TestUserCommandsResolveUsernameAndSupportJSON(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	user := client.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	apiClient := &fakeStatusClient{users: []client.User{user}}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"--json", "user", "disable", "ALICE"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if apiClient.enabled == nil || *apiClient.enabled || !strings.Contains(stdout.String(), `"enabled":false`) {
		t.Fatalf("enabled = %v, stdout = %q", apiClient.enabled, stdout.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"user", "delete", string(user.ID)}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if apiClient.deleted != user.ID || !strings.Contains(stdout.String(), "files were preserved") {
		t.Fatalf("deleted = %q, stdout = %q", apiClient.deleted, stdout.String())
	}
}

func TestUserPasswordInteractiveAndMissingUser(t *testing.T) {
	user := client.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", Enabled: true}
	apiClient := &fakeStatusClient{users: []client.User{user}}
	deps, _, stderr := testDependencies(apiClient)
	if code := run([]string{"user", "passwd", "alice"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if apiClient.password != "interactive password" {
		t.Fatalf("password = %q", apiClient.password)
	}
	apiClient.users = nil
	deps, _, stderr = testDependencies(apiClient)
	if code := run([]string{"user", "enable", "missing"}, deps); code != exitOperational || !strings.Contains(stderr.String(), "USER_NOT_FOUND") {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

func TestShareCommands(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	share := client.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Team Files", Slug: "team-files", Path: "/srv/team", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	apiClient := &fakeStatusClient{shares: []client.Share{share}}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"share", "list"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "team-files") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"share", "add", "Project Files", "/srv/project"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if apiClient.createdShare.Slug != "project-files" {
		t.Fatalf("slug = %q", apiClient.createdShare.Slug)
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"--json", "share", "update", "team-files", "--name", "Team Docs", "--disable"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if apiClient.shareUpdate.Name == nil || *apiClient.shareUpdate.Name != "Team Docs" || apiClient.shareUpdate.Enabled == nil || *apiClient.shareUpdate.Enabled {
		t.Fatalf("update = %#v", apiClient.shareUpdate)
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"share", "remove", "Team Files"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if apiClient.deletedShare != share.ID || !strings.Contains(stdout.String(), "physical files were preserved") {
		t.Fatalf("deleted = %q, stdout = %q", apiClient.deletedShare, stdout.String())
	}
}

func TestSlugify(t *testing.T) {
	for input, want := range map[string]string{"Project Files": "project-files", "  A -- B  ": "a-b", "Docs_2026": "docs-2026"} {
		if got := slugify(input); got != want {
			t.Errorf("slugify(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestACLCommandsAndPermissionParsing(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	share := client.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Team", Slug: "team", Path: "/srv/team", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	user := client.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	apiClient := &fakeStatusClient{shares: []client.Share{share}, users: []client.User{user}, permissions: []client.PermissionEntry{{ShareID: share.ID, UserID: user.ID, Username: user.Username, Permission: domain.PermissionNone}}}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"acl", "list", "team"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "NONE") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"--json", "acl", "set", "team", "alice", "read-write"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if apiClient.setPermission != domain.PermissionReadWrite || !strings.Contains(stdout.String(), "READ_WRITE") {
		t.Fatalf("permission = %q, stdout = %q", apiClient.setPermission, stdout.String())
	}
	deps, _, stderr = testDependencies(apiClient)
	if code := run([]string{"acl", "set", "team", "alice", "owner"}, deps); code != exitUsage || !strings.Contains(stderr.String(), "INVALID_PERMISSION") {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

func TestConfigApplyStatusAndRevisionCommands(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	desired, active := uint64(2), uint64(1)
	revision := client.Revision{ID: "11111111-1111-4111-8111-111111111111", Number: 2, CreatedAt: stamp, ConfigHash: strings.Repeat("a", 64), ValidationStatus: domain.RevisionValidationValid, ApplyStatus: domain.RevisionApplyApplied, AppVersion: "test"}
	apiClient := &fakeStatusClient{revision: revision, revisionState: client.RevisionState{DesiredRevision: &desired, ActiveRevision: &active, Pending: true}, revisions: []client.Revision{revision}}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"config", "apply"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "revision 2") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"config", "status"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "Pending: true") {
		t.Fatalf("exit = %d, stdout = %q", code, stdout.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"--json", "revision", "list"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), `"number":2`) {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	apiClient.err = &client.APIError{StatusCode: 422, Code: "CADDY_VALIDATE_FAILED", Message: "invalid"}
	deps, _, _ = testDependencies(apiClient)
	if code := run([]string{"config", "apply"}, deps); code != exitConfiguration {
		t.Fatalf("exit = %d", code)
	}
}

func TestConfigValidateRevisionRestoreAndLogsCommands(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	revision := client.Revision{ID: "11111111-1111-4111-8111-111111111111", Number: 3, CreatedAt: stamp, ConfigHash: strings.Repeat("b", 64), ValidationStatus: domain.RevisionValidationValid, ApplyStatus: domain.RevisionApplyApplied, AppVersion: "test"}
	apiClient := &fakeStatusClient{
		revision:   revision,
		validation: client.ConfigValidation{Valid: true, ConfigHash: revision.ConfigHash, Warnings: []string{"share has no authorized users"}},
		logPage:    client.LogPage{Records: []client.LogRecord{{ID: 2, Timestamp: time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC), Level: "ERROR", Component: "caddy", Message: "reload failed"}}, HasMore: true, NextCursor: 2},
	}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"config", "validate"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "Configuration is valid") || !strings.Contains(stdout.String(), "Warning:") {
		t.Fatalf("validate exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"revision", "restore", string(revision.ID)}, deps); code != exitSuccess || apiClient.restored != revision.ID || !strings.Contains(stdout.String(), "Restored configuration revision 3") {
		t.Fatalf("restore exit=%d restored=%q stdout=%q stderr=%q", code, apiClient.restored, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"revision", "delete", string(revision.ID)}, deps); code != exitSuccess || apiClient.restored != revision.ID || !strings.Contains(stdout.String(), "Deleted configuration revision") {
		t.Fatalf("delete exit=%d deleted=%q stdout=%q stderr=%q", code, apiClient.restored, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"--json", "logs", "--limit", "10", "--level", "error", "--component", "caddy"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), `"has_more":true`) || apiClient.logQuery.Limit != 10 || apiClient.logQuery.Level != "error" {
		t.Fatalf("logs exit=%d query=%#v stdout=%q stderr=%q", code, apiClient.logQuery, stdout.String(), stderr.String())
	}
	deps, _, stderr = testDependencies(apiClient)
	if code := run([]string{"logs", "--follow"}, deps); code != exitUsage || !strings.Contains(stderr.String(), "LOG_FOLLOW_UNSUPPORTED") {
		t.Fatalf("follow exit=%d stderr=%q", code, stderr.String())
	}
}

func TestConfigExportAndImportCommands(t *testing.T) {
	yaml := "version: 1\nusers: []\nshares: []\n"
	apiClient := &fakeStatusClient{
		exportedConfig: yaml,
		importResult: client.ConfigImportResult{
			UsersCreated:          1,
			SharesUpdated:         1,
			PermissionsUpserted:   2,
			PasswordResetRequired: []string{"Alice"},
			PendingApply:          true,
		},
	}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"config", "export"}, deps); code != exitSuccess || stdout.String() != yaml {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}

	deps, stdout, stderr = testDependencies(apiClient)
	var outputPath string
	var outputBody []byte
	deps.writeFile = func(path string, body []byte) error {
		outputPath = path
		outputBody = append([]byte(nil), body...)
		return nil
	}
	if code := run([]string{"--json", "config", "export", "--output", "/tmp/davdeck.yaml"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if outputPath != "/tmp/davdeck.yaml" || string(outputBody) != yaml || !strings.Contains(stdout.String(), `"contains_secrets":false`) {
		t.Fatalf("path = %q, body = %q, stdout = %q", outputPath, outputBody, stdout.String())
	}

	deps, stdout, stderr = testDependencies(apiClient)
	originalRead := deps.readFile
	deps.readFile = func(path string) ([]byte, error) {
		if path == "/tmp/import.yaml" {
			return []byte(yaml), nil
		}
		return originalRead(path)
	}
	if code := run([]string{"config", "import", "/tmp/import.yaml"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if string(apiClient.importedConfig) != yaml || !strings.Contains(stdout.String(), "Set passwords separately for: Alice") || !strings.Contains(stdout.String(), "config apply") {
		t.Fatalf("imported = %q, stdout = %q", apiClient.importedConfig, stdout.String())
	}
}

func TestConfigImportFailureUsesConfigurationExitCode(t *testing.T) {
	apiClient := &fakeStatusClient{err: &client.APIError{StatusCode: 422, Code: "CONFIG_IMPORT_INVALID", Message: "invalid configuration"}}
	deps, _, stderr := testDependencies(apiClient)
	baseRead := deps.readFile
	deps.readFile = func(path string) ([]byte, error) {
		if path == "bad.yaml" {
			return []byte("version: 1\nunknown: true\n"), nil
		}
		return baseRead(path)
	}
	if code := run([]string{"config", "import", "bad.yaml"}, deps); code != exitConfiguration || !strings.Contains(stderr.String(), "CONFIG_IMPORT_INVALID") {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

func TestTLSCommandsConfigureShowAndCheck(t *testing.T) {
	apiClient := &fakeStatusClient{tlsCheck: client.TLSCheckResult{Ready: true, Checks: []client.TLSCheck{{Name: "trust", OK: true, Message: "Internal CA trust is required"}}}}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"tls", "internal", "dav.local"}, deps); code != exitSuccess {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if apiClient.tlsUpdate.Mode != domain.TLSModeInternal || apiClient.tlsUpdate.Hostname != "dav.local" || !strings.Contains(stdout.String(), "Apply") {
		t.Fatalf("update = %#v, stdout = %q", apiClient.tlsUpdate, stdout.String())
	}
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	apiClient.tlsProfile = &domain.TLSProfile{ID: "11111111-1111-4111-8111-111111111111", Mode: domain.TLSModeCustom, Hostname: "dav.example.com", CertificatePath: "/cert.pem", PrivateKeyPath: "/key.pem", CreatedAt: stamp, UpdatedAt: stamp}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"--json", "tls", "show"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), `"private_key_path":"/key.pem"`) {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"tls", "check"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "Internal CA trust") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	deps, _, stderr = testDependencies(apiClient)
	if code := run([]string{"tls", "custom", "--hostname", "dav.local", "--cert", "/cert.pem"}, deps); code != exitUsage {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

func TestDoctorHumanJSONAndFailureExit(t *testing.T) {
	report := diagnostics.Report{SchemaVersion: 1, Sanitized: true, Overall: diagnostics.StatusWarn, Results: []diagnostics.Result{{ID: "tls", Title: "TLS", Status: diagnostics.StatusWarn, Code: "TLS_CONFIGURATION_ERROR", Message: "TLS is not configured"}}}
	apiClient := &fakeStatusClient{diagnostics: report}
	deps, stdout, stderr := testDependencies(apiClient)
	if code := run([]string{"doctor"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), "DavDeck diagnostics: WARN") || !strings.Contains(stdout.String(), "TLS_CONFIGURATION_ERROR") {
		t.Fatalf("exit output = %q, stderr = %q", stdout.String(), stderr.String())
	}
	deps, stdout, stderr = testDependencies(apiClient)
	if code := run([]string{"--json", "doctor"}, deps); code != exitSuccess || !strings.Contains(stdout.String(), `"sanitized":true`) {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
	report.Overall = diagnostics.StatusFail
	apiClient.diagnostics = report
	deps, _, _ = testDependencies(apiClient)
	if code := run([]string{"doctor"}, deps); code != exitOperational {
		t.Fatalf("failure exit = %d", code)
	}
}
