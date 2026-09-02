package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"davdeck.dev/davdeck/core/internal/domain"
)

var (
	ErrDNSProviderNotFound = domain.ErrDNSProviderNotFound
	ErrDNSProviderInUse    = domain.ErrDNSProviderInUse
)

// SQLiteDNSProviderRepository stores only public credential metadata. Secret
// material is handled by LocalEncryptedSecretStore.
type SQLiteDNSProviderRepository struct{ db *sql.DB }

func NewDNSProviderRepository(db *sql.DB) *SQLiteDNSProviderRepository {
	return &SQLiteDNSProviderRepository{db: db}
}

func (r *SQLiteDNSProviderRepository) List(ctx context.Context) ([]domain.DNSProviderCredential, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, provider, allowed_zones_json, created_at, updated_at FROM dns_provider_credentials ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.DNSProviderCredential, 0)
	for rows.Next() {
		credential, err := scanDNSProviderCredential(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, credential)
	}
	return result, rows.Err()
}

func (r *SQLiteDNSProviderRepository) Get(ctx context.Context, id domain.ID) (domain.DNSProviderCredential, error) {
	credential, err := scanDNSProviderCredential(r.db.QueryRowContext(ctx, `SELECT id, name, provider, allowed_zones_json, created_at, updated_at FROM dns_provider_credentials WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DNSProviderCredential{}, ErrDNSProviderNotFound
	}
	return credential, err
}

func (r *SQLiteDNSProviderRepository) Save(ctx context.Context, credential domain.DNSProviderCredential) error {
	if err := credential.Validate(); err != nil {
		return err
	}
	zones := append([]string(nil), credential.AllowedZones...)
	sort.Strings(zones)
	zonesJSON, err := json.Marshal(zones)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO dns_provider_credentials(id, name, provider, allowed_zones_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, provider = excluded.provider, allowed_zones_json = excluded.allowed_zones_json, updated_at = excluded.updated_at`,
		credential.ID, credential.Name, credential.Provider, zonesJSON, credential.CreatedAt.String(), credential.UpdatedAt.String())
	return err
}

func (r *SQLiteDNSProviderRepository) Delete(ctx context.Context, id domain.ID) error {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tls_profiles WHERE dns_provider_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrDNSProviderInUse
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM dns_provider_credentials WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected != 1 {
		return ErrDNSProviderNotFound
	}
	return nil
}

func scanDNSProviderCredential(row scanner) (domain.DNSProviderCredential, error) {
	var credential domain.DNSProviderCredential
	var id, provider, zonesJSON, created, updated string
	if err := row.Scan(&id, &credential.Name, &provider, &zonesJSON, &created, &updated); err != nil {
		return domain.DNSProviderCredential{}, err
	}
	parsedID, err := domain.ParseID(id)
	if err != nil {
		return domain.DNSProviderCredential{}, fmt.Errorf("parse DNS provider id: %w", err)
	}
	if err := json.Unmarshal([]byte(zonesJSON), &credential.AllowedZones); err != nil {
		return domain.DNSProviderCredential{}, fmt.Errorf("decode DNS provider zones: %w", err)
	}
	sort.Strings(credential.AllowedZones)
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.DNSProviderCredential{}, fmt.Errorf("parse DNS provider created_at: %w", err)
	}
	updatedAt, err := domain.ParseTimestamp(updated)
	if err != nil {
		return domain.DNSProviderCredential{}, fmt.Errorf("parse DNS provider updated_at: %w", err)
	}
	credential.ID, credential.Provider = parsedID, domain.DNSProviderType(provider)
	credential.CreatedAt, credential.UpdatedAt = createdAt, updatedAt
	if err := credential.Validate(); err != nil {
		return domain.DNSProviderCredential{}, err
	}
	return credential, nil
}
