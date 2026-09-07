package domain

import (
	"errors"
	"strings"
)

var (
	ErrDNSProviderNotFound = errors.New("DNS provider credential not found")
	ErrDNSProviderInUse    = errors.New("DNS provider credential is in use")
)

// DNSProviderType identifies a DNS provider module compiled into Caddy.
type DNSProviderType string

const (
	DNSProviderCloudflare   DNSProviderType = "cloudflare"
	DNSProviderTencentCloud DNSProviderType = "tencentcloud"
	DNSProviderDNSPod       DNSProviderType = "dnspod"
	DNSProviderAliDNS       DNSProviderType = "alidns"
)

func (p DNSProviderType) Valid() bool {
	switch p {
	case DNSProviderCloudflare, DNSProviderTencentCloud, DNSProviderDNSPod, DNSProviderAliDNS:
		return true
	default:
		return false
	}
}

// DNSProviderCredential contains public metadata for a DNS credential.
// Authentication material is deliberately stored outside this entity and is
// never part of runtime config revisions or normal API responses.
type DNSProviderCredential struct {
	ID           ID              `json:"id"`
	Name         string          `json:"name"`
	Provider     DNSProviderType `json:"provider"`
	AllowedZones []string        `json:"allowed_zones,omitempty"`
	CreatedAt    Timestamp       `json:"created_at"`
	UpdatedAt    Timestamp       `json:"updated_at"`
}

func (c DNSProviderCredential) Validate() error {
	if err := validateID("id", c.ID); err != nil {
		return err
	}
	if c.Name == "" || c.Name != strings.TrimSpace(c.Name) || containsControl(c.Name) || len([]byte(c.Name)) > 128 {
		return invalid(CodeInvalidDNSProviderCredential, "name", "must be a non-empty name without control characters")
	}
	if !c.Provider.Valid() {
		return invalid(CodeInvalidDNSProvider, "provider", "must be cloudflare, tencentcloud, dnspod, or alidns")
	}
	seen := make(map[string]struct{}, len(c.AllowedZones))
	for _, zone := range c.AllowedZones {
		if !validDNSZone(zone) {
			return invalid(CodeInvalidDNSZone, "allowed_zones", "must contain valid DNS zone names")
		}
		if _, exists := seen[zone]; exists {
			return invalid(CodeInvalidDNSZone, "allowed_zones", "must not contain duplicate zones")
		}
		seen[zone] = struct{}{}
	}
	return validateTimeRange("created_at", c.CreatedAt, "updated_at", c.UpdatedAt)
}

func validDNSZone(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, "*") {
		return false
	}
	return validHostname(value)
}

// DNSProviderSecret is the provider-specific authentication material held by
// SecretStore. It cannot be marshaled accidentally into API responses or
// revision data; the encrypted store uses an explicit internal conversion.
type DNSProviderSecret map[string]string

func (DNSProviderSecret) MarshalJSON() ([]byte, error) {
	return nil, errors.New("DNS provider secrets must not be serialized")
}
