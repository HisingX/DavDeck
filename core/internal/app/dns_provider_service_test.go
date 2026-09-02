package app

import (
	"context"
	"testing"

	"davdeck.dev/davdeck/core/internal/dnsprovider"
	"davdeck.dev/davdeck/core/internal/domain"
)

type memoryDNSProviders struct {
	providers map[domain.ID]domain.DNSProviderCredential
}

func newMemoryDNSProviders() *memoryDNSProviders {
	return &memoryDNSProviders{providers: make(map[domain.ID]domain.DNSProviderCredential)}
}

func (r *memoryDNSProviders) List(context.Context) ([]domain.DNSProviderCredential, error) {
	result := make([]domain.DNSProviderCredential, 0, len(r.providers))
	for _, provider := range r.providers {
		result = append(result, provider)
	}
	return result, nil
}

func (r *memoryDNSProviders) Get(_ context.Context, id domain.ID) (domain.DNSProviderCredential, error) {
	provider, ok := r.providers[id]
	if !ok {
		return domain.DNSProviderCredential{}, ErrDNSProviderNotFound
	}
	return provider, nil
}

func (r *memoryDNSProviders) Save(_ context.Context, provider domain.DNSProviderCredential) error {
	r.providers[provider.ID] = provider
	return nil
}

func (r *memoryDNSProviders) Delete(_ context.Context, id domain.ID) error {
	if _, ok := r.providers[id]; !ok {
		return ErrDNSProviderNotFound
	}
	delete(r.providers, id)
	return nil
}

type memoryDNSSecrets struct {
	values map[domain.ID]domain.DNSProviderSecret
}

func newMemoryDNSSecrets() *memoryDNSSecrets {
	return &memoryDNSSecrets{values: make(map[domain.ID]domain.DNSProviderSecret)}
}

func (s *memoryDNSSecrets) Get(_ context.Context, id domain.ID) (domain.DNSProviderSecret, bool, error) {
	value, ok := s.values[id]
	if !ok {
		return nil, false, nil
	}
	return cloneDNSSecret(value), true, nil
}

func (s *memoryDNSSecrets) Put(_ context.Context, id domain.ID, value domain.DNSProviderSecret, _ domain.Timestamp) error {
	s.values[id] = cloneDNSSecret(value)
	return nil
}

func (s *memoryDNSSecrets) Delete(_ context.Context, id domain.ID) error {
	delete(s.values, id)
	return nil
}

func cloneDNSSecret(value domain.DNSProviderSecret) domain.DNSProviderSecret {
	result := make(domain.DNSProviderSecret, len(value))
	for key, secret := range value {
		result[key] = secret
	}
	return result
}

func TestDNSProviderServiceKeepsSecretsOutOfViewsAndResolvesEnvironment(t *testing.T) {
	repository := newMemoryDNSProviders()
	secrets := newMemoryDNSSecrets()
	service := NewDNSProviderService(repository, secrets, fixedID{}, fixedClock{})
	provider, err := service.Save(context.Background(), DNSProviderUpdate{
		Name:         "Cloudflare production",
		Provider:     domain.DNSProviderCloudflare,
		AllowedZones: []string{"example.com"},
		Secret:       domain.DNSProviderSecret{"api_token": "real-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !provider.SecretConfigured || provider.Name != "Cloudflare production" {
		t.Fatalf("provider view = %#v", provider)
	}

	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 1 || !listed[0].SecretConfigured {
		t.Fatalf("listed = %#v, err = %v", listed, err)
	}

	profile := domain.TLSProfile{Challenge: domain.TLSChallengeDNS, Hostname: "dav.example.com", DNSProviderID: &provider.ID}
	environment, err := service.Environment(context.Background(), domain.RuntimeConfigInput{TLSProfile: &profile})
	if err != nil {
		t.Fatal(err)
	}
	if got := environment[dnsprovider.EnvironmentName(provider.ID, "api_token")]; got != "real-secret" {
		t.Fatalf("environment token = %q", got)
	}

	profile.Hostname = "dav.other.test"
	if _, err := service.Environment(context.Background(), domain.RuntimeConfigInput{TLSProfile: &profile}); !hasCode(err, CodeDNSProviderZoneNotAllowed) {
		t.Fatalf("out-of-zone error = %v", err)
	}

	updated, err := service.Save(context.Background(), DNSProviderUpdate{ID: provider.ID, Name: provider.Name, Provider: provider.Provider, AllowedZones: provider.AllowedZones})
	if err != nil || !updated.SecretConfigured {
		t.Fatalf("secret-preserving update = %#v, err = %v", updated, err)
	}
}

func TestDNSProviderServiceRejectsUnsupportedOrIncompleteSecrets(t *testing.T) {
	service := NewDNSProviderService(newMemoryDNSProviders(), newMemoryDNSSecrets(), fixedID{}, fixedClock{})
	for _, secret := range []domain.DNSProviderSecret{nil, {"api_token": ""}, {"unknown": "value"}} {
		if _, err := service.Save(context.Background(), DNSProviderUpdate{Name: "Cloudflare", Provider: domain.DNSProviderCloudflare, Secret: secret}); !hasCode(err, CodeInvalidDNSProviderSecret) {
			t.Fatalf("secret %#v error = %v", secret, err)
		}
	}
}
