package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestTLSProfileModes(t *testing.T) {
	t.Parallel()
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	for _, profile := range []TLSProfile{
		{ID: testID, Mode: TLSModeAutomatic, Hostname: "dav.example.com", CreatedAt: stamp, UpdatedAt: stamp},
		{ID: testID, Mode: TLSModeInternal, Hostname: "davdeck.local", CreatedAt: stamp, UpdatedAt: stamp},
		{ID: testID, Mode: TLSModeCustom, Hostname: "dav.example.com", CertificatePath: "/etc/davdeck/cert.pem", PrivateKeyPath: "/etc/davdeck/key.pem", CreatedAt: stamp, UpdatedAt: stamp},
	} {
		if err := profile.Validate(); err != nil {
			t.Errorf("profile %q: %v", profile.Mode, err)
		}
	}
	invalidHost := TLSProfile{ID: testID, Mode: TLSModeInternal, Hostname: "https://dav.local", CreatedAt: stamp, UpdatedAt: stamp}
	assertValidationCode(t, invalidHost.Validate(), CodeInvalidHostname)
	missingKey := TLSProfile{ID: testID, Mode: TLSModeCustom, Hostname: "dav.local", CertificatePath: "/cert.pem", CreatedAt: stamp, UpdatedAt: stamp}
	assertValidationCode(t, missingKey.Validate(), CodeInvalidPrivateKey)
}

func TestServerSettingsValidation(t *testing.T) {
	t.Parallel()
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	settings := ServerSettings{ID: testID, PublicBasePath: "/dav", HTTPPort: 80, HTTPSPort: 443, RuntimeMode: RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}
	if err := settings.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name string
		edit func(*ServerSettings)
		code ErrorCode
	}{
		{"relative base path", func(value *ServerSettings) { value.PublicBasePath = "dav" }, CodeInvalidBasePath},
		{"non-canonical base path", func(value *ServerSettings) { value.PublicBasePath = "/dav/../files" }, CodeInvalidBasePath},
		{"invalid port", func(value *ServerSettings) { value.HTTPPort = 0 }, CodeInvalidPort},
		{"duplicate ports", func(value *ServerSettings) { value.HTTPSPort = 80 }, CodeInvalidPort},
		{"invalid runtime", func(value *ServerSettings) { value.RuntimeMode = "desktop" }, CodeInvalidRuntimeMode},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			value := settings
			testCase.edit(&value)
			assertValidationCode(t, value.Validate(), testCase.code)
		})
	}
}

func TestConfigRevisionValidation(t *testing.T) {
	t.Parallel()
	config := []byte(`{"apps":{"http":{}}}`)
	hash := fmt.Sprintf("%x", sha256.Sum256(config))
	revision := ConfigRevision{
		ID: testID, Number: 1, CreatedAt: testTimestamp(t, "2026-08-20T00:00:00Z"),
		ConfigJSON: config, ConfigHash: hash, ValidationStatus: RevisionValidationValid,
		ApplyStatus: RevisionApplyApplied, AppVersion: "0.1.0",
	}
	if err := revision.Validate(); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(revision)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"apps"`) || strings.Contains(string(body), "ConfigJSON") {
		t.Fatalf("serialized revision leaked generated config: %s", body)
	}
	wrongHash := revision
	wrongHash.ConfigHash = fmt.Sprintf("%064d", 0)
	assertValidationCode(t, wrongHash.Validate(), CodeInvalidConfigHash)
	invalidJSON := revision
	invalidJSON.ConfigJSON = []byte("{")
	assertValidationCode(t, invalidJSON.Validate(), CodeInvalidConfig)
	appliedWithoutValidation := revision
	appliedWithoutValidation.ValidationStatus = RevisionValidationPending
	assertValidationCode(t, appliedWithoutValidation.Validate(), CodeInvalidRevisionStatus)
	failedWithoutDetails := revision
	failedWithoutDetails.ApplyStatus = RevisionApplyFailed
	assertValidationCode(t, failedWithoutDetails.Validate(), CodeInvalidRevisionStatus)
	failed := revision
	failed.ApplyStatus = RevisionApplyFailed
	failed.ErrorCode = "CADDY_RELOAD_FAILED"
	failed.ErrorSummary = "Caddy rejected the generated configuration"
	if err := failed.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRevisionSnapshotRoundTripsDisabledUsersAndPermissions(t *testing.T) {
	t.Parallel()
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	user := User{ID: testID, Username: "Alice", UsernameNormalized: "alice", PasswordHash: "$2a$12$example-hash", Enabled: false, CreatedAt: stamp, UpdatedAt: stamp}
	share := Share{ID: testOtherID, Name: "Photos", Slug: "photos", Path: "/srv/photos", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	input := RuntimeConfigInput{
		ServerSettings: ServerSettings{ID: testID, PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp},
		Users:          []User{user},
		Shares:         []ShareWithPermissions{{Share: share, Permissions: []SharePermission{{ShareID: share.ID, UserID: user.ID, Permission: PermissionReadWrite, CreatedAt: stamp, UpdatedAt: stamp}}}},
	}
	body, err := MarshalConfigRevisionSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "password_hash") || !strings.Contains(string(body), user.PasswordHash) {
		t.Fatalf("snapshot did not contain the internal password hash needed for rollback: %s", body)
	}
	restored, err := ParseConfigRevisionSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Users) != 1 || restored.Users[0].Enabled || restored.Users[0].PasswordHash != user.PasswordHash {
		t.Fatalf("restored users = %#v", restored.Users)
	}
	if len(restored.Shares) != 1 || len(restored.Shares[0].Permissions) != 1 || restored.Shares[0].Permissions[0].Permission != PermissionReadWrite {
		t.Fatalf("restored shares = %#v", restored.Shares)
	}
}

func TestConfigRevisionSnapshotRejectsDanglingPermission(t *testing.T) {
	t.Parallel()
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	input := RuntimeConfigInput{
		ServerSettings: ServerSettings{ID: testID, PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp},
		Shares:         []ShareWithPermissions{{Share: Share{ID: testOtherID, Name: "Photos", Slug: "photos", Path: "/srv/photos", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}, Permissions: []SharePermission{{ShareID: testOtherID, UserID: testID, Permission: PermissionRead, CreatedAt: stamp, UpdatedAt: stamp}}}},
	}
	if _, err := MarshalConfigRevisionSnapshot(input); err == nil {
		t.Fatal("expected dangling permission to be rejected")
	}
}
