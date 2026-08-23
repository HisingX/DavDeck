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

type SQLiteUserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *SQLiteUserRepository { return &SQLiteUserRepository{db: db} }

func (r *SQLiteUserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, username, username_normalized, password_hash, enabled, created_at, updated_at FROM users ORDER BY username_normalized, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *SQLiteUserRepository) Get(ctx context.Context, id domain.ID) (domain.User, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx, `SELECT id, username, username_normalized, password_hash, enabled, created_at, updated_at FROM users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, app.ErrUserNotFound
	}
	return user, err
}

func (r *SQLiteUserRepository) Create(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO users(id, username, username_normalized, password_hash, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, user.ID, user.Username, user.UsernameNormalized, user.PasswordHash, user.Enabled, user.CreatedAt.String(), user.UpdatedAt.String())
	if err != nil && strings.Contains(err.Error(), "users.username_normalized") {
		return app.ErrUserAlreadyExists
	}
	return err
}

func (r *SQLiteUserRepository) Delete(ctx context.Context, id domain.ID) error {
	return expectUserAffected(r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id))
}

func (r *SQLiteUserRepository) SetEnabled(ctx context.Context, id domain.ID, enabled bool, updated domain.Timestamp) error {
	return expectUserAffected(r.db.ExecContext(ctx, `UPDATE users SET enabled = ?, updated_at = ? WHERE id = ?`, enabled, updated.String(), id))
}

func (r *SQLiteUserRepository) SetPasswordHash(ctx context.Context, id domain.ID, hash string, updated domain.Timestamp) error {
	return expectUserAffected(r.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, hash, updated.String(), id))
}

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (domain.User, error) {
	var user domain.User
	var id, created, updated string
	if err := row.Scan(&id, &user.Username, &user.UsernameNormalized, &user.PasswordHash, &user.Enabled, &created, &updated); err != nil {
		return domain.User{}, err
	}
	parsedID, err := domain.ParseID(id)
	if err != nil {
		return domain.User{}, fmt.Errorf("parse user id: %w", err)
	}
	createdAt, err := domain.ParseTimestamp(created)
	if err != nil {
		return domain.User{}, fmt.Errorf("parse user created_at: %w", err)
	}
	updatedAt, err := domain.ParseTimestamp(updated)
	if err != nil {
		return domain.User{}, fmt.Errorf("parse user updated_at: %w", err)
	}
	user.ID, user.CreatedAt, user.UpdatedAt = parsedID, createdAt, updatedAt
	return user, nil
}

func expectUserAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return app.ErrUserNotFound
	}
	return nil
}
