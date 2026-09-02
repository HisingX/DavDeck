package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type SQLiteRevisionRepository struct{ db *sql.DB }

func NewRevisionRepository(db *sql.DB) *SQLiteRevisionRepository {
	return &SQLiteRevisionRepository{db: db}
}

func (r *SQLiteRevisionRepository) Create(ctx context.Context, revision domain.ConfigRevision) (domain.ConfigRevision, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE revision_sequence SET next_number = next_number + 1 WHERE singleton = 1`); err != nil {
		return domain.ConfigRevision{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT next_number - 1 FROM revision_sequence WHERE singleton = 1`).Scan(&revision.Number); err != nil {
		return domain.ConfigRevision{}, err
	}
	if err := revision.Validate(); err != nil {
		return domain.ConfigRevision{}, err
	}
	stateSnapshot := revision.StateSnapshotJSON
	if stateSnapshot == nil {
		stateSnapshot = []byte{}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO config_revisions(id, revision_number, created_at, config_json, state_snapshot_json, config_hash, validation_status, apply_status, app_version, error_code, error_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, revision.ID, revision.Number, revision.CreatedAt.String(), revision.ConfigJSON, stateSnapshot, revision.ConfigHash, revision.ValidationStatus, revision.ApplyStatus, revision.AppVersion, revision.ErrorCode, revision.ErrorSummary)
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runtime_state SET desired_revision_id = ?, dirty = 0, updated_at = ? WHERE id = 1`, revision.ID, revision.CreatedAt.String()); err != nil {
		return domain.ConfigRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ConfigRevision{}, err
	}
	return revision, nil
}

func (r *SQLiteRevisionRepository) FindByHash(ctx context.Context, hash string) (domain.ConfigRevision, bool, error) {
	revision, err := scanRevision(r.db.QueryRowContext(ctx, revisionSelect+` WHERE config_hash = ? AND validation_status = 'VALID' AND state_snapshot_json <> '' ORDER BY revision_number DESC LIMIT 1`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConfigRevision{}, false, nil
	}
	if err != nil {
		return domain.ConfigRevision{}, false, err
	}
	return revision, true, nil
}

func (r *SQLiteRevisionRepository) SetDesired(ctx context.Context, id domain.ID, updated domain.Timestamp) error {
	result, err := r.db.ExecContext(ctx, `UPDATE runtime_state SET desired_revision_id = ?, dirty = 0, updated_at = ? WHERE id = 1 AND EXISTS (SELECT 1 FROM config_revisions WHERE id = ?)`, id, updated.String(), id)
	if err != nil {
		return err
	}
	return expectRevisionAffected(result)
}

func (r *SQLiteRevisionRepository) MarkApplied(ctx context.Context, id domain.ID, updated domain.Timestamp) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE config_revisions SET apply_status = 'APPLIED', error_code = '', error_summary = '' WHERE id = ? AND validation_status = 'VALID'`, id)
	if err != nil {
		return err
	}
	if err := expectRevisionAffected(result); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runtime_state SET desired_revision_id = ?, active_revision_id = ?, dirty = 0, updated_at = ? WHERE id = 1`, id, id, updated.String()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRevisionRepository) Delete(ctx context.Context, id domain.ID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var desired, active sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT desired_revision_id, active_revision_id FROM runtime_state WHERE id = 1`).Scan(&desired, &active); err != nil {
		return err
	}
	if active.Valid && active.String == string(id) {
		return app.ErrRevisionActive
	}
	if desired.Valid && desired.String == string(id) {
		return app.ErrRevisionDesired
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM config_revisions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if err := expectRevisionAffected(result); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRevisionRepository) MarkFailed(ctx context.Context, id domain.ID, code, summary string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE config_revisions SET apply_status = 'FAILED', error_code = ?, error_summary = ? WHERE id = ? AND validation_status = 'VALID'`, code, summary, id)
	if err != nil {
		return err
	}
	return expectRevisionAffected(result)
}

func (r *SQLiteRevisionRepository) MarkRestored(ctx context.Context, id domain.ID, updated domain.Timestamp) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE config_revisions SET apply_status = 'APPLIED', error_code = '', error_summary = '' WHERE id = ? AND validation_status = 'VALID'`, id)
	if err != nil {
		return err
	}
	if err := expectRevisionAffected(result); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runtime_state SET desired_revision_id = ?, active_revision_id = ?, dirty = 0, updated_at = ? WHERE id = 1`, id, id, updated.String()); err != nil {
		return err
	}
	return tx.Commit()
}

// RestoreState atomically replaces the authoritative SQLite state with the
// validated state represented by a complete revision snapshot. Runtime
// activation is deliberately handled by the application service before this
// transaction is committed.
func (r *SQLiteRevisionRepository) RestoreState(ctx context.Context, input domain.RuntimeConfigInput, id domain.ID, updated domain.Timestamp) error {
	if err := input.Validate(); err != nil {
		return err
	}
	expectedSnapshot, err := domain.MarshalConfigRevisionSnapshot(input)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var snapshot []byte
	if err := tx.QueryRowContext(ctx, `SELECT state_snapshot_json FROM config_revisions WHERE id = ? AND validation_status = 'VALID'`, id).Scan(&snapshot); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return app.ErrRevisionNotFound
		}
		return err
	}
	if len(snapshot) == 0 {
		return fmt.Errorf("revision %s has no state snapshot", id)
	}
	if !bytes.Equal(snapshot, expectedSnapshot) {
		return fmt.Errorf("revision %s state snapshot does not match the requested state", id)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM share_permissions`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM shares`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tls_profiles`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM server_settings`); err != nil {
		return err
	}
	settings := input.ServerSettings
	if _, err := tx.ExecContext(ctx, `INSERT INTO server_settings(id, public_base_path, http_port, https_port, runtime_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, settings.ID, settings.PublicBasePath, settings.HTTPPort, settings.HTTPSPort, settings.RuntimeMode, settings.CreatedAt.String(), settings.UpdatedAt.String()); err != nil {
		return err
	}
	for _, user := range input.Users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users(id, username, username_normalized, password_hash, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, user.ID, user.Username, user.UsernameNormalized, user.PasswordHash, user.Enabled, user.CreatedAt.String(), user.UpdatedAt.String()); err != nil {
			return err
		}
	}
	for _, item := range input.Shares {
		share := item.Share
		if _, err := tx.ExecContext(ctx, `INSERT INTO shares(id, name, slug, path, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, share.ID, share.Name, share.Slug, share.Path, share.Enabled, share.CreatedAt.String(), share.UpdatedAt.String()); err != nil {
			return err
		}
		for _, permission := range item.Permissions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO share_permissions(share_id, user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, permission.ShareID, permission.UserID, permission.Permission, permission.CreatedAt.String(), permission.UpdatedAt.String()); err != nil {
				return err
			}
		}
	}
	for _, provider := range input.DNSProviderCredentials {
		zonesJSON, err := json.Marshal(provider.AllowedZones)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO dns_provider_credentials(id, name, provider, allowed_zones_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET name = excluded.name, provider = excluded.provider, allowed_zones_json = excluded.allowed_zones_json, updated_at = excluded.updated_at`, provider.ID, provider.Name, provider.Provider, zonesJSON, provider.CreatedAt.String(), provider.UpdatedAt.String()); err != nil {
			return err
		}
	}
	if input.TLSProfile != nil {
		profile := input.TLSProfile
		challenge := profile.Challenge
		if challenge == "" {
			challenge = domain.TLSChallengeAuto
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tls_profiles(id, mode, hostname, certificate_path, private_key_path, created_at, updated_at, challenge, dns_provider_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, profile.ID, profile.Mode, profile.Hostname, profile.CertificatePath, profile.PrivateKeyPath, profile.CreatedAt.String(), profile.UpdatedAt.String(), challenge, profile.DNSProviderID); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE config_revisions SET apply_status = 'APPLIED', error_code = '', error_summary = '' WHERE id = ? AND validation_status = 'VALID'`, id)
	if err != nil {
		return err
	}
	if err := expectRevisionAffected(result); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runtime_state SET desired_revision_id = ?, active_revision_id = ?, dirty = 0, updated_at = ? WHERE id = 1`, id, id, updated.String()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRevisionRepository) List(ctx context.Context) ([]domain.ConfigRevision, error) {
	rows, err := r.db.QueryContext(ctx, revisionSelect+` ORDER BY revision_number DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ConfigRevision, 0)
	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, revision)
	}
	return result, rows.Err()
}

func (r *SQLiteRevisionRepository) Get(ctx context.Context, id domain.ID) (domain.ConfigRevision, error) {
	revision, err := scanRevision(r.db.QueryRowContext(ctx, revisionSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConfigRevision{}, app.ErrRevisionNotFound
	}
	return revision, err
}

func (r *SQLiteRevisionRepository) Active(ctx context.Context) (domain.ConfigRevision, bool, error) {
	revision, err := scanRevision(r.db.QueryRowContext(ctx, revisionSelect+` WHERE id = (SELECT active_revision_id FROM runtime_state WHERE id = 1)`))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConfigRevision{}, false, nil
	}
	if err != nil {
		return domain.ConfigRevision{}, false, err
	}
	return revision, true, nil
}

func (r *SQLiteRevisionRepository) State(ctx context.Context) (app.RevisionState, error) {
	var state app.RevisionState
	var desired, active sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT runtime_state.dirty, desired.revision_number, active.revision_number FROM runtime_state LEFT JOIN config_revisions desired ON desired.id = runtime_state.desired_revision_id LEFT JOIN config_revisions active ON active.id = runtime_state.active_revision_id WHERE runtime_state.id = 1`).Scan(&state.Dirty, &desired, &active)
	if err != nil {
		return app.RevisionState{}, err
	}
	if desired.Valid {
		value := uint64(desired.Int64)
		state.DesiredRevision = &value
	}
	if active.Valid {
		value := uint64(active.Int64)
		state.ActiveRevision = &value
	}
	state.Pending = state.Dirty || !sameRevision(state.DesiredRevision, state.ActiveRevision)
	return state, nil
}

const revisionSelect = `SELECT id, revision_number, created_at, config_json, state_snapshot_json, config_hash, validation_status, apply_status, app_version, error_code, error_summary FROM config_revisions`

func scanRevision(row scanner) (domain.ConfigRevision, error) {
	var revision domain.ConfigRevision
	var id, created string
	if err := row.Scan(&id, &revision.Number, &created, &revision.ConfigJSON, &revision.StateSnapshotJSON, &revision.ConfigHash, &revision.ValidationStatus, &revision.ApplyStatus, &revision.AppVersion, &revision.ErrorCode, &revision.ErrorSummary); err != nil {
		return domain.ConfigRevision{}, err
	}
	parsedID, err := domain.ParseID(id)
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	revision.ID, revision.CreatedAt = parsedID, createdAt
	if err := revision.Validate(); err != nil {
		return domain.ConfigRevision{}, fmt.Errorf("validate stored revision: %w", err)
	}
	return revision, nil
}

func expectRevisionAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return app.ErrRevisionNotFound
	}
	return nil
}
func sameRevision(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
