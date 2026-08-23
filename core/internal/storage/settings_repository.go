package storage

import (
	"context"
	"database/sql"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type SQLiteServerSettingsRepository struct{ db *sql.DB }

func NewServerSettingsRepository(db *sql.DB) *SQLiteServerSettingsRepository {
	return &SQLiteServerSettingsRepository{db: db}
}

func (r *SQLiteServerSettingsRepository) Get(ctx context.Context) (domain.ServerSettings, error) {
	return scanSettings(r.db.QueryRowContext(ctx, `SELECT id, public_base_path, http_port, https_port, runtime_mode, created_at, updated_at FROM server_settings LIMIT 1`))
}

func (r *SQLiteServerSettingsRepository) UpdatePorts(ctx context.Context, httpPort, httpsPort int, updatedAt domain.Timestamp) (domain.ServerSettings, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE server_settings SET http_port = ?, https_port = ?, updated_at = ?`, httpPort, httpsPort, updatedAt.String())
	if err != nil {
		return domain.ServerSettings{}, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return domain.ServerSettings{}, app.ErrServerSettingsNotFound
	}
	return r.Get(ctx)
}
