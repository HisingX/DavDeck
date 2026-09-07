package configfile

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

const validYAML = `version: 1
server:
  public_base_path: /dav
  http_port: 8080
  https_port: 8443
  runtime_mode: portable
tls:
  mode: internal
  hostname: dav.local
users:
  - username: Alice
    enabled: true
shares:
  - name: Documents
    slug: documents
    path: /srv/documents
    enabled: true
    permissions:
      Alice: read_write
`

func TestParseStrictVersionedConfiguration(t *testing.T) {
	document, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if document.Version != 1 || document.Server == nil || document.TLS == nil || len(document.Users) != 1 || len(document.Shares) != 1 {
		t.Fatalf("document = %#v", document)
	}
	if permission, ok := ParsePermission(document.Shares[0].Permissions["Alice"]); !ok || permission != domain.PermissionReadWrite {
		t.Fatalf("permission = %q, ok = %t", permission, ok)
	}
}

func TestParseDoesNotTreatScalarTextAsASecretField(t *testing.T) {
	body := strings.Replace(validYAML, "name: Documents", `name: "Password: recovery notes"`, 1)
	if _, err := Parse([]byte(body)); err != nil {
		t.Fatalf("safe scalar text was rejected: %v", err)
	}
}

func TestParseRejectsUnsafeAndAmbiguousYAML(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
		code ErrorCode
	}{
		{"unsupported version", strings.Replace(validYAML, "version: 1", "version: 2", 1), CodeUnsupported},
		{"unknown secret field", validYAML + "password: secret\n", CodeInvalid},
		{"duplicate key", strings.Replace(validYAML, "version: 1", "version: 1\nversion: 1", 1), CodeInvalid},
		{"alias", "version: 1\nusers: &users []\nshares: *users\n", CodeInvalid},
		{"multiple documents", validYAML + "---\nversion: 1\n", CodeInvalid},
		{"duplicate normalized user", strings.Replace(validYAML, "shares:", "  - username: alice\n    enabled: true\nshares:", 1), CodeInvalid},
		{"traversal path", strings.Replace(validYAML, "/srv/documents", "/srv/../private", 1), CodeInvalid},
		{"invalid permission", strings.Replace(validYAML, "read_write", "owner", 1), CodeInvalid},
		{"missing enabled", strings.Replace(validYAML, "    enabled: true\nshares:", "shares:", 1), CodeInvalid},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Parse([]byte(testCase.body))
			var configError *Error
			if !errors.As(err, &configError) || configError.Code != testCase.code {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestExportIsDeterministicRoundTripsAndContainsNoSecrets(t *testing.T) {
	input := exportFixture(t)
	first, err := Export(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Users[0], input.Users[1] = input.Users[1], input.Users[0]
	input.Shares[0].Permissions[0], input.Shares[0].Permissions[1] = input.Shares[0].Permissions[1], input.Shares[0].Permissions[0]
	second, err := Export(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("exports differ\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	text := string(first)
	for _, forbidden := range []string{"SECRET_HASH", "plaintext-password", "management-token", "PRIVATE KEY-----"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%q leaked: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "private_key_path: /etc/davdeck/key.pem") || strings.Index(text, "username: Alice") > strings.Index(text, "username: Bob") || strings.Index(text, "Alice: read_write") > strings.Index(text, "Bob: read") {
		t.Fatalf("unexpected deterministic export: %s", text)
	}
	if _, err := Parse(first); err != nil {
		t.Fatalf("export did not round trip: %v\n%s", err, first)
	}
}

func TestExportIncludesDNSChallengeReferenceWithoutCredentialMaterial(t *testing.T) {
	input := exportFixture(t)
	stamp := input.ServerSettings.CreatedAt
	providerID := domain.ID("99999999-9999-4999-8999-999999999999")
	input.DNSProviderCredentials = []domain.DNSProviderCredential{{ID: providerID, Name: "Cloudflare production", Provider: domain.DNSProviderCloudflare, CreatedAt: stamp, UpdatedAt: stamp}}
	input.TLSProfile = &domain.TLSProfile{ID: input.TLSProfile.ID, Mode: domain.TLSModeAutomatic, Hostname: "dav.example.com", Challenge: domain.TLSChallengeDNS, DNSProviderID: &providerID, CreatedAt: stamp, UpdatedAt: stamp}
	body, err := Export(input)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "challenge: dns") || !strings.Contains(text, "dns_provider: Cloudflare production") || strings.Contains(text, "api_token") {
		t.Fatalf("DNS export = %s", text)
	}
	if _, err := Parse(body); err != nil {
		t.Fatalf("DNS export did not parse: %v\n%s", err, body)
	}
}

func exportFixture(t *testing.T) domain.RuntimeConfigInput {
	t.Helper()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	settings := domain.ServerSettings{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}
	tls := domain.TLSProfile{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Mode: domain.TLSModeCustom, Hostname: "dav.example.com", CertificatePath: "/etc/davdeck/cert.pem", PrivateKeyPath: "/etc/davdeck/key.pem", CreatedAt: stamp, UpdatedAt: stamp}
	alice := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", UsernameNormalized: "alice", PasswordHash: "SECRET_HASH:plaintext-password", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	bob := domain.User{ID: "22222222-2222-4222-8222-222222222222", Username: "Bob", UsernameNormalized: "bob", PasswordHash: "SECRET_HASH:management-token", Enabled: false, CreatedAt: stamp, UpdatedAt: stamp}
	share := domain.Share{ID: "33333333-3333-4333-8333-333333333333", Name: "Documents", Slug: "documents", Path: "/srv/documents", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	permissions := []domain.SharePermission{{ShareID: share.ID, UserID: bob.ID, Permission: domain.PermissionRead, CreatedAt: stamp, UpdatedAt: stamp}, {ShareID: share.ID, UserID: alice.ID, Permission: domain.PermissionReadWrite, CreatedAt: stamp, UpdatedAt: stamp}}
	return domain.RuntimeConfigInput{ServerSettings: settings, TLSProfile: &tls, Users: []domain.User{bob, alice}, Shares: []domain.ShareWithPermissions{{Share: share, Permissions: permissions}}}
}
