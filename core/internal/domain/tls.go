package domain

import "strings"

// TLSMode selects one managed DavDeck TLS strategy.
type TLSMode string

const (
	TLSModeAutomatic TLSMode = "automatic"
	TLSModeInternal  TLSMode = "internal"
	TLSModeCustom    TLSMode = "custom"
)

// TLSChallenge selects the ACME validation method for automatic TLS.
type TLSChallenge string

const (
	TLSChallengeAuto TLSChallenge = "auto"
	TLSChallengeDNS  TLSChallenge = "dns"
)

func (c TLSChallenge) Valid() bool { return c == TLSChallengeAuto || c == TLSChallengeDNS }

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
	ID              ID           `json:"id"`
	Mode            TLSMode      `json:"mode"`
	Hostname        string       `json:"hostname"`
	Challenge       TLSChallenge `json:"challenge,omitempty"`
	DNSProviderID   *ID          `json:"dns_provider_id,omitempty"`
	CertificatePath string       `json:"certificate_path,omitempty"`
	PrivateKeyPath  string       `json:"private_key_path,omitempty"`
	CreatedAt       Timestamp    `json:"created_at"`
	UpdatedAt       Timestamp    `json:"updated_at"`
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
	challenge := p.Challenge
	if challenge == "" {
		challenge = TLSChallengeAuto
	}
	if !challenge.Valid() {
		return invalid(CodeInvalidTLSChallenge, "challenge", "must be auto or dns")
	}
	if challenge == TLSChallengeDNS && p.Mode != TLSModeAutomatic {
		return invalid(CodeInvalidTLSChallenge, "challenge", "DNS challenge is available only for automatic TLS")
	}
	if challenge == TLSChallengeDNS {
		if p.DNSProviderID == nil {
			return invalid(CodeInvalidDNSProvider, "dns_provider_id", "is required for DNS challenge")
		}
		if err := validateID("dns_provider_id", *p.DNSProviderID); err != nil {
			return err
		}
	} else if p.DNSProviderID != nil {
		return invalid(CodeInvalidTLSChallenge, "dns_provider_id", "must be omitted unless DNS challenge is selected")
	}
	if strings.HasPrefix(p.Hostname, "*.") && challenge != TLSChallengeDNS {
		return invalid(CodeInvalidTLSChallenge, "challenge", "wildcard hostnames require DNS challenge")
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
	labels := strings.Split(value, ".")
	if len(labels) > 1 && labels[0] == "*" {
		labels = labels[1:]
		if len(labels) == 0 {
			return false
		}
	}
	for _, label := range labels {
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
