package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestLocalEncryptedSecretStoreRoundTripsWithoutPlaintextSQLite(t *testing.T) {
	database, _, err := Open(context.Background(), filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	keyPath := filepath.Join(t.TempDir(), "davdeck.secret.key")
	store, err := NewLocalEncryptedSecretStore(database, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	stamp := mustTestTimestamp(t)
	credentialID := domain.ID("11111111-1111-4111-8111-111111111111")
	if _, err := database.Exec(`INSERT INTO dns_provider_credentials(id, name, provider, created_at, updated_at) VALUES (?, 'test', 'cloudflare', ?, ?)`, credentialID, stamp.String(), stamp.String()); err != nil {
		t.Fatal(err)
	}
	secret := domain.DNSProviderSecret{"api_token": "cfut_super_secret_value"}
	if err := store.Put(context.Background(), credentialID, secret, stamp); err != nil {
		t.Fatal(err)
	}
	var ciphertext []byte
	if err := database.QueryRow(`SELECT ciphertext FROM dns_provider_secrets WHERE credential_id = ?`, credentialID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ciphertext), "cfut_super_secret_value") {
		t.Fatal("ciphertext contains plaintext provider secret")
	}
	got, configured, err := store.Get(context.Background(), credentialID)
	if err != nil || !configured || got["api_token"] != secret["api_token"] {
		t.Fatalf("secret = %#v, configured = %t, err = %v", got, configured, err)
	}
	second, err := NewLocalEncryptedSecretStore(database, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	got, configured, err = second.Get(context.Background(), credentialID)
	if err != nil || !configured || got["api_token"] != secret["api_token"] {
		t.Fatalf("second read = %#v, configured = %t, err = %v", got, configured, err)
	}
	assertSecretKeyPermissions(t, keyPath)
}

func mustTestTimestamp(t *testing.T) domain.Timestamp {
	t.Helper()
	stamp, err := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return stamp
}
