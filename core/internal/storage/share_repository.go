package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type SQLiteShareRepository struct{ db *sql.DB }

func NewShareRepository(db *sql.DB) *SQLiteShareRepository { return &SQLiteShareRepository{db: db} }

func (r *SQLiteShareRepository) List(ctx context.Context) ([]domain.Share, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, slug, path, enabled, created_at, updated_at FROM shares ORDER BY slug, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shares := make([]domain.Share, 0)
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, rows.Err()
}

func (r *SQLiteShareRepository) Get(ctx context.Context, id domain.ID) (domain.Share, error) {
	share, err := scanShare(r.db.QueryRowContext(ctx, `SELECT id, name, slug, path, enabled, created_at, updated_at FROM shares WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Share{}, app.ErrShareNotFound
	}
	return share, err
}

func (r *SQLiteShareRepository) Create(ctx context.Context, share domain.Share) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO shares(id, name, slug, path, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, share.ID, share.Name, share.Slug, share.Path, share.Enabled, share.CreatedAt.String(), share.UpdatedAt.String())
	if err != nil && strings.Contains(err.Error(), "shares.slug") {
		return app.ErrShareAlreadyExists
	}
	return err
}

func (r *SQLiteShareRepository) Update(ctx context.Context, share domain.Share) error {
	result, err := r.db.ExecContext(ctx, `UPDATE shares SET name = ?, slug = ?, path = ?, enabled = ?, updated_at = ? WHERE id = ?`, share.Name, share.Slug, share.Path, share.Enabled, share.UpdatedAt.String(), share.ID)
	if err != nil && strings.Contains(err.Error(), "shares.slug") {
		return app.ErrShareAlreadyExists
	}
	return expectShareAffected(result, err)
}

func (r *SQLiteShareRepository) Delete(ctx context.Context, id domain.ID) error {
	return expectShareAffected(r.db.ExecContext(ctx, `DELETE FROM shares WHERE id = ?`, id))
}

func scanShare(row scanner) (domain.Share, error) {
	var share domain.Share
	var id, created, updated string
	if err := row.Scan(&id, &share.Name, &share.Slug, &share.Path, &share.Enabled, &created, &updated); err != nil {
		return domain.Share{}, err
	}
	parsedID, err := domain.ParseID(id)
	if err != nil {
		return domain.Share{}, fmt.Errorf("parse share id: %w", err)
	}
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.Share{}, fmt.Errorf("parse share created_at: %w", err)
	}
	updatedAt, err := domain.ParseTimestamp(updated)
	if err != nil {
		return domain.Share{}, fmt.Errorf("parse share updated_at: %w", err)
	}
	share.ID, share.CreatedAt, share.UpdatedAt = parsedID, createdAt, updatedAt
	return share, nil
}

func expectShareAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return app.ErrShareNotFound
	}
	return nil
}
