package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"davdeck.dev/davdeck/core/internal/api"
	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/buildinfo"
	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/diagnostics"
	"davdeck.dev/davdeck/core/internal/logging"
	"davdeck.dev/davdeck/core/internal/platform"
	"davdeck.dev/davdeck/core/internal/status"
	"davdeck.dev/davdeck/core/internal/storage"
)

func main() {
	if err := runPlatform(); err != nil {
		fmt.Fprintln(os.Stderr, "davd:", err)
		os.Exit(1)
	}
}

func run() error {
	stopChannel := make(chan os.Signal, 1)
	signal.Notify(stopChannel, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopChannel)
	return runDaemon(stopChannel)
}

func runDaemon(stopChannel <-chan os.Signal) error {
	build := buildinfo.Current()
	defaults, err := platform.DefaultPaths()
	if err != nil {
		return err
	}
	address := flag.String("listen", "127.0.0.1:0", "loopback Management API address")
	dataDir := flag.String("data-dir", defaults.DataDir, "DavDeck data directory")
	configDir := flag.String("config-dir", defaults.ConfigDir, "DavDeck config directory")
	runtimeDir := flag.String("runtime-dir", defaults.RuntimeDir, "DavDeck runtime directory")
	portableOwner := flag.String("portable-owner", "", "owner of a portable daemon instance (currently: gui)")
	caddyBinary := flag.String("caddy-binary", "", "managed Caddy binary path (defaults to bundled Caddy)")
	caddyAdmin := flag.String("caddy-admin", "http://127.0.0.1:2019", "loopback Caddy Admin endpoint")
	flag.Parse()
	if *portableOwner != "" && *portableOwner != "gui" {
		return fmt.Errorf("portable-owner must be empty or gui")
	}
	resolvedCaddyBinary := platform.ResolveCaddyBinary(*caddyBinary)
	daemonExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve daemon executable: %w", err)
	}
	serviceManager, err := platform.NewServiceManager(platform.ServiceConfig{
		Executable: daemonExecutable,
		Arguments: []string{
			"--listen", *address,
			"--data-dir", *dataDir,
			"--config-dir", *configDir,
			"--runtime-dir", *runtimeDir,
			"--caddy-binary", resolvedCaddyBinary,
			"--caddy-admin", *caddyAdmin,
		},
		Description: "DavDeck WebDAV Server Manager",
	})
	if err != nil {
		return fmt.Errorf("configure service manager: %w", err)
	}
	instanceLock, err := platform.AcquireInstanceLock(*runtimeDir)
	if err != nil {
		return err
	}
	defer instanceLock.Release()

	logStore := logging.NewStore(logging.DefaultCapacity)
	logger := logging.NewWithStore(os.Stderr, slog.LevelInfo, "davd", logStore)
	caddyStdout := logging.NewLineWriter(logStore, "caddy", "INFO", os.Stdout)
	caddyStderr := logging.NewLineWriter(logStore, "caddy", "ERROR", os.Stderr)
	defer caddyStdout.Flush()
	defer caddyStderr.Flush()
	ctx := context.Background()
	database, schemaVersion, err := storage.Open(ctx, filepath.Join(*dataDir, "davdeck.db"))
	if err != nil {
		return err
	}
	defer database.Close()
	token, err := api.LoadOrCreateToken(filepath.Join(*configDir, "management.token"))
	if err != nil {
		return err
	}
	userRepository := storage.NewUserRepository(database)
	shareRepository := storage.NewShareRepository(database)
	adminClient, err := caddyruntime.NewAdminClient(*caddyAdmin)
	if err != nil {
		return err
	}
	validator := caddyruntime.BinaryValidator{BinaryPath: resolvedCaddyBinary, TempDirectory: *runtimeDir}
	runtimeManager := caddyruntime.NewRuntimeManager(resolvedCaddyBinary, filepath.Join(*runtimeDir, "caddy.json"), validator, adminClient, caddyStdout, caddyStderr)
	runtimeManager.SetLogger(logger.With("component", "runtime"))
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := runtimeManager.Stop(shutdownContext); err != nil {
			logger.Error("stop managed Caddy runtime", "error", "shutdown failed")
		}
	}()
	snapshotRepository := storage.NewSnapshotRepository(database)
	applyService := app.NewApplyService(snapshotRepository, caddyruntime.Compiler{}, validator, runtimeManager, storage.NewRevisionRepository(database), app.CryptoIDGenerator{}, app.SystemClock{}, build.Version)
	tlsService := app.NewTLSService(storage.NewTLSRepository(database), app.SystemTLSResolver{}, app.SystemTLSFileChecker{}, app.CryptoIDGenerator{}, app.SystemClock{})
	configService := app.NewConfigService(snapshotRepository, storage.NewConfigRepository(database), platform.SharePathValidator{}, app.BcryptHasher{}, app.CryptoIDGenerator{}, app.SystemClock{})
	diagnosticsService := diagnostics.NewService([]diagnostics.Check{
		diagnostics.DatabaseCheck{Database: database, SchemaVersion: schemaVersion},
		diagnostics.DirectoryCheck{Name: "config", Path: *configDir, Required: true},
		diagnostics.DirectoryCheck{Name: "data", Path: *dataDir, Required: true},
		diagnostics.DirectoryCheck{Name: "logs", Path: defaults.LogDir},
		diagnostics.DirectoryCheck{Name: "runtime", Path: *runtimeDir, Required: true},
		diagnostics.CaddyBinaryCheck{Inspector: caddyruntime.ModuleInspector{BinaryPath: resolvedCaddyBinary}},
		diagnostics.CaddyRuntimeCheck{Runtime: runtimeManager},
		diagnostics.ConfigCheck{Snapshots: snapshotRepository, Compiler: caddyruntime.Compiler{}, Validator: validator},
		diagnostics.SharePathsCheck{Shares: shareRepository, Paths: platform.SharePathValidator{}},
		diagnostics.TLSCheck{TLS: tlsService},
	}, diagnostics.SystemClock{}, build.Version)
	server, err := api.NewServer(*address, token, status.Snapshot{
		Name: "DavDeck", Version: build.Version, Daemon: status.DaemonRunning,
		Database: status.DatabaseReady, SchemaVersion: schemaVersion,
		PortableDaemonOwned: *portableOwner == "gui",
	}, logger, api.WithLogStore(logStore), api.WithUserService(app.NewUserService(
		userRepository,
		app.BcryptHasher{},
		app.CryptoIDGenerator{},
		app.SystemClock{},
	)), api.WithShareService(app.NewShareService(
		shareRepository,
		platform.SharePathValidator{},
		app.CryptoIDGenerator{},
		app.SystemClock{},
	)), api.WithPermissionService(app.NewPermissionService(
		storage.NewPermissionRepository(database),
		shareRepository,
		userRepository,
		app.SystemClock{},
	)), api.WithApplyService(applyService), api.WithRuntimeService(applyService), api.WithServerSettingsService(app.NewServerSettingsService(storage.NewServerSettingsRepository(database), platform.PortChecker{}, app.SystemClock{})), api.WithTLSService(tlsService), api.WithDiagnosticsService(diagnosticsService), api.WithConfigService(configService), api.WithServiceManager(serviceManager))
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return fmt.Errorf("listen on management address: %w", err)
	}
	endpoint := "http://" + listener.Addr().String()
	if err := writeEndpoint(filepath.Join(*runtimeDir, "management.endpoint"), endpoint); err != nil {
		listener.Close()
		return err
	}
	defer removeEndpoint(filepath.Join(*runtimeDir, "management.endpoint"))
	logger.Info("management API started", "address", listener.Addr().String(), "schema_version", schemaVersion)

	errChannel := make(chan error, 1)
	go func() { errChannel <- server.Serve(listener) }()
	select {
	case serveErr := <-errChannel:
		if !errors.Is(serveErr, net.ErrClosed) {
			return fmt.Errorf("serve management API: %w", serveErr)
		}
	case signalValue := <-stopChannel:
		logger.Info("shutdown requested", "signal", signalValue.String())
		if err := shutdownManagementAPI(server); err != nil {
			return err
		}
	case <-server.ShutdownRequested():
		logger.Info("shutdown requested", "source", "management_api")
		if err := shutdownManagementAPI(server); err != nil {
			return err
		}
	}
	return nil
}

func removeEndpoint(path string) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return
	}
	_ = os.Remove(path)
}

func shutdownManagementAPI(server *api.Server) error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown management API: %w", err)
	}
	return nil
}

func writeEndpoint(path, endpoint string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}
	if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("management endpoint path must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect management endpoint: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open management endpoint: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("secure management endpoint: %w", err)
	}
	if _, err := file.WriteString(endpoint + "\n"); err != nil {
		file.Close()
		return fmt.Errorf("write management endpoint: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close management endpoint: %w", err)
	}
	return nil
}
