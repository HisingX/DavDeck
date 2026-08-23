package caddy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestCompilerMatchesGoldenAndIsDeterministic(t *testing.T) {
	input := compilerFixture(t)
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/users_shares_acl.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compiled.JSON, want) {
		t.Fatalf("compiled JSON differs from golden\n--- got ---\n%s\n--- want ---\n%s", compiled.JSON, want)
	}
	hash := sha256.Sum256(compiled.JSON)
	if compiled.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("hash = %s", compiled.SHA256)
	}
	input.Users[0], input.Users[2] = input.Users[2], input.Users[0]
	input.Shares[0].Permissions[0], input.Shares[0].Permissions[2] = input.Shares[0].Permissions[2], input.Shares[0].Permissions[0]
	shuffled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compiled.JSON, shuffled.JSON) || compiled.SHA256 != shuffled.SHA256 {
		t.Fatal("compiler output changed with input ordering")
	}
}

func TestCompilerEnforcesReferencesAndOmitsDisabledState(t *testing.T) {
	input := compilerFixture(t)
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(compiled.JSON, []byte("Charlie")) || bytes.Contains(compiled.JSON, []byte("private")) {
		t.Fatalf("disabled state leaked into config: %s", compiled.JSON)
	}
	input.Shares[0].Permissions[0].UserID = "99999999-9999-4999-8999-999999999999"
	if _, err := (Compiler{}).Compile(input); err == nil {
		t.Fatal("unknown ACL user was accepted")
	}
}

func TestCompilerWarnsWhenShareHasNoAuthorizedUsers(t *testing.T) {
	input := compilerFixture(t)
	input.Shares[0].Permissions = nil
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Warnings) != 1 || len(compiled.JSON) == 0 {
		t.Fatalf("compiled = %#v", compiled)
	}
}

func TestCompilerSelectsManagedTLSModes(t *testing.T) {
	for _, testCase := range []struct {
		mode domain.TLSMode
		want []string
	}{
		{domain.TLSModeAutomatic, []string{`":8446"`, `"host"`, `"dav.example.com"`}},
		{domain.TLSModeInternal, []string{`"module": "internal"`, `"subjects"`}},
		{domain.TLSModeCustom, []string{`"load_files"`, `"certificate": "/cert.pem"`, `"key": "/key.pem"`, `"any_tag"`}},
	} {
		t.Run(string(testCase.mode), func(t *testing.T) {
			input := compilerFixture(t)
			input.ServerSettings.HTTPPort, input.ServerSettings.HTTPSPort = 8089, 8446
			stamp := input.ServerSettings.CreatedAt
			profile := domain.TLSProfile{ID: "66666666-6666-4666-8666-666666666666", Mode: testCase.mode, Hostname: "dav.example.com", CreatedAt: stamp, UpdatedAt: stamp}
			if testCase.mode == domain.TLSModeCustom {
				profile.CertificatePath, profile.PrivateKeyPath = "/cert.pem", "/key.pem"
			}
			input.TLSProfile = &profile
			compiled, err := (Compiler{}).Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range testCase.want {
				if !bytes.Contains(compiled.JSON, []byte(want)) {
					t.Fatalf("mode %s missing %q: %s", testCase.mode, want, compiled.JSON)
				}
			}
			for _, want := range []string{`"http_port": 8089`, `"https_port": 8446`} {
				if !bytes.Contains(compiled.JSON, []byte(want)) {
					t.Fatalf("mode %s missing managed listener port %q: %s", testCase.mode, want, compiled.JSON)
				}
			}
			if !bytes.Contains(compiled.JSON, []byte(`":8446"`)) {
				t.Fatalf("TLS mode does not listen on HTTPS port: %s", compiled.JSON)
			}
			if !bytes.Contains(compiled.JSON, []byte(`"disable_redirects": true`)) {
				t.Fatalf("TLS mode did not disable Caddy's automatic redirect: %s", compiled.JSON)
			}
			if !bytes.Contains(compiled.JSON, []byte(`https://dav.example.com:8446{http.request.uri}`)) {
				t.Fatalf("TLS mode did not emit the managed HTTPS port in the redirect: %s", compiled.JSON)
			}
		})
	}
}

func compilerFixture(t *testing.T) RuntimeConfigInput {
	t.Helper()
	stamp, err := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	settings := domain.ServerSettings{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}
	alice := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", UsernameNormalized: "alice", PasswordHash: "$2a$12$alice", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	bob := domain.User{ID: "22222222-2222-4222-8222-222222222222", Username: "Bob", UsernameNormalized: "bob", PasswordHash: "$2a$12$bob", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	charlie := domain.User{ID: "33333333-3333-4333-8333-333333333333", Username: "Charlie", UsernameNormalized: "charlie", PasswordHash: "$2a$12$charlie", Enabled: false, CreatedAt: stamp, UpdatedAt: stamp}
	documents := domain.Share{ID: "44444444-4444-4444-8444-444444444444", Name: "Documents", Slug: "documents", Path: "/srv/documents", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	private := domain.Share{ID: "55555555-5555-4555-8555-555555555555", Name: "Private", Slug: "private", Path: "/srv/private", Enabled: false, CreatedAt: stamp, UpdatedAt: stamp}
	permission := func(shareID, userID domain.ID, value domain.Permission) domain.SharePermission {
		return domain.SharePermission{ShareID: shareID, UserID: userID, Permission: value, CreatedAt: stamp, UpdatedAt: stamp}
	}
	return RuntimeConfigInput{ServerSettings: settings, Users: []domain.User{charlie, bob, alice}, Shares: []ShareWithPermissions{{Share: documents, Permissions: []domain.SharePermission{permission(documents.ID, charlie.ID, domain.PermissionReadWrite), permission(documents.ID, bob.ID, domain.PermissionRead), permission(documents.ID, alice.ID, domain.PermissionReadWrite)}}, {Share: private, Permissions: []domain.SharePermission{permission(private.ID, alice.ID, domain.PermissionReadWrite)}}}}
}
