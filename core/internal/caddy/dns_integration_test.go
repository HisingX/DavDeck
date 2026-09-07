package caddy

import (
	"context"
	"os"
	"strings"
	"testing"

	"davdeck.dev/davdeck/core/internal/dnsprovider"
	"davdeck.dev/davdeck/core/internal/domain"
)

func TestDNSChallengeConfigValidatesWithPinnedCaddy(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run pinned DNS provider validation")
	}
	for _, testCase := range []struct {
		provider domain.DNSProviderType
		secret   domain.DNSProviderSecret
	}{
		{provider: domain.DNSProviderCloudflare, secret: domain.DNSProviderSecret{"api_token": "cfut_" + strings.Repeat("a", 32)}},
		{provider: domain.DNSProviderTencentCloud, secret: domain.DNSProviderSecret{"secret_id": "secret-id", "secret_key": "secret-key"}},
		{provider: domain.DNSProviderDNSPod, secret: domain.DNSProviderSecret{"api_token": "app-id,app-token"}},
		{provider: domain.DNSProviderAliDNS, secret: domain.DNSProviderSecret{"access_key_id": "access-key-id", "access_key_secret": "access-key-secret"}},
	} {
		t.Run(string(testCase.provider), func(t *testing.T) {
			adapter, ok := dnsprovider.For(testCase.provider)
			if !ok {
				t.Fatal("provider adapter is missing")
			}
			if err := adapter.ValidateSecret(testCase.secret); err != nil {
				t.Fatal(err)
			}
			input := compilerFixture(t)
			input.Shares[0].Share.Path = t.TempDir()
			providerID := domain.ID("99999999-9999-4999-8999-999999999999")
			stamp := input.ServerSettings.CreatedAt
			input.DNSProviderCredentials = []domain.DNSProviderCredential{{ID: providerID, Name: string(testCase.provider), Provider: testCase.provider, CreatedAt: stamp, UpdatedAt: stamp}}
			input.TLSProfile = &domain.TLSProfile{ID: "66666666-6666-4666-8666-666666666666", Mode: domain.TLSModeAutomatic, Hostname: "dav.example.com", Challenge: domain.TLSChallengeDNS, DNSProviderID: &providerID, CreatedAt: stamp, UpdatedAt: stamp}
			compiled, err := (Compiler{}).Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := (BinaryValidator{BinaryPath: binary, TempDirectory: t.TempDir()}).ValidateWithEnvironment(context.Background(), compiled.JSON, adapter.Environment(providerID, testCase.secret)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
