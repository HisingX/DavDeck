package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"davdeck.dev/davdeck/core/internal/domain"
)

type SQLiteTLSRepository struct{ db *sql.DB }

func NewTLSRepository(db *sql.DB) *SQLiteTLSRepository { return &SQLiteTLSRepository{db: db} }

func (r *SQLiteTLSRepository) Get(ctx context.Context) (domain.TLSProfile, bool, error) {
	profile, err := scanTLSProfile(r.db.QueryRowContext(ctx, `SELECT id, mode, hostname, certificate_path, private_key_path, created_at, updated_at FROM tls_profiles ORDER BY created_at, id LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TLSProfile{}, false, nil
	}
	return profile, err == nil, err
}

func (r *SQLiteTLSRepository) Save(ctx context.Context, profile domain.TLSProfile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM tls_profiles WHERE id <> ?`, profile.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tls_profiles(id, mode, hostname, certificate_path, private_key_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET mode = excluded.mode, hostname = excluded.hostname, certificate_path = excluded.certificate_path, private_key_path = excluded.private_key_path, updated_at = excluded.updated_at`,
		profile.ID, profile.Mode, profile.Hostname, profile.CertificatePath, profile.PrivateKeyPath, profile.CreatedAt.String(), profile.UpdatedAt.String())
	if err != nil {
		return err
	}
	return tx.Commit()
}

func scanTLSProfile(row scanner) (domain.TLSProfile, error) {
	var profile domain.TLSProfile
	var id, mode, created, updated string
	if err := row.Scan(&id, &mode, &profile.Hostname, &profile.CertificatePath, &profile.PrivateKeyPath, &created, &updated); err != nil {
		return domain.TLSProfile{}, err
	}
	parsedID, err := domain.ParseID(id)
	if err != nil {
		return domain.TLSProfile{}, fmt.Errorf("parse TLS profile id: %w", err)
	}
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.TLSProfile{}, fmt.Errorf("parse TLS profile created_at: %w", err)
	}
	updatedAt, err := domain.ParseTimestamp(updated)
	if err != nil {
		return domain.TLSProfile{}, fmt.Errorf("parse TLS profile updated_at: %w", err)
	}
	profile.ID, profile.Mode, profile.CreatedAt, profile.UpdatedAt = parsedID, domain.TLSMode(mode), createdAt, updatedAt
	if err := profile.Validate(); err != nil {
		return domain.TLSProfile{}, err
	}
	return profile, nil
}
