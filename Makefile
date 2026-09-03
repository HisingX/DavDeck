.PHONY: check core-format core-vet core-test core-build gui-format gui-analyze gui-test gui-build-macos caddy-module-test caddy-build caddy-verify caddy-tooling-test caddy-integration-test caddy-security-release-gate platform-smoke release-packaging-test release-package linux-release-smoke macos-app smoke

ifeq ($(OS),Windows_NT)
CADDY_BINARY ?= $(CURDIR)/core/bin/caddy.exe
else
CADDY_BINARY ?= $(CURDIR)/core/bin/caddy
endif

check: core-format core-vet core-test core-build gui-format gui-analyze gui-test

core-format:
	cd core && test -z "$$(gofmt -l .)"

core-vet:
	cd core && go vet ./...

core-test:
	cd core && go test ./...

core-build:
	cd core && go build ./cmd/davd && go build ./cmd/davctl

gui-format:
	cd gui && dart format --output=none --set-exit-if-changed lib test

gui-analyze:
	cd gui && flutter analyze

gui-test:
	cd gui && flutter test

gui-build-macos:
	cd gui && flutter build macos

caddy-module-test:
	cd caddy/caddy-webdav && go test ./...
	cd caddy/caddy-dnspod && go test ./...

caddy-build:
	./scripts/build_caddy.sh "$(CADDY_BINARY)"

caddy-verify:
	./scripts/verify_caddy.sh "$(CADDY_BINARY)"

caddy-tooling-test:
	./scripts/test_caddy_tooling.sh

release-packaging-test:
	./scripts/test_release_packaging.sh

release-package:
	test -n "$(VERSION)" && test -n "$(TARGET)"
	./scripts/package_release.sh "$(VERSION)" "$(TARGET)" "$(or $(OUTPUT_DIR),dist)"

linux-release-smoke:
	test -n "$(ARCHIVE)"
	./scripts/smoke_linux_release.sh "$(ARCHIVE)"

macos-app:
	test -n "$(VERSION)"
	./scripts/package_macos_app.sh "$(VERSION)" "$(or $(OUTPUT_DIR),dist)"

caddy-integration-test:
	cd core && DAVDECK_CADDY_BINARY="$(CADDY_BINARY)" go test ./internal/caddy ./integration -run 'Test(DNSChallengeConfigValidatesWithPinnedCaddy|Pinned(CaddyRuntimeLifecycle|CaddyStartsInternalTLSEndpoint|CaddyRenewalAdminEndpoint|WebDAVAuthenticationAndACLMatrix)|ApplyWorkflowWithPinnedRuntime)' -count=1 -v

caddy-security-release-gate:
	cd core && DAVDECK_CADDY_BINARY="$(CADDY_BINARY)" go test ./internal/caddy -run TestPinnedWebDAVAuthenticationAndACLMatrix -count=1 -v

platform-smoke:
	./scripts/smoke_supported_targets.sh

smoke:
	./scripts/smoke_phase0.sh
