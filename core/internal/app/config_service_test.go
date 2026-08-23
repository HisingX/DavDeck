package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

type configSnapshots struct{ input domain.RuntimeConfigInput }

func (s configSnapshots) Snapshot(context.Context) (domain.RuntimeConfigInput, error) {
	return s.input, nil
}

type configImports struct {
	seed  ConfigImportSeed
	err   error
	calls int
}

func (r *configImports) Import(_ context.Context, seed ConfigImportSeed) (ConfigImportResult, error) {
	r.seed, r.calls = seed, r.calls+1
	if r.err != nil {
		return ConfigImportResult{}, r.err
	}
	return ConfigImportResult{UsersCreated: len(seed.Document.Users), SharesCreated: len(seed.Document.Shares), PasswordResetRequired: []string{"Alice"}}, nil
}

type configPaths struct{ err error }

func (p configPaths) ValidateSharePath(string) error { return p.err }

type configHasher struct {
	calls int
	input string
}

func (h *configHasher) Hash(value string) (string, error) {
	h.calls++
	h.input = value
	return "placeholder-hash", nil
}
func (*configHasher) Compare(string, string) error { return nil }

func TestConfigServiceImportValidatesThenSeedsNoSecretTransaction(t *testing.T) {
	repository, hasher := &configImports{}, &configHasher{}
	service := NewConfigService(configSnapshots{}, repository, configPaths{}, hasher, fixedID{}, fixedClock{})
	service.randomRead = func(target []byte) (int, error) {
		for index := range target {
			target[index] = byte(index + 1)
		}
		return len(target), nil
	}
	body := []byte(`version: 1
users:
  - username: Alice
    enabled: true
shares:
  - name: Documents
    slug: documents
    path: /srv/documents
    enabled: true
    permissions:
      Alice: read
`)
	result, err := service.Import(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || hasher.calls != 1 || hasher.input == "" || strings.Contains(string(body), hasher.input) {
		t.Fatalf("calls = %d, hasher = %#v", repository.calls, hasher)
	}
	if repository.seed.UserHashes["alice"] != "placeholder-hash" || !result.PendingApply || len(result.PasswordResetRequired) != 1 {
		t.Fatalf("seed = %#v, result = %#v", repository.seed, result)
	}
}

func TestConfigServiceRejectsSecretsPathsVersionsAndConflicts(t *testing.T) {
	repository := &configImports{}
	service := NewConfigService(configSnapshots{}, repository, configPaths{err: ErrSharePathNotFound}, &configHasher{}, fixedID{}, fixedClock{})
	secret := []byte("version: 1\npassword: do-not-store\n")
	if _, err := service.Import(context.Background(), secret); !hasCode(err, CodeConfigImportInvalid) || repository.calls != 0 {
		t.Fatalf("secret error = %v, calls = %d", err, repository.calls)
	}
	unsupported := []byte("version: 9\n")
	if _, err := service.Import(context.Background(), unsupported); !hasCode(err, CodeConfigVersionUnsupported) {
		t.Fatalf("version error = %v", err)
	}
	path := []byte("version: 1\nshares:\n  - name: Docs\n    slug: docs\n    path: /srv/docs\n    enabled: true\n")
	if _, err := service.Import(context.Background(), path); !hasCode(err, CodeConfigImportInvalid) || repository.calls != 0 {
		t.Fatalf("path error = %v, calls = %d", err, repository.calls)
	}
	service.paths = configPaths{}
	repository.err = ErrConfigImportConflict
	if _, err := service.Import(context.Background(), []byte("version: 1\n")); !hasCode(err, CodeConfigImportInvalid) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestConfigServiceExportNeverIncludesPasswordHash(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	settings := domain.ServerSettings{ID: testUserID, PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}
	user := domain.User{ID: testUserID, Username: "Alice", UsernameNormalized: "alice", PasswordHash: "SECRET_PASSWORD_HASH", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	service := NewConfigService(configSnapshots{domain.RuntimeConfigInput{ServerSettings: settings, Users: []domain.User{user}}}, &configImports{}, configPaths{}, &configHasher{}, fixedID{}, fixedClock{})
	body, err := service.Export(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "SECRET_PASSWORD_HASH") || strings.Contains(string(body), "password") {
		t.Fatalf("secret leaked: %s", body)
	}
}

func TestConfigServiceRandomFailureDoesNotWrite(t *testing.T) {
	repository := &configImports{}
	service := NewConfigService(configSnapshots{}, repository, configPaths{}, &configHasher{}, fixedID{}, fixedClock{})
	service.randomRead = func([]byte) (int, error) { return 0, errors.New("rng failed") }
	if _, err := service.Import(context.Background(), []byte("version: 1\nusers:\n  - username: Alice\n    enabled: true\n")); !hasCode(err, CodeDatabase) || repository.calls != 0 {
		t.Fatalf("error = %v, calls = %d", err, repository.calls)
	}
}
