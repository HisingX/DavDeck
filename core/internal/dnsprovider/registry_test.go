package dnsprovider

import (
	"strings"
	"testing"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestProviderAdaptersUseOnlyEnvironmentPlaceholders(t *testing.T) {
	for _, provider := range []domain.DNSProviderType{domain.DNSProviderCloudflare, domain.DNSProviderTencentCloud, domain.DNSProviderDNSPod, domain.DNSProviderAliDNS} {
		adapter, ok := For(provider)
		if !ok {
			t.Fatalf("adapter %q not found", provider)
		}
		id := domain.ID("11111111-1111-4111-8111-111111111111")
		config := adapter.CaddyProvider(id)
		for field, value := range config {
			if field == "name" {
				continue
			}
			if !strings.HasPrefix(value, "{env.DAVDECK_DNS_") || !strings.HasSuffix(value, "}") {
				t.Errorf("%s.%s = %q is not an environment placeholder", provider, field, value)
			}
		}
	}
}

func TestProviderAdaptersValidateRequiredSecrets(t *testing.T) {
	for _, provider := range []domain.DNSProviderType{domain.DNSProviderCloudflare, domain.DNSProviderTencentCloud, domain.DNSProviderDNSPod, domain.DNSProviderAliDNS} {
		adapter, _ := For(provider)
		if err := adapter.ValidateSecret(nil); err == nil {
			t.Errorf("%s accepted an empty secret", provider)
		}
	}
	adapter, _ := For(domain.DNSProviderCloudflare)
	if err := adapter.ValidateSecret(domain.DNSProviderSecret{"api_token": "token"}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ValidateSecret(domain.DNSProviderSecret{"api_token": "token", "password": "unexpected"}); err == nil {
		t.Fatal("accepted an unknown secret field")
	}
	dnsPod, _ := For(domain.DNSProviderDNSPod)
	if err := dnsPod.ValidateSecret(domain.DNSProviderSecret{"api_token": "app-id,app-token"}); err != nil {
		t.Fatal(err)
	}
	if err := dnsPod.ValidateSecret(domain.DNSProviderSecret{"api_token": "app-id"}); err != nil {
		t.Fatal(err)
	}
}
