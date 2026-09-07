package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"davdeck.dev/davdeck/core/internal/domain"
)

type SnapshotRepository struct{ db *sql.DB }

func NewSnapshotRepository(db *sql.DB) *SnapshotRepository { return &SnapshotRepository{db: db} }

func (r *SnapshotRepository) Snapshot(ctx context.Context) (domain.RuntimeConfigInput, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	defer tx.Rollback()
	settings, err := scanSettings(tx.QueryRowContext(ctx, `SELECT id, public_base_path, http_port, https_port, runtime_mode, created_at, updated_at FROM server_settings LIMIT 1`))
	if err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	dnsProviders, err := scanDNSProviderCredentials(ctx, tx)
	if err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	tlsProfile, hasTLS, err := scanOptionalTLSProfile(tx.QueryRowContext(ctx, `SELECT id, mode, hostname, certificate_path, private_key_path, created_at, updated_at, challenge, dns_provider_id FROM tls_profiles ORDER BY created_at, id LIMIT 1`))
	if err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	userRows, err := tx.QueryContext(ctx, `SELECT id, username, username_normalized, password_hash, enabled, created_at, updated_at FROM users ORDER BY username_normalized, id`)
	if err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	users := make([]domain.User, 0)
	for userRows.Next() {
		user, err := scanUser(userRows)
		if err != nil {
			userRows.Close()
			return domain.RuntimeConfigInput{}, err
		}
		users = append(users, user)
	}
	if err := userRows.Close(); err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	shareRows, err := tx.QueryContext(ctx, `SELECT id, name, slug, path, enabled, created_at, updated_at FROM shares ORDER BY slug, id`)
	if err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	shares := make([]domain.ShareWithPermissions, 0)
	for shareRows.Next() {
		share, err := scanShare(shareRows)
		if err != nil {
			shareRows.Close()
			return domain.RuntimeConfigInput{}, err
		}
		shares = append(shares, domain.ShareWithPermissions{Share: share})
	}
	if err := shareRows.Close(); err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	for index := range shares {
		rows, err := tx.QueryContext(ctx, `SELECT share_id, user_id, permission, created_at, updated_at FROM share_permissions WHERE share_id = ? ORDER BY user_id`, shares[index].Share.ID)
		if err != nil {
			return domain.RuntimeConfigInput{}, err
		}
		for rows.Next() {
			permission, err := scanPermission(rows)
			if err != nil {
				rows.Close()
				return domain.RuntimeConfigInput{}, err
			}
			shares[index].Permissions = append(shares[index].Permissions, permission)
		}
		if err := rows.Close(); err != nil {
			return domain.RuntimeConfigInput{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.RuntimeConfigInput{}, err
	}
	result := domain.RuntimeConfigInput{ServerSettings: settings, DNSProviderCredentials: dnsProviders, Users: users, Shares: shares}
	if hasTLS {
		result.TLSProfile = &tlsProfile
	}
	return result, nil
}

func scanDNSProviderCredentials(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) ([]domain.DNSProviderCredential, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT id, name, provider, allowed_zones_json, created_at, updated_at FROM dns_provider_credentials ORDER BY name, id`)
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

func scanOptionalTLSProfile(row scanner) (domain.TLSProfile, bool, error) {
	profile, err := scanTLSProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TLSProfile{}, false, nil
	}
	return profile, err == nil, err
}

func scanSettings(row scanner) (domain.ServerSettings, error) {
	var settings domain.ServerSettings
	var id, mode, created, updated string
	if err := row.Scan(&id, &settings.PublicBasePath, &settings.HTTPPort, &settings.HTTPSPort, &mode, &created, &updated); err != nil {
		return domain.ServerSettings{}, err
	}
	parsedID, err := domain.ParseID(id)
	if err != nil {
		return domain.ServerSettings{}, fmt.Errorf("parse settings id: %w", err)
	}
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.ServerSettings{}, err
	}
	updatedAt, err := domain.ParseTimestamp(updated)
	if err != nil {
		return domain.ServerSettings{}, err
	}
	settings.ID, settings.RuntimeMode, settings.CreatedAt, settings.UpdatedAt = parsedID, domain.RuntimeMode(mode), createdAt, updatedAt
	if err := settings.Validate(); err != nil {
		return domain.ServerSettings{}, err
	}
	return settings, nil
}
