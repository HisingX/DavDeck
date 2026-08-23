package domain

import (
	"path"
	"strings"
)

// RuntimeMode selects portable/development or installed-service operation.
type RuntimeMode string

const (
	RuntimeModePortable RuntimeMode = "portable"
	RuntimeModeService  RuntimeMode = "service"
)

func (m RuntimeMode) Valid() bool {
	return m == RuntimeModePortable || m == RuntimeModeService
}

// ServerSettings contains only product settings currently needed to describe
// managed listeners and the public WebDAV base path.
type ServerSettings struct {
	ID             ID
	PublicBasePath string
	HTTPPort       int
	HTTPSPort      int
	RuntimeMode    RuntimeMode
	CreatedAt      Timestamp
	UpdatedAt      Timestamp
}

func (s ServerSettings) Validate() error {
	if err := validateID("id", s.ID); err != nil {
		return err
	}
	if !validPublicBasePath(s.PublicBasePath) {
		return invalid(CodeInvalidBasePath, "public_base_path", "must be a canonical absolute URL path")
	}
	if !validPort(s.HTTPPort) {
		return invalid(CodeInvalidPort, "http_port", "must be between 1 and 65535")
	}
	if !validPort(s.HTTPSPort) {
		return invalid(CodeInvalidPort, "https_port", "must be between 1 and 65535")
	}
	if s.HTTPPort == s.HTTPSPort {
		return invalid(CodeInvalidPort, "https_port", "must differ from the HTTP port")
	}
	if !s.RuntimeMode.Valid() {
		return invalid(CodeInvalidRuntimeMode, "runtime_mode", "must be portable or service")
	}
	return validateTimeRange("created_at", s.CreatedAt, "updated_at", s.UpdatedAt)
}

func validPublicBasePath(value string) bool {
	return value != "" && strings.HasPrefix(value, "/") && !strings.ContainsAny(value, "?#\\") && !containsControl(value) && path.Clean(value) == value
}

func validPort(value int) bool { return value >= 1 && value <= 65535 }
