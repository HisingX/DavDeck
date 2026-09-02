package domain

import "testing"

func TestTLSDNSChallengeValidation(t *testing.T) {
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	providerID := testOtherID
	valid := TLSProfile{ID: testID, Mode: TLSModeAutomatic, Hostname: "*.example.com", Challenge: TLSChallengeDNS, DNSProviderID: &providerID, CreatedAt: stamp, UpdatedAt: stamp}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, profile := range map[string]TLSProfile{
		"wildcard without DNS": {ID: testID, Mode: TLSModeAutomatic, Hostname: "*.example.com", CreatedAt: stamp, UpdatedAt: stamp},
		"DNS on internal":      {ID: testID, Mode: TLSModeInternal, Hostname: "dav.local", Challenge: TLSChallengeDNS, DNSProviderID: &providerID, CreatedAt: stamp, UpdatedAt: stamp},
		"multiple wildcards":   {ID: testID, Mode: TLSModeAutomatic, Hostname: "*.*.example.com", Challenge: TLSChallengeDNS, DNSProviderID: &providerID, CreatedAt: stamp, UpdatedAt: stamp},
		"DNS without provider": {ID: testID, Mode: TLSModeAutomatic, Hostname: "dav.example.com", Challenge: TLSChallengeDNS, CreatedAt: stamp, UpdatedAt: stamp},
	} {
		if err := profile.Validate(); err == nil {
			t.Errorf("%s: expected validation failure", name)
		}
	}
}

func TestDNSProviderCredentialValidation(t *testing.T) {
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	credential := DNSProviderCredential{ID: testID, Name: "Home Cloudflare", Provider: DNSProviderCloudflare, AllowedZones: []string{"example.com"}, CreatedAt: stamp, UpdatedAt: stamp}
	if err := credential.Validate(); err != nil {
		t.Fatal(err)
	}
	credential.AllowedZones = []string{"*.example.com"}
	if err := credential.Validate(); err == nil {
		t.Fatal("expected wildcard allowed zone to be rejected")
	}
}
