package domain

import (
	"strings"
	"unicode"
)

// User is a password-authenticated WebDAV principal. PasswordHash is excluded
// from JSON to reduce accidental secret exposure at interface boundaries.
type User struct {
	ID                 ID
	Username           string
	UsernameNormalized string
	PasswordHash       string `json:"-"`
	Enabled            bool
	CreatedAt          Timestamp
	UpdatedAt          Timestamp
}

// NormalizeUsername defines the stable MVP uniqueness key convention.
func NormalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (u User) Validate() error {
	if err := validateID("id", u.ID); err != nil {
		return err
	}
	if u.Username == "" || u.Username != strings.TrimSpace(u.Username) || containsControl(u.Username) {
		return invalid(CodeInvalidUsername, "username", "must be non-empty, trimmed, and contain no control characters")
	}
	if u.UsernameNormalized != NormalizeUsername(u.Username) {
		return invalid(CodeInvalidUsername, "username_normalized", "must match the normalized username")
	}
	if strings.TrimSpace(u.PasswordHash) == "" || containsControl(u.PasswordHash) {
		return invalid(CodeInvalidPasswordHash, "password_hash", "must contain a password hash")
	}
	return validateTimeRange("created_at", u.CreatedAt, "updated_at", u.UpdatedAt)
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}
