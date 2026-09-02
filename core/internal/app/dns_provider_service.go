package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"davdeck.dev/davdeck/core/internal/dnsprovider"
	"davdeck.dev/davdeck/core/internal/domain"
)

var (
	ErrDNSProviderNotFound      = domain.ErrDNSProviderNotFound
	ErrDNSProviderInUse         = domain.ErrDNSProviderInUse
	ErrDNSProviderAlreadyExists = errors.New("DNS provider credential already exists")
)

type DNSProviderRepository interface {
	List(context.Context) ([]domain.DNSProviderCredential, error)
	Get(context.Context, domain.ID) (domain.DNSProviderCredential, error)
	Save(context.Context, domain.DNSProviderCredential) error
	Delete(context.Context, domain.ID) error
}

type DNSSecretStore interface {
	Get(context.Context, domain.ID) (domain.DNSProviderSecret, bool, error)
	Put(context.Context, domain.ID, domain.DNSProviderSecret, domain.Timestamp) error
	Delete(context.Context, domain.ID) error
}

type DNSProviderCredentialView struct {
	ID               domain.ID              `json:"id"`
	Name             string                 `json:"name"`
	Provider         domain.DNSProviderType `json:"provider"`
	AllowedZones     []string               `json:"allowed_zones,omitempty"`
	SecretConfigured bool                   `json:"secret_configured"`
	CreatedAt        domain.Timestamp       `json:"created_at"`
	UpdatedAt        domain.Timestamp       `json:"updated_at"`
}

type DNSProviderUpdate struct {
	ID           domain.ID
	Name         string
	Provider     domain.DNSProviderType
	AllowedZones []string
	Secret       domain.DNSProviderSecret
}

type DNSProviderService struct {
	repository DNSProviderRepository
	secrets    DNSSecretStore
	ids        IDGenerator
	clock      Clock
}

func NewDNSProviderService(repository DNSProviderRepository, secrets DNSSecretStore, ids IDGenerator, clock Clock) *DNSProviderService {
	return &DNSProviderService{repository: repository, secrets: secrets, ids: ids, clock: clock}
}

func (s *DNSProviderService) List(ctx context.Context) ([]DNSProviderCredentialView, error) {
	credentials, err := s.repository.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]DNSProviderCredentialView, 0, len(credentials))
	for _, credential := range credentials {
		view, err := s.view(ctx, credential)
		if err != nil {
			return nil, databaseError(err)
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *DNSProviderService) Get(ctx context.Context, id domain.ID) (DNSProviderCredentialView, error) {
	credential, err := s.repository.Get(ctx, id)
	if errors.Is(err, ErrDNSProviderNotFound) {
		return DNSProviderCredentialView{}, &Error{Code: CodeDNSProviderNotFound, Message: "DNS provider credential was not found", Cause: err}
	}
	if err != nil {
		return DNSProviderCredentialView{}, databaseError(err)
	}
	view, err := s.view(ctx, credential)
	if err != nil {
		return DNSProviderCredentialView{}, databaseError(err)
	}
	return view, nil
}

func (s *DNSProviderService) Save(ctx context.Context, update DNSProviderUpdate) (DNSProviderCredentialView, error) {
	var existing domain.DNSProviderCredential
	found := false
	if update.ID != "" {
		var err error
		existing, err = s.repository.Get(ctx, update.ID)
		if errors.Is(err, ErrDNSProviderNotFound) {
			return DNSProviderCredentialView{}, &Error{Code: CodeDNSProviderNotFound, Message: "DNS provider credential was not found", Cause: err}
		}
		if err != nil {
			return DNSProviderCredentialView{}, databaseError(err)
		}
		found = true
	}
	providers, err := s.repository.List(ctx)
	if err != nil {
		return DNSProviderCredentialView{}, databaseError(err)
	}
	for _, value := range providers {
		if value.Name == update.Name && (!found || value.ID != update.ID) {
			return DNSProviderCredentialView{}, &Error{Code: CodeDNSProviderAlreadyExists, Message: "DNS provider credential name already exists", Cause: ErrDNSProviderAlreadyExists}
		}
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return DNSProviderCredentialView{}, databaseError(err)
	}
	credential := domain.DNSProviderCredential{ID: update.ID, Name: update.Name, Provider: update.Provider, AllowedZones: append([]string(nil), update.AllowedZones...), UpdatedAt: stamp}
	if found {
		credential.CreatedAt, credential.ID = existing.CreatedAt, existing.ID
	} else {
		credential.ID, err = s.ids.NewID()
		if err != nil {
			return DNSProviderCredentialView{}, databaseError(fmt.Errorf("generate DNS provider id: %w", err))
		}
		credential.CreatedAt = stamp
	}
	if err := credential.Validate(); err != nil {
		return DNSProviderCredentialView{}, &Error{Code: CodeInvalidDNSProvider, Message: "DNS provider credential metadata is invalid", Cause: err}
	}
	adapter, ok := dnsprovider.For(credential.Provider)
	if !ok {
		return DNSProviderCredentialView{}, &Error{Code: CodeInvalidDNSProvider, Message: "DNS provider is not supported by this build"}
	}
	if update.Secret == nil && found && existing.Provider == credential.Provider {
		_, configured, err := s.secrets.Get(ctx, credential.ID)
		if err != nil {
			return DNSProviderCredentialView{}, databaseError(err)
		}
		if !configured {
			return DNSProviderCredentialView{}, &Error{Code: CodeDNSProviderSecretMissing, Message: "DNS provider credentials must be supplied"}
		}
	} else {
		if err := adapter.ValidateSecret(update.Secret); err != nil {
			return DNSProviderCredentialView{}, &Error{Code: CodeInvalidDNSProviderSecret, Message: "DNS provider secret is invalid", Cause: err}
		}
	}
	if err := s.repository.Save(ctx, credential); err != nil {
		return DNSProviderCredentialView{}, mapDNSProviderRepositoryError(err)
	}
	if update.Secret != nil {
		if err := s.secrets.Put(ctx, credential.ID, update.Secret, stamp); err != nil {
			return DNSProviderCredentialView{}, databaseError(err)
		}
	}
	return s.view(ctx, credential)
}

func (s *DNSProviderService) Delete(ctx context.Context, id domain.ID) error {
	if err := s.repository.Delete(ctx, id); err != nil {
		return mapDNSProviderRepositoryError(err)
	}
	return nil
}

// Environment returns the secret environment required by the active DNS
// challenge. It is intentionally limited to the referenced credential.
func (s *DNSProviderService) Environment(ctx context.Context, input domain.RuntimeConfigInput) (map[string]string, error) {
	result := make(map[string]string)
	if input.TLSProfile == nil || input.TLSProfile.Challenge != domain.TLSChallengeDNS {
		return result, nil
	}
	if input.TLSProfile.DNSProviderID == nil {
		return nil, &Error{Code: CodeDNSProviderSecretMissing, Message: "DNS TLS challenge has no provider"}
	}
	credential, err := s.repository.Get(ctx, *input.TLSProfile.DNSProviderID)
	if errors.Is(err, ErrDNSProviderNotFound) {
		return nil, &Error{Code: CodeDNSProviderNotFound, Message: "DNS provider credential was not found", Cause: err}
	}
	if err != nil {
		return nil, databaseError(err)
	}
	if len(credential.AllowedZones) > 0 && !hostnameAllowedByZones(input.TLSProfile.Hostname, credential.AllowedZones) {
		return nil, &Error{Code: CodeDNSProviderZoneNotAllowed, Message: "TLS hostname is outside the DNS provider credential's allowed zones"}
	}
	secret, configured, err := s.secrets.Get(ctx, credential.ID)
	if err != nil {
		return nil, databaseError(err)
	}
	if !configured {
		return nil, &Error{Code: CodeDNSProviderSecretMissing, Message: "DNS provider credential has no secret"}
	}
	adapter, ok := dnsprovider.For(credential.Provider)
	if !ok {
		return nil, &Error{Code: CodeInvalidDNSProvider, Message: "DNS provider is not supported by this build"}
	}
	if err := adapter.ValidateSecret(secret); err != nil {
		return nil, &Error{Code: CodeInvalidDNSProviderSecret, Message: "DNS provider secret is invalid", Cause: err}
	}
	return adapter.Environment(credential.ID, secret), nil
}

func hostnameAllowedByZones(hostname string, zones []string) bool {
	hostname = strings.TrimPrefix(strings.ToLower(hostname), "*.")
	for _, zone := range zones {
		zone = strings.ToLower(strings.TrimSuffix(zone, "."))
		if hostname == zone || strings.HasSuffix(hostname, "."+zone) {
			return true
		}
	}
	return false
}

func (s *DNSProviderService) view(ctx context.Context, credential domain.DNSProviderCredential) (DNSProviderCredentialView, error) {
	_, configured, err := s.secrets.Get(ctx, credential.ID)
	if err != nil {
		return DNSProviderCredentialView{}, err
	}
	return DNSProviderCredentialView{ID: credential.ID, Name: credential.Name, Provider: credential.Provider, AllowedZones: append([]string(nil), credential.AllowedZones...), SecretConfigured: configured, CreatedAt: credential.CreatedAt, UpdatedAt: credential.UpdatedAt}, nil
}

func mapDNSProviderRepositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrDNSProviderNotFound):
		return &Error{Code: CodeDNSProviderNotFound, Message: "DNS provider credential was not found", Cause: err}
	case errors.Is(err, ErrDNSProviderInUse):
		return &Error{Code: CodeDNSProviderInUse, Message: "DNS provider credential is still used by TLS", Cause: err}
	default:
		return databaseError(err)
	}
}
