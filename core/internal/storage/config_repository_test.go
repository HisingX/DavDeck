package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/configfile"
	"davdeck.dev/davdeck/core/internal/domain"
)

func TestSQLiteConfigRepositoryMergesTransactionallyAndPreservesSecrets(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	const aliceID = "11111111-1111-4111-8111-111111111111"
	const docsID = "22222222-2222-4222-8222-222222222222"
	if _, err := database.Exec(`INSERT INTO users(id, username, username_normalized, password_hash, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, aliceID, "Alice", "alice", "ORIGINAL_SECRET_HASH", true, stamp.String(), stamp.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO shares(id, name, slug, path, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, docsID, "Old Docs", "documents", "/old/docs", true, stamp.String(), stamp.String()); err != nil {
		t.Fatal(err)
	}
	falseValue, trueValue := false, true
	document := configfile.Document{Version: 1,
		Server: &configfile.Server{PublicBasePath: "/webdav", HTTPPort: 9080, HTTPSPort: 9443, RuntimeMode: "service"},
		TLS:    &configfile.TLS{Mode: "internal", Hostname: "dav.local"},
		Users:  []configfile.User{{Username: "ALICE", Enabled: &falseValue}, {Username: "Bob", Enabled: &trueValue}},
		Shares: []configfile.Share{
			{Name: "Documents", Slug: "documents", Path: "/srv/documents", Enabled: &trueValue, Permissions: configfile.Permissions{"Alice": "read_write", "Bob": "read"}},
			{Name: "Photos", Slug: "photos", Path: "/srv/photos", Enabled: &falseValue},
		},
	}
	seed := app.ConfigImportSeed{Document: document,
		UserIDs: map[string]domain.ID{"alice": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bob": "33333333-3333-4333-8333-333333333333"}, UserHashes: map[string]string{"alice": "unused", "bob": "RANDOM_PLACEHOLDER_HASH"},
		ShareIDs: map[string]domain.ID{"documents": "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "photos": "44444444-4444-4444-8444-444444444444"}, TLSID: "55555555-5555-4555-8555-555555555555", Timestamp: stamp}
	result, err := NewConfigRepository(database).Import(ctx, seed)
	if err != nil {
		t.Fatal(err)
	}
	if result.UsersCreated != 1 || result.UsersUpdated != 1 || result.SharesCreated != 1 || result.SharesUpdated != 1 || result.PermissionsUpserted != 2 || !result.ServerUpdated || !result.TLSUpdated {
		t.Fatalf("result = %#v", result)
	}
	var hash, username string
	var enabled bool
	if err := database.QueryRow(`SELECT username, password_hash, enabled FROM users WHERE id = ?`, aliceID).Scan(&username, &hash, &enabled); err != nil || hash != "ORIGINAL_SECRET_HASH" || enabled || username != "ALICE" {
		t.Fatalf("existing user username=%q hash=%q enabled=%t err=%v", username, hash, enabled, err)
	}
	if err := database.QueryRow(`SELECT password_hash FROM users WHERE username_normalized = 'bob'`).Scan(&hash); err != nil || hash != "RANDOM_PLACEHOLDER_HASH" {
		t.Fatalf("new hash = %q, err = %v", hash, err)
	}
	var userCount, shareCount, permissionCount, port int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM shares`).Scan(&shareCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM share_permissions`).Scan(&permissionCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT http_port FROM server_settings LIMIT 1`).Scan(&port); err != nil {
		t.Fatal(err)
	}
	if userCount != 2 || shareCount != 2 || permissionCount != 2 || port != 9080 {
		t.Fatalf("counts users=%d shares=%d permissions=%d port=%d", userCount, shareCount, permissionCount, port)
	}

	conflict := configfile.Document{Version: 1, Server: &configfile.Server{PublicBasePath: "/changed", HTTPPort: 7080, HTTPSPort: 7443, RuntimeMode: "portable"}, Users: []configfile.User{{Username: "Charlie", Enabled: &trueValue}}, Shares: []configfile.Share{{Name: "Bad", Slug: "bad", Path: "/srv/bad", Enabled: &falseValue, Permissions: configfile.Permissions{"Missing": "read"}}}}
	conflictSeed := app.ConfigImportSeed{Document: conflict, UserIDs: map[string]domain.ID{"charlie": "66666666-6666-4666-8666-666666666666"}, UserHashes: map[string]string{"charlie": "hash"}, ShareIDs: map[string]domain.ID{"bad": "77777777-7777-4777-8777-777777777777"}, Timestamp: stamp}
	if _, err := NewConfigRepository(database).Import(ctx, conflictSeed); !errors.Is(err, app.ErrConfigImportConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil || userCount != 2 {
		t.Fatalf("rollback user count = %d, err = %v", userCount, err)
	}
	if err := database.QueryRow(`SELECT http_port FROM server_settings LIMIT 1`).Scan(&port); err != nil || port != 9080 {
		t.Fatalf("rollback port = %d, err = %v", port, err)
	}
}

func TestSQLiteConfigRepositoryResolvesDNSProviderByPortableName(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	providerID := domain.ID("11111111-1111-4111-8111-111111111111")
	if _, err := database.Exec(`INSERT INTO dns_provider_credentials(id, name, provider, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, providerID, "Cloudflare production", domain.DNSProviderCloudflare, stamp.String(), stamp.String()); err != nil {
		t.Fatal(err)
	}
	document := configfile.Document{Version: 1, TLS: &configfile.TLS{Mode: "automatic", Hostname: "dav.example.com", Challenge: "dns", DNSProvider: "Cloudflare production"}}
	seed := app.ConfigImportSeed{Document: document, TLSID: "22222222-2222-4222-8222-222222222222", Timestamp: stamp}
	if _, err := NewConfigRepository(database).Import(ctx, seed); err != nil {
		t.Fatal(err)
	}
	var challenge, storedProvider string
	if err := database.QueryRow(`SELECT challenge, dns_provider_id FROM tls_profiles`).Scan(&challenge, &storedProvider); err != nil {
		t.Fatal(err)
	}
	if challenge != "dns" || storedProvider != string(providerID) {
		t.Fatalf("challenge=%q provider=%q", challenge, storedProvider)
	}

	missing := configfile.Document{Version: 1, TLS: &configfile.TLS{Mode: "automatic", Hostname: "dav.example.com", Challenge: "dns", DNSProvider: "missing"}}
	if _, err := NewConfigRepository(database).Import(ctx, app.ConfigImportSeed{Document: missing, TLSID: "33333333-3333-4333-8333-333333333333", Timestamp: stamp}); !errors.Is(err, app.ErrConfigImportConflict) {
		t.Fatalf("missing provider error = %v", err)
	}
}
