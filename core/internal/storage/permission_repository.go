package storage

import (
	"context"
	"database/sql"
	"fmt"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type SQLitePermissionRepository struct{ db *sql.DB }

func NewPermissionRepository(db *sql.DB) *SQLitePermissionRepository {
	return &SQLitePermissionRepository{db: db}
}

func (r *SQLitePermissionRepository) ListByShare(ctx context.Context, shareID domain.ID) ([]domain.SharePermission, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT share_id, user_id, permission, created_at, updated_at FROM share_permissions WHERE share_id = ? ORDER BY user_id`, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.SharePermission, 0)
	for rows.Next() {
		value, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *SQLitePermissionRepository) Set(ctx context.Context, value domain.SharePermission) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO share_permissions(share_id, user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(share_id, user_id) DO UPDATE SET permission = excluded.permission, updated_at = excluded.updated_at`, value.ShareID, value.UserID, value.Permission, value.CreatedAt.String(), value.UpdatedAt.String())
	return err
}

func (r *SQLitePermissionRepository) Delete(ctx context.Context, shareID, userID domain.ID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM share_permissions WHERE share_id = ? AND user_id = ?`, shareID, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return app.ErrPermissionNotFound
	}
	return nil
}

func scanPermission(row scanner) (domain.SharePermission, error) {
	var value domain.SharePermission
	var shareID, userID, permission, created, updated string
	if err := row.Scan(&shareID, &userID, &permission, &created, &updated); err != nil {
		return domain.SharePermission{}, err
	}
	parsedShare, err := domain.ParseID(shareID)
	if err != nil {
		return domain.SharePermission{}, fmt.Errorf("parse permission share id: %w", err)
	}
	parsedUser, err := domain.ParseID(userID)
	if err != nil {
		return domain.SharePermission{}, fmt.Errorf("parse permission user id: %w", err)
	}
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.SharePermission{}, fmt.Errorf("parse permission created_at: %w", err)
	}
	updatedAt, err := domain.ParseTimestamp(updated)
	if err != nil {
		return domain.SharePermission{}, fmt.Errorf("parse permission updated_at: %w", err)
	}
	value.ShareID, value.UserID, value.Permission, value.CreatedAt, value.UpdatedAt = parsedShare, parsedUser, domain.Permission(permission), createdAt, updatedAt
	if err := value.Validate(); err != nil {
		return domain.SharePermission{}, err
	}
	return value, nil
}
