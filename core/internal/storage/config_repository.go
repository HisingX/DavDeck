package storage

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/configfile"
	"davdeck.dev/davdeck/core/internal/domain"
)

type SQLiteConfigRepository struct{ db *sql.DB }

func NewConfigRepository(db *sql.DB) *SQLiteConfigRepository {
	return &SQLiteConfigRepository{db: db}
}

func (r *SQLiteConfigRepository) Import(ctx context.Context, seed app.ConfigImportSeed) (app.ConfigImportResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return app.ConfigImportResult{}, err
	}
	defer tx.Rollback()
	result := app.ConfigImportResult{}
	if server := seed.Document.Server; server != nil {
		updated, err := tx.ExecContext(ctx, `UPDATE server_settings SET public_base_path = ?, http_port = ?, https_port = ?, runtime_mode = ?, updated_at = ?`, server.PublicBasePath, server.HTTPPort, server.HTTPSPort, server.RuntimeMode, seed.Timestamp.String())
		if err != nil {
			return result, err
		}
		count, err := updated.RowsAffected()
		if err != nil || count != 1 {
			return result, app.ErrConfigImportConflict
		}
		result.ServerUpdated = true
	}
	if tls := seed.Document.TLS; tls != nil {
		id, created := seed.TLSID, seed.Timestamp.String()
		var existingID, existingCreated string
		err := tx.QueryRowContext(ctx, `SELECT id, created_at FROM tls_profiles ORDER BY created_at, id LIMIT 1`).Scan(&existingID, &existingCreated)
		if err == nil {
			parsedID, parseErr := domain.ParseID(existingID)
			if parseErr != nil {
				return result, parseErr
			}
			id, created = parsedID, existingCreated
		} else if !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM tls_profiles WHERE id <> ?`, id); err != nil {
			return result, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO tls_profiles(id, mode, hostname, certificate_path, private_key_path, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET mode = excluded.mode, hostname = excluded.hostname, certificate_path = excluded.certificate_path, private_key_path = excluded.private_key_path, updated_at = excluded.updated_at`,
			id, tls.Mode, tls.Hostname, tls.CertificatePath, tls.PrivateKeyPath, created, seed.Timestamp.String())
		if err != nil {
			return result, err
		}
		result.TLSUpdated = true
	}
	userIDs := make(map[string]domain.ID)
	for _, imported := range seed.Document.Users {
		normalized := domain.NormalizeUsername(imported.Username)
		var existingID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username_normalized = ?`, normalized).Scan(&existingID)
		switch {
		case err == nil:
			id, parseErr := domain.ParseID(existingID)
			if parseErr != nil {
				return result, parseErr
			}
			if _, err := tx.ExecContext(ctx, `UPDATE users SET username = ?, enabled = ?, updated_at = ? WHERE id = ?`, imported.Username, *imported.Enabled, seed.Timestamp.String(), id); err != nil {
				return result, err
			}
			userIDs[normalized] = id
			result.UsersUpdated++
		case errors.Is(err, sql.ErrNoRows):
			id, hash := seed.UserIDs[normalized], seed.UserHashes[normalized]
			if id == "" || hash == "" {
				return result, app.ErrConfigImportConflict
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO users(id, username, username_normalized, password_hash, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, imported.Username, normalized, hash, *imported.Enabled, seed.Timestamp.String(), seed.Timestamp.String()); err != nil {
				return result, err
			}
			userIDs[normalized] = id
			result.UsersCreated++
			result.PasswordResetRequired = append(result.PasswordResetRequired, imported.Username)
		default:
			return result, err
		}
	}
	shareIDs := make(map[string]domain.ID)
	for _, imported := range seed.Document.Shares {
		var existingID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM shares WHERE slug = ?`, imported.Slug).Scan(&existingID)
		switch {
		case err == nil:
			id, parseErr := domain.ParseID(existingID)
			if parseErr != nil {
				return result, parseErr
			}
			if _, err := tx.ExecContext(ctx, `UPDATE shares SET name = ?, path = ?, enabled = ?, updated_at = ? WHERE id = ?`, imported.Name, imported.Path, *imported.Enabled, seed.Timestamp.String(), id); err != nil {
				return result, err
			}
			shareIDs[imported.Slug] = id
			result.SharesUpdated++
		case errors.Is(err, sql.ErrNoRows):
			id := seed.ShareIDs[imported.Slug]
			if id == "" {
				return result, app.ErrConfigImportConflict
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO shares(id, name, slug, path, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, imported.Name, imported.Slug, imported.Path, *imported.Enabled, seed.Timestamp.String(), seed.Timestamp.String()); err != nil {
				return result, err
			}
			shareIDs[imported.Slug] = id
			result.SharesCreated++
		default:
			return result, err
		}
	}
	for _, imported := range seed.Document.Shares {
		shareID := shareIDs[imported.Slug]
		usernames := make([]string, 0, len(imported.Permissions))
		for username := range imported.Permissions {
			usernames = append(usernames, username)
		}
		sort.Slice(usernames, func(i, j int) bool {
			return domain.NormalizeUsername(usernames[i]) < domain.NormalizeUsername(usernames[j])
		})
		for _, username := range usernames {
			normalized := domain.NormalizeUsername(username)
			userID := userIDs[normalized]
			if userID == "" {
				var existingID string
				if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username_normalized = ?`, normalized).Scan(&existingID); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return result, app.ErrConfigImportConflict
					}
					return result, err
				}
				parsed, err := domain.ParseID(existingID)
				if err != nil {
					return result, err
				}
				userID = parsed
			}
			permission, ok := configfile.ParsePermission(imported.Permissions[username])
			if !ok {
				return result, app.ErrConfigImportConflict
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO share_permissions(share_id, user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(share_id, user_id) DO UPDATE SET permission = excluded.permission, updated_at = excluded.updated_at`, shareID, userID, permission, seed.Timestamp.String(), seed.Timestamp.String())
			if err != nil {
				return result, err
			}
			result.PermissionsUpserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
