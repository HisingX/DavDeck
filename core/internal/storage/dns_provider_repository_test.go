package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestDNSProviderRepositoryStoresMetadataAndProtectsInUseCredentials(t *testing.T) {
	database, _, err := Open(context.Background(), filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	credential := domain.DNSProviderCredential{ID: "11111111-1111-4111-8111-111111111111", Name: "Cloudflare home", Provider: domain.DNSProviderCloudflare, AllowedZones: []string{"z.example", "a.example"}, CreatedAt: stamp, UpdatedAt: stamp}
	repository := NewDNSProviderRepository(database)
	if err := repository.Save(context.Background(), credential); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(context.Background(), credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != credential.Name || stored.Provider != credential.Provider || len(stored.AllowedZones) != 2 || stored.AllowedZones[0] != "a.example" {
		t.Fatalf("stored credential = %#v", stored)
	}
	if _, err := database.Exec(`INSERT INTO tls_profiles(id, mode, hostname, created_at, updated_at, challenge, dns_provider_id) VALUES (?, 'automatic', 'dav.example.com', ?, ?, 'dns', ?)`, "22222222-2222-4222-8222-222222222222", stamp.String(), stamp.String(), credential.ID); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), credential.ID); err != ErrDNSProviderInUse {
		t.Fatalf("delete error = %v, want %v", err, ErrDNSProviderInUse)
	}
	if _, err := database.Exec(`DELETE FROM tls_profiles`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), credential.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(context.Background(), credential.ID); err != ErrDNSProviderNotFound {
		t.Fatalf("get after delete error = %v, want %v", err, ErrDNSProviderNotFound)
	}
	var secretRows int
	if err := database.QueryRow(`SELECT COUNT(*) FROM dns_provider_secrets WHERE credential_id = ?`, credential.ID).Scan(&secretRows); err != nil {
		t.Fatal(err)
	}
}
