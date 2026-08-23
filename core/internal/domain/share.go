package domain

import (
	"path"
	"regexp"
	"strings"
)

var shareSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Share is metadata for one physical filesystem authorization boundary.
type Share struct {
	ID        ID
	Name      string
	Slug      string
	Path      string
	Enabled   bool
	CreatedAt Timestamp
	UpdatedAt Timestamp
}

func (s Share) Validate() error {
	if err := validateID("id", s.ID); err != nil {
		return err
	}
	if s.Name == "" || s.Name != strings.TrimSpace(s.Name) || containsControl(s.Name) {
		return invalid(CodeInvalidShareName, "name", "must be non-empty, trimmed, and contain no control characters")
	}
	if !shareSlugPattern.MatchString(s.Slug) || s.Slug == "." || s.Slug == ".." {
		return invalid(CodeInvalidShareSlug, "slug", "must contain lowercase letters, digits, and single hyphens only")
	}
	if !IsAbsolutePath(s.Path) {
		return invalid(CodeInvalidSharePath, "path", "must be an absolute POSIX or drive-letter path")
	}
	return validateTimeRange("created_at", s.CreatedAt, "updated_at", s.UpdatedAt)
}

// IsAbsolutePath recognizes POSIX and Windows drive-letter absolute paths
// consistently on every build host. UNC paths remain unsupported in MVP.
func IsAbsolutePath(value string) bool {
	if value == "" || containsControl(value) {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return path.Clean(value) == value
	}
	if len(value) < 3 || !isASCIILetter(value[0]) || value[1] != ':' || (value[2] != '\\' && value[2] != '/') {
		return false
	}
	if strings.Contains(value[2:], "\\") && strings.Contains(value[2:], "/") {
		return false
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	if len(normalized) == 3 {
		return true
	}
	return path.Clean(normalized) == normalized
}

func isASCIILetter(value byte) bool {
	return (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}

// Permission is the stable, extensible MVP share access enum.
type Permission string

const (
	PermissionNone      Permission = "NONE"
	PermissionRead      Permission = "READ"
	PermissionReadWrite Permission = "READ_WRITE"
)

func (p Permission) Valid() bool {
	switch p {
	case PermissionNone, PermissionRead, PermissionReadWrite:
		return true
	default:
		return false
	}
}

// SharePermission associates one user with one share and an explicit access
// level. Absence of an association is semantically equivalent to NONE.
type SharePermission struct {
	ShareID    ID
	UserID     ID
	Permission Permission
	CreatedAt  Timestamp
	UpdatedAt  Timestamp
}

func (p SharePermission) Validate() error {
	if err := validateID("share_id", p.ShareID); err != nil {
		return err
	}
	if err := validateID("user_id", p.UserID); err != nil {
		return err
	}
	if !p.Permission.Valid() {
		return invalid(CodeInvalidPermission, "permission", "must be NONE, READ, or READ_WRITE")
	}
	return validateTimeRange("created_at", p.CreatedAt, "updated_at", p.UpdatedAt)
}
