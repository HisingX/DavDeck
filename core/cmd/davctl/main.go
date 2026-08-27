package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"davdeck.dev/davdeck/core/internal/buildinfo"
	"davdeck.dev/davdeck/core/internal/client"
	"davdeck.dev/davdeck/core/internal/diagnostics"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/platform"
	"davdeck.dev/davdeck/core/internal/status"
	"golang.org/x/term"
)

const (
	exitSuccess = iota
	exitOperational
	exitUsage
	exitConnection
	exitConfiguration
)

type managementClient interface {
	Status(context.Context) (status.Snapshot, error)
	ServerStatus(context.Context) (client.ServerStatus, error)
	StartServer(context.Context) error
	StopServer(context.Context) error
	RestartServer(context.Context) error
	ServerSettings(context.Context) (client.ServerSettings, error)
	UpdateServerPorts(context.Context, int, int) (client.ServerSettings, error)
	ServiceInstall(context.Context) error
	ServiceUninstall(context.Context) error
	ServiceStart(context.Context) error
	ServiceStop(context.Context) error
	ServiceStatus(context.Context) (client.ServiceStatus, error)
	ListUsers(context.Context) ([]client.User, error)
	CreateUser(context.Context, string, string) (client.User, error)
	DeleteUser(context.Context, domain.ID) error
	SetUserEnabled(context.Context, domain.ID, bool) (client.User, error)
	ChangeUserPassword(context.Context, domain.ID, string) error
	ListShares(context.Context) ([]client.Share, error)
	CreateShare(context.Context, string, string, string) (client.Share, error)
	UpdateShare(context.Context, domain.ID, client.ShareUpdate) (client.Share, error)
	DeleteShare(context.Context, domain.ID) error
	ListPermissions(context.Context, domain.ID) ([]client.PermissionEntry, error)
	SetPermission(context.Context, domain.ID, domain.ID, domain.Permission) (client.PermissionEntry, error)
	ApplyConfig(context.Context) (client.Revision, error)
	ValidateConfig(context.Context) (client.ConfigValidation, error)
	ConfigState(context.Context) (client.RevisionState, error)
	ListRevisions(context.Context) ([]client.Revision, error)
	RestoreRevision(context.Context, domain.ID) (client.Revision, error)
	DeleteRevision(context.Context, domain.ID) error
	Logs(context.Context, client.LogQuery) (client.LogPage, error)
	GetTLS(context.Context) (*domain.TLSProfile, error)
	UpdateTLS(context.Context, client.TLSUpdate) (domain.TLSProfile, error)
	CheckTLS(context.Context) (client.TLSCheckResult, error)
	RunDiagnostics(context.Context) (diagnostics.Report, error)
	ExportConfig(context.Context) (string, error)
	ImportConfig(context.Context, []byte) (client.ConfigImportResult, error)
}

type dependencies struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	paths          platform.Paths
	readFile       func(string) ([]byte, error)
	writeFile      func(string, []byte) error
	readPassword   func(string) ([]byte, error)
	newClient      func(string, string) (managementClient, error)
}

type cliError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type globalOptions struct {
	json                bool
	endpoint, tokenFile string
}

func main() {
	paths, err := platform.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "davctl:", err)
		os.Exit(exitOperational)
	}
	deps := dependencies{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, paths: paths, readFile: os.ReadFile,
		writeFile: writeExportFile,
		readPassword: func(prompt string) ([]byte, error) {
			fmt.Fprint(os.Stderr, prompt)
			password, readErr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			return password, readErr
		},
		newClient: func(endpoint, token string) (managementClient, error) { return client.New(endpoint, token) },
	}
	os.Exit(run(os.Args[1:], deps))
}

func run(arguments []string, deps dependencies) int {
	options, remaining, err := parseGlobalOptions(arguments, deps.paths.TokenPath())
	if err != nil || len(remaining) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	if remaining[0] == "version" {
		if len(remaining) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		return runVersion(deps, options.json)
	}
	apiClient, code := connect(deps, options)
	if code != exitSuccess {
		return code
	}
	switch remaining[0] {
	case "status":
		if len(remaining) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		return runStatus(deps, options.json, apiClient)
	case "server":
		return runServer(deps, options.json, apiClient, remaining[1:])
	case "service":
		return runService(deps, options.json, apiClient, remaining[1:])
	case "logs":
		return runLogs(deps, options.json, apiClient, remaining[1:])
	case "user":
		return runUser(deps, options.json, apiClient, remaining[1:])
	case "share":
		return runShare(deps, options.json, apiClient, remaining[1:])
	case "acl":
		return runACL(deps, options.json, apiClient, remaining[1:])
	case "config":
		return runConfig(deps, options.json, apiClient, remaining[1:])
	case "revision":
		return runRevision(deps, options.json, apiClient, remaining[1:])
	case "tls":
		return runTLS(deps, options.json, apiClient, remaining[1:])
	case "doctor":
		if len(remaining) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		return runDoctor(deps, options.json, apiClient)
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func runService(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) != 1 {
		printUsage(deps.stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch arguments[0] {
	case "status":
		value, err := apiClient.ServiceStatus(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, value)
		}
		fmt.Fprintf(deps.stdout, "Installed: %t\nState: %s\n", value.Installed, value.State)
		return exitSuccess
	case "install", "uninstall", "start", "stop":
		var err error
		switch arguments[0] {
		case "install":
			err = apiClient.ServiceInstall(ctx)
		case "uninstall":
			err = apiClient.ServiceUninstall(ctx)
		case "start":
			err = apiClient.ServiceStart(ctx)
		case "stop":
			err = apiClient.ServiceStop(ctx)
		}
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, map[string]string{"result": arguments[0]})
		}
		fmt.Fprintf(deps.stdout, "Service %sed.\n", arguments[0])
		return exitSuccess
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func runServer(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch arguments[0] {
	case "settings":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		value, err := apiClient.ServerSettings(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, value)
		}
		fmt.Fprintf(deps.stdout, "HTTP port: %d\nHTTPS port: %d\n", value.HTTPPort, value.HTTPSPort)
		return exitSuccess
	case "ports":
		values, positional, err := parseValueFlags(arguments[1:], map[string]bool{"--http": true, "--https": true})
		if err != nil || len(positional) != 0 || values["--http"] == "" || values["--https"] == "" {
			printUsage(deps.stderr)
			return exitUsage
		}
		httpPort, err := strconv.Atoi(values["--http"])
		if err != nil {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_SERVER_PORTS", "HTTP port must be an integer")
		}
		httpsPort, err := strconv.Atoi(values["--https"])
		if err != nil {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_SERVER_PORTS", "HTTPS port must be an integer")
		}
		value, err := apiClient.UpdateServerPorts(ctx, httpPort, httpsPort)
		if err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, value)
		}
		fmt.Fprintf(deps.stdout, "WebDAV ports updated: HTTP %d, HTTPS %d.\n", value.HTTPPort, value.HTTPSPort)
		return exitSuccess
	case "status":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		value, err := apiClient.ServerStatus(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, value)
		}
		fmt.Fprintf(deps.stdout, "Caddy: %s\n", value.Caddy)
		return exitSuccess
	case "start":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		err := apiClient.StartServer(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
	case "stop":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		err := apiClient.StopServer(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
	case "restart":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		err := apiClient.RestartServer(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
	if jsonOutput {
		return encodeOutput(deps, map[string]string{"result": arguments[0]})
	}
	fmt.Fprintf(deps.stdout, "Caddy %sed.\n", arguments[0])
	return exitSuccess
}

func runVersion(deps dependencies, jsonOutput bool) int {
	info := buildinfo.Current()
	if jsonOutput {
		return encodeOutput(deps, info)
	}
	fmt.Fprintf(deps.stdout, "%s %s\nGit commit: %s\nBuild date: %s\nGo: %s\nFlutter: %s\nCaddy: %s\ncaddy-webdav: %s\nTarget: %s/%s\n", info.Product, info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.FlutterVersion, info.CaddyVersion, info.WebDAVVersion, info.TargetOS, info.TargetArch)
	return exitSuccess
}

func runDoctor(deps dependencies, jsonOutput bool, apiClient managementClient) int {
	report, err := apiClient.RunDiagnostics(context.Background())
	if err != nil {
		return printClientError(deps, jsonOutput, err)
	}
	if jsonOutput {
		if code := encodeOutput(deps, report); code != exitSuccess {
			return code
		}
	} else {
		fmt.Fprintf(deps.stdout, "DavDeck diagnostics: %s\n", report.Overall)
		for _, result := range report.Results {
			fmt.Fprintf(deps.stdout, "%s\t%s\t%s", result.Status, result.Title, result.Message)
			if result.Code != "" {
				fmt.Fprintf(deps.stdout, " (%s)", result.Code)
			}
			fmt.Fprintln(deps.stdout)
		}
	}
	if report.Overall == diagnostics.StatusFail {
		return exitOperational
	}
	return exitSuccess
}

func runTLS(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch arguments[0] {
	case "show":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		profile, err := apiClient.GetTLS(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, profile)
		}
		if profile == nil {
			fmt.Fprintln(deps.stdout, "TLS is not configured.")
			return exitSuccess
		}
		fmt.Fprintf(deps.stdout, "Mode: %s\nHostname: %s\n", profile.Mode, profile.Hostname)
		if profile.Mode == domain.TLSModeCustom {
			fmt.Fprintf(deps.stdout, "Certificate: %s\nPrivate key: %s\n", profile.CertificatePath, profile.PrivateKeyPath)
		}
		return exitSuccess
	case "check":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		result, err := apiClient.CheckTLS(ctx)
		if err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, result)
		}
		for _, check := range result.Checks {
			fmt.Fprintf(deps.stdout, "OK\t%s\t%s\n", check.Name, check.Message)
		}
		return exitSuccess
	case "automatic", "internal":
		if len(arguments) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		return updateTLS(deps, jsonOutput, apiClient, client.TLSUpdate{Mode: domain.TLSMode(arguments[0]), Hostname: arguments[1]})
	case "custom":
		values, positional, err := parseValueFlags(arguments[1:], map[string]bool{"--hostname": true, "--cert": true, "--key": true})
		if err != nil || len(positional) != 0 || values["--hostname"] == "" || values["--cert"] == "" || values["--key"] == "" {
			printUsage(deps.stderr)
			return exitUsage
		}
		return updateTLS(deps, jsonOutput, apiClient, client.TLSUpdate{Mode: domain.TLSModeCustom, Hostname: values["--hostname"], CertificatePath: values["--cert"], PrivateKeyPath: values["--key"]})
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func updateTLS(deps dependencies, jsonOutput bool, apiClient managementClient, update client.TLSUpdate) int {
	profile, err := apiClient.UpdateTLS(context.Background(), update)
	if err != nil {
		return printConfigurationError(deps, jsonOutput, err)
	}
	if jsonOutput {
		return encodeOutput(deps, profile)
	}
	fmt.Fprintf(deps.stdout, "Configured %s TLS for %s. Apply the configuration to activate it.\n", profile.Mode, profile.Hostname)
	return exitSuccess
}

func runConfig(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	switch arguments[0] {
	case "apply":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		revision, err := apiClient.ApplyConfig(context.Background())
		if err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, revision)
		}
		fmt.Fprintf(deps.stdout, "Applied configuration revision %d (%s).\n", revision.Number, revision.ConfigHash)
		return exitSuccess
	case "validate":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		result, err := apiClient.ValidateConfig(context.Background())
		if err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, result)
		}
		fmt.Fprintf(deps.stdout, "Configuration is valid (%s).\n", result.ConfigHash)
		for _, warning := range result.Warnings {
			fmt.Fprintf(deps.stdout, "Warning: %s\n", warning)
		}
		return exitSuccess
	case "status":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		state, err := apiClient.ConfigState(context.Background())
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, state)
		}
		desired, active := "none", "none"
		if state.DesiredRevision != nil {
			desired = fmt.Sprint(*state.DesiredRevision)
		}
		if state.ActiveRevision != nil {
			active = fmt.Sprint(*state.ActiveRevision)
		}
		fmt.Fprintf(deps.stdout, "Desired: %s\nActive:  %s\nPending: %t\n", desired, active, state.Pending)
		return exitSuccess
	case "export":
		values, positional, err := parseValueFlags(arguments[1:], map[string]bool{"--output": true})
		if err != nil || len(positional) != 0 {
			printUsage(deps.stderr)
			return exitUsage
		}
		content, err := apiClient.ExportConfig(context.Background())
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if output := values["--output"]; output != "" {
			if err := deps.writeFile(output, []byte(content)); err != nil {
				return printFailure(deps, jsonOutput, exitOperational, "OUTPUT_ERROR", "Configuration export file could not be created")
			}
			if jsonOutput {
				return encodeOutput(deps, map[string]any{"output": output, "contains_secrets": false})
			}
			fmt.Fprintf(deps.stdout, "Exported safe YAML configuration to %s.\n", output)
			return exitSuccess
		}
		if jsonOutput {
			return encodeOutput(deps, map[string]any{"format": "yaml", "content": content, "contains_secrets": false})
		}
		fmt.Fprint(deps.stdout, content)
		return exitSuccess
	case "import":
		if len(arguments) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		body, err := deps.readFile(arguments[1])
		if err != nil {
			return printFailure(deps, jsonOutput, exitOperational, "CONFIG_FILE_READ_FAILED", "Configuration file could not be read")
		}
		result, err := apiClient.ImportConfig(context.Background(), body)
		if err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, result)
		}
		fmt.Fprintf(deps.stdout, "Imported desired configuration: %d users created, %d users updated, %d shares created, %d shares updated, %d permissions upserted.\n", result.UsersCreated, result.UsersUpdated, result.SharesCreated, result.SharesUpdated, result.PermissionsUpserted)
		if len(result.PasswordResetRequired) > 0 {
			fmt.Fprintf(deps.stdout, "Set passwords separately for: %s.\n", strings.Join(result.PasswordResetRequired, ", "))
		}
		fmt.Fprintln(deps.stdout, "Run config apply to activate the imported desired state.")
		return exitSuccess
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func runRevision(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	if arguments[0] == "restore" {
		if len(arguments) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		id, err := domain.ParseID(arguments[1])
		if err != nil {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_REVISION_ID", "Revision ID is invalid")
		}
		revision, err := apiClient.RestoreRevision(context.Background(), id)
		if err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, revision)
		}
		fmt.Fprintf(deps.stdout, "Restored configuration revision %d (%s).\n", revision.Number, revision.ConfigHash)
		return exitSuccess
	}
	if arguments[0] == "delete" {
		if len(arguments) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		id, err := domain.ParseID(arguments[1])
		if err != nil {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_REVISION_ID", "Revision ID is invalid")
		}
		if err := apiClient.DeleteRevision(context.Background(), id); err != nil {
			return printConfigurationError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, map[string]any{"id": id, "deleted": true})
		}
		fmt.Fprintf(deps.stdout, "Deleted configuration revision %s.\n", id)
		return exitSuccess
	}
	if len(arguments) != 1 || arguments[0] != "list" {
		printUsage(deps.stderr)
		return exitUsage
	}
	revisions, err := apiClient.ListRevisions(context.Background())
	if err != nil {
		return printClientError(deps, jsonOutput, err)
	}
	if jsonOutput {
		return encodeOutput(deps, revisions)
	}
	if len(revisions) == 0 {
		fmt.Fprintln(deps.stdout, "No revisions.")
		return exitSuccess
	}
	for _, revision := range revisions {
		fmt.Fprintf(deps.stdout, "%d\t%s\t%s\t%s\n", revision.Number, revision.ValidationStatus, revision.ApplyStatus, revision.ConfigHash)
	}
	return exitSuccess
}

func runLogs(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if hasArgument(arguments, "--follow") {
		return printFailure(deps, jsonOutput, exitUsage, "LOG_FOLLOW_UNSUPPORTED", "Live log follow is unavailable until the Management API provides a safe stream")
	}
	values, positional, err := parseValueFlags(arguments, map[string]bool{"--limit": true, "--cursor": true, "--since": true, "--level": true, "--component": true})
	if err != nil || len(positional) != 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	query := client.LogQuery{Level: values["--level"], Component: values["--component"]}
	if query.Level != "" {
		switch strings.ToUpper(query.Level) {
		case "DEBUG", "INFO", "WARN", "ERROR":
		default:
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_LOG_QUERY", "Log level must be DEBUG, INFO, WARN, or ERROR")
		}
	}
	if value := values["--limit"]; value != "" {
		query.Limit, err = strconv.Atoi(value)
		if err != nil || query.Limit < 1 || query.Limit > 200 {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_LOG_QUERY", "Log limit must be between 1 and 200")
		}
	}
	if value := values["--cursor"]; value != "" {
		query.Cursor, err = strconv.ParseUint(value, 10, 64)
		if err != nil || query.Cursor == 0 {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_LOG_QUERY", "Log cursor must be a positive integer")
		}
	}
	if value := values["--since"]; value != "" {
		since, parseErr := time.Parse(time.RFC3339Nano, value)
		if parseErr != nil {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_LOG_QUERY", "Log since must be an RFC3339 timestamp")
		}
		query.Since = &since
	}
	page, err := apiClient.Logs(context.Background(), query)
	if err != nil {
		return printClientError(deps, jsonOutput, err)
	}
	if jsonOutput {
		return encodeOutput(deps, page)
	}
	if len(page.Records) == 0 {
		fmt.Fprintln(deps.stdout, "No logs.")
		return exitSuccess
	}
	for _, record := range page.Records {
		fmt.Fprintf(deps.stdout, "%s\t%s\t%s\t%s\n", record.Timestamp.Format(time.RFC3339Nano), record.Level, record.Component, record.Message)
	}
	if page.HasMore {
		fmt.Fprintf(deps.stdout, "Next cursor: %d\n", page.NextCursor)
	}
	return exitSuccess
}

func hasArgument(arguments []string, wanted string) bool {
	for _, argument := range arguments {
		if argument == wanted {
			return true
		}
	}
	return false
}

func printConfigurationError(deps dependencies, jsonOutput bool, err error) int {
	var apiError *client.APIError
	if errors.As(err, &apiError) {
		return printFailure(deps, jsonOutput, exitConfiguration, apiError.Code, apiError.Message)
	}
	return printFailure(deps, jsonOutput, exitConnection, "DAEMON_UNAVAILABLE", "Unable to connect to the local DavDeck daemon")
}

func runACL(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch arguments[0] {
	case "list":
		if len(arguments) > 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		shares, err := apiClient.ListShares(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if len(arguments) == 2 {
			share, err := resolveShare(ctx, apiClient, arguments[1])
			if err != nil {
				return printClientError(deps, jsonOutput, err)
			}
			shares = []client.Share{share}
		}
		all := make([]client.PermissionEntry, 0)
		for _, share := range shares {
			entries, err := apiClient.ListPermissions(ctx, share.ID)
			if err != nil {
				return printClientError(deps, jsonOutput, err)
			}
			all = append(all, entries...)
		}
		if jsonOutput {
			return encodeOutput(deps, all)
		}
		if len(all) == 0 {
			fmt.Fprintln(deps.stdout, "No permissions.")
			return exitSuccess
		}
		for _, entry := range all {
			fmt.Fprintf(deps.stdout, "%s\t%s\t%s\t%s\n", entry.ShareID, entry.UserID, entry.Username, entry.Permission)
		}
		return exitSuccess
	case "set":
		if len(arguments) != 4 {
			printUsage(deps.stderr)
			return exitUsage
		}
		share, err := resolveShare(ctx, apiClient, arguments[1])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		user, err := resolveUser(ctx, apiClient, arguments[2])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		permission, ok := parsePermission(arguments[3])
		if !ok {
			return printFailure(deps, jsonOutput, exitUsage, "INVALID_PERMISSION", "Permission must be none, read, or read-write")
		}
		entry, err := apiClient.SetPermission(ctx, share.ID, user.ID, permission)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, entry)
		}
		fmt.Fprintf(deps.stdout, "Set %s access for %s on %s.\n", strings.ToLower(strings.ReplaceAll(string(permission), "_", "-")), user.Username, share.Name)
		return exitSuccess
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func parsePermission(value string) (domain.Permission, bool) {
	switch strings.ToLower(value) {
	case "none":
		return domain.PermissionNone, true
	case "read":
		return domain.PermissionRead, true
	case "read-write", "read_write":
		return domain.PermissionReadWrite, true
	default:
		return "", false
	}
}

func runShare(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch arguments[0] {
	case "list":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		shares, err := apiClient.ListShares(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, shares)
		}
		if len(shares) == 0 {
			fmt.Fprintln(deps.stdout, "No shares.")
			return exitSuccess
		}
		for _, share := range shares {
			fmt.Fprintf(deps.stdout, "%s\t%s\t%s\t%s\t%t\n", share.ID, share.Slug, share.Name, share.Path, share.Enabled)
		}
		return exitSuccess
	case "add":
		values, positional, err := parseValueFlags(arguments[1:], map[string]bool{"--slug": true})
		if err != nil || len(positional) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		slug := values["--slug"]
		if slug == "" {
			slug = slugify(positional[0])
		}
		share, err := apiClient.CreateShare(ctx, positional[0], slug, positional[1])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		return printShare(deps, jsonOutput, share, "Created")
	case "update":
		values, positional, err := parseValueFlags(arguments[1:], map[string]bool{"--name": true, "--slug": true, "--path": true})
		if err != nil || len(positional) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		enabledSet, enabled, validEnabledFlags := shareEnabledFlag(arguments[1:])
		if !validEnabledFlags {
			printUsage(deps.stderr)
			return exitUsage
		}
		update := client.ShareUpdate{}
		if value, ok := values["--name"]; ok {
			update.Name = &value
		}
		if value, ok := values["--slug"]; ok {
			update.Slug = &value
		}
		if value, ok := values["--path"]; ok {
			update.Path = &value
		}
		if enabledSet {
			update.Enabled = &enabled
		}
		if update.Name == nil && update.Slug == nil && update.Path == nil && update.Enabled == nil {
			printUsage(deps.stderr)
			return exitUsage
		}
		share, err := resolveShare(ctx, apiClient, positional[0])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		updated, err := apiClient.UpdateShare(ctx, share.ID, update)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		return printShare(deps, jsonOutput, updated, "Updated")
	case "remove":
		if len(arguments) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		share, err := resolveShare(ctx, apiClient, arguments[1])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if err := apiClient.DeleteShare(ctx, share.ID); err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, map[string]any{"id": share.ID, "deleted": true, "files_preserved": true})
		}
		fmt.Fprintf(deps.stdout, "Removed share %s (%s); physical files were preserved.\n", share.Name, share.ID)
		return exitSuccess
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func resolveShare(ctx context.Context, apiClient managementClient, reference string) (client.Share, error) {
	shares, err := apiClient.ListShares(ctx)
	if err != nil {
		return client.Share{}, err
	}
	for _, share := range shares {
		if string(share.ID) == reference || share.Slug == strings.ToLower(reference) || strings.EqualFold(share.Name, reference) {
			return share, nil
		}
	}
	return client.Share{}, &client.APIError{StatusCode: 404, Code: "SHARE_NOT_FOUND", Message: "Share was not found"}
}

func slugify(name string) string {
	var builder strings.Builder
	previousHyphen := false
	for _, character := range strings.ToLower(strings.TrimSpace(name)) {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if valid {
			builder.WriteRune(character)
			previousHyphen = false
		} else if !previousHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
			previousHyphen = true
		}
	}
	return strings.TrimSuffix(builder.String(), "-")
}

func parseValueFlags(arguments []string, allowed map[string]bool) (map[string]string, []string, error) {
	values := make(map[string]string)
	positional := make([]string, 0)
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--enable" || argument == "--disable" {
			continue
		}
		if strings.HasPrefix(argument, "--") {
			if !allowed[argument] || index+1 >= len(arguments) {
				return nil, nil, errors.New("invalid flag")
			}
			index++
			values[argument] = arguments[index]
		} else {
			positional = append(positional, argument)
		}
	}
	return values, positional, nil
}

func shareEnabledFlag(arguments []string) (bool, bool, bool) {
	found, enabled := false, false
	for _, argument := range arguments {
		if argument == "--enable" {
			if found {
				return false, false, false
			}
			found, enabled = true, true
		}
		if argument == "--disable" {
			if found {
				return false, false, false
			}
			found, enabled = true, false
		}
	}
	return found, enabled, true
}

func printShare(deps dependencies, jsonOutput bool, share client.Share, action string) int {
	if jsonOutput {
		return encodeOutput(deps, share)
	}
	fmt.Fprintf(deps.stdout, "%s share %s (%s) at %s.\n", action, share.Name, share.ID, share.Path)
	return exitSuccess
}

func connect(deps dependencies, options globalOptions) (managementClient, int) {
	endpoint := options.endpoint
	if endpoint == "" {
		body, err := deps.readFile(deps.paths.EndpointPath())
		if err != nil {
			return nil, printFailure(deps, options.json, exitConnection, "DAEMON_DISCOVERY_FAILED", "Unable to read the local daemon endpoint")
		}
		endpoint = strings.TrimSpace(string(body))
	}
	tokenBody, err := deps.readFile(options.tokenFile)
	if err != nil {
		return nil, printFailure(deps, options.json, exitConnection, "AUTH_TOKEN_UNAVAILABLE", "Unable to read the management token")
	}
	apiClient, err := deps.newClient(endpoint, strings.TrimSpace(string(tokenBody)))
	if err != nil {
		return nil, printFailure(deps, options.json, exitConnection, "INVALID_LOCAL_CONFIGURATION", err.Error())
	}
	return apiClient, exitSuccess
}

func runStatus(deps dependencies, jsonOutput bool, apiClient managementClient) int {
	snapshot, err := apiClient.Status(context.Background())
	if err != nil {
		return printClientError(deps, jsonOutput, err)
	}
	if jsonOutput {
		return encodeOutput(deps, snapshot)
	}
	desired, active := "none", "none"
	if snapshot.DesiredRevision != nil {
		desired = strconv.FormatUint(*snapshot.DesiredRevision, 10)
	}
	if snapshot.ActiveRevision != nil {
		active = strconv.FormatUint(*snapshot.ActiveRevision, 10)
	}
	fmt.Fprintf(deps.stdout, "DavDeck\nDaemon:   %s\nDatabase: %s (schema %d)\nVersion:  %s\nCaddy:    %s\nWebDAV:   %s\nService:  %s (installed: %t, starts at boot: %t)\nConfig:   %s desired / %s active (pending: %t)\n", snapshot.Daemon, snapshot.Database, snapshot.SchemaVersion, snapshot.Version, snapshot.Caddy, snapshot.WebDAV, snapshot.Service.State, snapshot.Service.Installed, snapshot.Service.StartsAtBoot, desired, active, snapshot.PendingChanges)
	if snapshot.LastErrorCode != "" {
		fmt.Fprintf(deps.stdout, "Last error: %s\n", snapshot.LastErrorCode)
	}
	if snapshot.PortableDaemonOwned {
		fmt.Fprintln(deps.stdout, "Portable daemon: owned by GUI")
	}
	return exitSuccess
}

func runUser(deps dependencies, jsonOutput bool, apiClient managementClient, arguments []string) int {
	if len(arguments) == 0 {
		printUsage(deps.stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch arguments[0] {
	case "list":
		if len(arguments) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		users, err := apiClient.ListUsers(ctx)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		return printUsers(deps, jsonOutput, users)
	case "add":
		args, passwordStdin := removeBoolFlag(arguments[1:], "--password-stdin")
		if len(args) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		password, code := obtainPassword(deps, jsonOutput, passwordStdin, "Password: ")
		if code != exitSuccess {
			return code
		}
		user, err := apiClient.CreateUser(ctx, args[0], password)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		return printUser(deps, jsonOutput, user, "Created")
	case "delete", "enable", "disable":
		if len(arguments) != 2 {
			printUsage(deps.stderr)
			return exitUsage
		}
		user, err := resolveUser(ctx, apiClient, arguments[1])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if arguments[0] == "delete" {
			if err := apiClient.DeleteUser(ctx, user.ID); err != nil {
				return printClientError(deps, jsonOutput, err)
			}
			if jsonOutput {
				return encodeOutput(deps, map[string]any{"id": user.ID, "deleted": true})
			}
			fmt.Fprintf(deps.stdout, "Deleted user %s (%s); files were preserved.\n", user.Username, user.ID)
			return exitSuccess
		}
		enabled := arguments[0] == "enable"
		updated, err := apiClient.SetUserEnabled(ctx, user.ID, enabled)
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		action := "Disabled"
		if enabled {
			action = "Enabled"
		}
		return printUser(deps, jsonOutput, updated, action)
	case "passwd":
		args, passwordStdin := removeBoolFlag(arguments[1:], "--password-stdin")
		if len(args) != 1 {
			printUsage(deps.stderr)
			return exitUsage
		}
		user, err := resolveUser(ctx, apiClient, args[0])
		if err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		password, code := obtainPassword(deps, jsonOutput, passwordStdin, "New password: ")
		if code != exitSuccess {
			return code
		}
		if err := apiClient.ChangeUserPassword(ctx, user.ID, password); err != nil {
			return printClientError(deps, jsonOutput, err)
		}
		if jsonOutput {
			return encodeOutput(deps, map[string]any{"id": user.ID, "password_changed": true})
		}
		fmt.Fprintf(deps.stdout, "Password changed for %s (%s).\n", user.Username, user.ID)
		return exitSuccess
	default:
		printUsage(deps.stderr)
		return exitUsage
	}
}

func resolveUser(ctx context.Context, apiClient managementClient, reference string) (client.User, error) {
	users, err := apiClient.ListUsers(ctx)
	if err != nil {
		return client.User{}, err
	}
	for _, user := range users {
		if string(user.ID) == reference || domain.NormalizeUsername(user.Username) == domain.NormalizeUsername(reference) {
			return user, nil
		}
	}
	return client.User{}, &client.APIError{StatusCode: 404, Code: "USER_NOT_FOUND", Message: "User was not found"}
}

func obtainPassword(deps dependencies, jsonOutput, fromStdin bool, prompt string) (string, int) {
	var body []byte
	var err error
	if fromStdin {
		body, err = io.ReadAll(io.LimitReader(deps.stdin, 74))
		body = []byte(strings.TrimSuffix(strings.TrimSuffix(string(body), "\n"), "\r"))
	} else if deps.readPassword != nil {
		body, err = deps.readPassword(prompt)
	} else {
		return "", printFailure(deps, jsonOutput, exitUsage, "PASSWORD_INPUT_REQUIRED", "Use --password-stdin or an interactive terminal")
	}
	if err != nil {
		return "", printFailure(deps, jsonOutput, exitOperational, "PASSWORD_READ_FAILED", "Unable to read password")
	}
	return string(body), exitSuccess
}

func printUsers(deps dependencies, jsonOutput bool, users []client.User) int {
	if jsonOutput {
		return encodeOutput(deps, users)
	}
	if len(users) == 0 {
		fmt.Fprintln(deps.stdout, "No users.")
		return exitSuccess
	}
	for _, user := range users {
		fmt.Fprintf(deps.stdout, "%s\t%s\t%t\n", user.ID, user.Username, user.Enabled)
	}
	return exitSuccess
}

func printUser(deps dependencies, jsonOutput bool, user client.User, action string) int {
	if jsonOutput {
		return encodeOutput(deps, user)
	}
	fmt.Fprintf(deps.stdout, "%s user %s (%s).\n", action, user.Username, user.ID)
	return exitSuccess
}

func encodeOutput(deps dependencies, value any) int {
	if err := json.NewEncoder(deps.stdout).Encode(value); err != nil {
		return printFailure(deps, true, exitOperational, "OUTPUT_ERROR", "Unable to encode command output")
	}
	return exitSuccess
}

func printClientError(deps dependencies, jsonOutput bool, err error) int {
	var apiError *client.APIError
	if errors.As(err, &apiError) {
		exitCode := exitOperational
		if apiError.StatusCode == 401 || apiError.StatusCode == 403 && apiError.Code != "PRIVILEGE_REQUIRED" {
			exitCode = exitConnection
		}
		return printFailure(deps, jsonOutput, exitCode, apiError.Code, apiError.Message)
	}
	return printFailure(deps, jsonOutput, exitConnection, "DAEMON_UNAVAILABLE", "Unable to connect to the local DavDeck daemon")
}

func parseGlobalOptions(arguments []string, defaultTokenFile string) (globalOptions, []string, error) {
	options := globalOptions{tokenFile: defaultTokenFile}
	remaining := make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch {
		case argument == "--json":
			options.json = true
		case argument == "--endpoint" || argument == "--token-file":
			if index+1 >= len(arguments) {
				return options, nil, errors.New("missing flag value")
			}
			index++
			if argument == "--endpoint" {
				options.endpoint = arguments[index]
			} else {
				options.tokenFile = arguments[index]
			}
		case strings.HasPrefix(argument, "--endpoint="):
			options.endpoint = strings.TrimPrefix(argument, "--endpoint=")
		case strings.HasPrefix(argument, "--token-file="):
			options.tokenFile = strings.TrimPrefix(argument, "--token-file=")
		default:
			remaining = append(remaining, argument)
		}
	}
	return options, remaining, nil
}

func removeBoolFlag(arguments []string, name string) ([]string, bool) {
	remaining := make([]string, 0, len(arguments))
	found := false
	for _, argument := range arguments {
		if argument == name {
			found = true
		} else {
			remaining = append(remaining, argument)
		}
	}
	return remaining, found
}

func printFailure(deps dependencies, jsonOutput bool, exitCode int, code, message string) int {
	if jsonOutput {
		_ = json.NewEncoder(deps.stderr).Encode(struct {
			Success bool     `json:"success"`
			Error   cliError `json:"error"`
		}{false, cliError{code, message}})
	} else {
		fmt.Fprintf(deps.stderr, "davctl: %s: %s\n", code, message)
	}
	return exitCode
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "usage: davctl [--json] version")
	fmt.Fprintln(output, "       davctl [--json] [--endpoint URL] [--token-file PATH] status")
	fmt.Fprintln(output, "       davctl [global options] server <start|stop|restart|status|settings>")
	fmt.Fprintln(output, "       davctl [global options] server ports --http PORT --https PORT")
	fmt.Fprintln(output, "       davctl [global options] service <install|uninstall|start|stop|status>")
	fmt.Fprintln(output, "       davctl [global options] user list")
	fmt.Fprintln(output, "       davctl [global options] user add <username> [--password-stdin]")
	fmt.Fprintln(output, "       davctl [global options] user <delete|enable|disable> <username-or-id>")
	fmt.Fprintln(output, "       davctl [global options] user passwd <username-or-id> [--password-stdin]")
	fmt.Fprintln(output, "       davctl [global options] share list")
	fmt.Fprintln(output, "       davctl [global options] share add <name> <path> [--slug SLUG]")
	fmt.Fprintln(output, "       davctl [global options] share update <share> [--name NAME] [--slug SLUG] [--path PATH] [--enable|--disable]")
	fmt.Fprintln(output, "       davctl [global options] share remove <share>")
	fmt.Fprintln(output, "       davctl [global options] acl list [share]")
	fmt.Fprintln(output, "       davctl [global options] acl set <share> <user> <none|read|read-write>")
	fmt.Fprintln(output, "       davctl [global options] config <status|apply>")
	fmt.Fprintln(output, "       davctl [global options] config export [--output PATH]")
	fmt.Fprintln(output, "       davctl [global options] config import <file>")
	fmt.Fprintln(output, "       davctl [global options] revision <list|restore|delete> [revision-id]")
	fmt.Fprintln(output, "       davctl [global options] tls <show|check>")
	fmt.Fprintln(output, "       davctl [global options] tls <automatic|internal> <hostname>")
	fmt.Fprintln(output, "       davctl [global options] tls custom --hostname HOST --cert PATH --key PATH")
	fmt.Fprintln(output, "       davctl [global options] doctor")
}

func writeExportFile(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
