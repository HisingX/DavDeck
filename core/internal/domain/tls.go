package domain

import "strings"

// TLSMode selects one managed DavDeck TLS strategy.
type TLSMode string

const (
	TLSModeAutomatic TLSMode = "automatic"
	TLSModeInternal  TLSMode = "internal"
	TLSModeCustom    TLSMode = "custom"
)

func (m TLSMode) Valid() bool {
	switch m {
	case TLSModeAutomatic, TLSModeInternal, TLSModeCustom:
		return true
	default:
		return false
	}
}

// TLSProfile contains product-level TLS intent, never private-key contents.
type TLSProfile struct {
	ID              ID        `json:"id"`
	Mode            TLSMode   `json:"mode"`
	Hostname        string    `json:"hostname"`
	CertificatePath string    `json:"certificate_path,omitempty"`
	PrivateKeyPath  string    `json:"private_key_path,omitempty"`
	CreatedAt       Timestamp `json:"created_at"`
	UpdatedAt       Timestamp `json:"updated_at"`
}

func (p TLSProfile) Validate() error {
	if err := validateID("id", p.ID); err != nil {
		return err
	}
	if !p.Mode.Valid() {
		return invalid(CodeInvalidTLSMode, "mode", "must be automatic, internal, or custom")
	}
	if !validHostname(p.Hostname) {
		return invalid(CodeInvalidHostname, "hostname", "must be a hostname without scheme, port, or path")
	}
	if p.Mode == TLSModeCustom {
		if !IsAbsolutePath(p.CertificatePath) {
			return invalid(CodeInvalidCertificate, "certificate_path", "must be an absolute path in custom mode")
		}
		if !IsAbsolutePath(p.PrivateKeyPath) {
			return invalid(CodeInvalidPrivateKey, "private_key_path", "must be an absolute path in custom mode")
		}
	} else if p.CertificatePath != "" || p.PrivateKeyPath != "" {
		return invalid(CodeInvalidTLSMode, "mode", "automatic and internal modes must not include custom certificate paths")
	}
	return validateTimeRange("created_at", p.CreatedAt, "updated_at", p.UpdatedAt)
}

func validHostname(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || containsControl(value) || len(value) > 253 {
		return false
	}
	if strings.ContainsAny(value, "/\\:@") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-') {
				return false
			}
		}
	}
	return true
}
