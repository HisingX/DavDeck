// Package dnsprovider contains the provider-specific, secret-free mapping used
// by the application and Caddy compiler. It never performs provider API calls.
package dnsprovider

import (
	"fmt"
	"strings"

	"davdeck.dev/davdeck/core/internal/domain"
)

// Adapter describes how one provider maps encrypted application secrets to
// Caddy's DNS module configuration. The returned Caddy config contains only
// environment placeholders, never secret values.
type Adapter interface {
	Type() domain.DNSProviderType
	ValidateSecret(domain.DNSProviderSecret) error
	CaddyProvider(domain.ID) map[string]string
	Environment(domain.ID, domain.DNSProviderSecret) map[string]string
}

type adapter struct {
	typeName    domain.DNSProviderType
	required    []string
	optional    []string
	caddyFields map[string]string
}

func (a adapter) Type() domain.DNSProviderType { return a.typeName }

func (a adapter) ValidateSecret(secret domain.DNSProviderSecret) error {
	allowed := make(map[string]struct{}, len(a.required)+len(a.optional))
	for _, field := range append(append([]string(nil), a.required...), a.optional...) {
		allowed[field] = struct{}{}
	}
	for field, value := range secret {
		if _, ok := allowed[field]; !ok {
			return fmt.Errorf("unsupported %s credential field %q", a.typeName, field)
		}
		if value == "" || value != strings.TrimSpace(value) || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			return fmt.Errorf("%s credential field %q is empty or contains control characters", a.typeName, field)
		}
		if len([]byte(value)) > 4096 {
			return fmt.Errorf("%s credential field %q is too long", a.typeName, field)
		}
	}
	for _, field := range a.required {
		if secret[field] == "" {
			return fmt.Errorf("%s credential field %q is required", a.typeName, field)
		}
	}
	return nil
}

func (a adapter) CaddyProvider(id domain.ID) map[string]string {
	result := make(map[string]string, len(a.caddyFields)+1)
	result["name"] = string(a.typeName)
	for caddyField, secretField := range a.caddyFields {
		result[caddyField] = "{env." + EnvironmentName(id, secretField) + "}"
	}
	return result
}

func (a adapter) Environment(id domain.ID, secret domain.DNSProviderSecret) map[string]string {
	result := make(map[string]string, len(secret))
	for field, value := range secret {
		result[EnvironmentName(id, field)] = value
	}
	return result
}

// EnvironmentName returns a stable Caddy process environment variable name.
func EnvironmentName(id domain.ID, field string) string {
	return "DAVDECK_DNS_" + strings.ToUpper(strings.ReplaceAll(string(id), "-", "")) + "_" + strings.ToUpper(strings.ReplaceAll(field, "-", "_"))
}

var adapters = map[domain.DNSProviderType]Adapter{
	domain.DNSProviderCloudflare: adapter{
		typeName:    domain.DNSProviderCloudflare,
		required:    []string{"api_token"},
		caddyFields: map[string]string{"api_token": "api_token"},
	},
	domain.DNSProviderTencentCloud: adapter{
		typeName:    domain.DNSProviderTencentCloud,
		required:    []string{"secret_id", "secret_key"},
		caddyFields: map[string]string{"secretid": "secret_id", "secretkey": "secret_key"},
	},
	domain.DNSProviderDNSPod: adapter{
		typeName:    domain.DNSProviderDNSPod,
		required:    []string{"api_token"},
		caddyFields: map[string]string{"api_token": "api_token"},
	},
	domain.DNSProviderAliDNS: adapter{
		typeName:    domain.DNSProviderAliDNS,
		required:    []string{"access_key_id", "access_key_secret"},
		optional:    []string{"security_token"},
		caddyFields: map[string]string{"access_key_id": "access_key_id", "access_key_secret": "access_key_secret", "security_token": "security_token"},
	},
}

// For returns the built-in adapter for a supported provider.
func For(provider domain.DNSProviderType) (Adapter, bool) {
	adapter, ok := adapters[provider]
	return adapter, ok
}
