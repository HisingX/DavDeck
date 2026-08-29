package app

import (
	"context"
	cryptotls "crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"

	"davdeck.dev/davdeck/core/internal/domain"
)

type TLSRepository interface {
	Get(context.Context) (domain.TLSProfile, bool, error)
	Save(context.Context, domain.TLSProfile) error
	Delete(context.Context) error
}

type TLSResolver interface {
	LookupHost(context.Context, string) ([]string, error)
}

type TLSFileChecker interface {
	CheckPair(string, string) error
}

type TLSCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type TLSCheckResult struct {
	Ready  bool       `json:"ready"`
	Checks []TLSCheck `json:"checks"`
}

type TLSUpdate struct {
	Mode            domain.TLSMode
	Hostname        string
	CertificatePath string
	PrivateKeyPath  string
}

type TLSService struct {
	repository TLSRepository
	resolver   TLSResolver
	files      TLSFileChecker
	ids        IDGenerator
	clock      Clock
}

func NewTLSService(repository TLSRepository, resolver TLSResolver, files TLSFileChecker, ids IDGenerator, clock Clock) *TLSService {
	return &TLSService{repository: repository, resolver: resolver, files: files, ids: ids, clock: clock}
}

func (s *TLSService) Get(ctx context.Context) (domain.TLSProfile, bool, error) {
	profile, found, err := s.repository.Get(ctx)
	if err != nil {
		return domain.TLSProfile{}, false, databaseError(err)
	}
	return profile, found, nil
}

func (s *TLSService) Update(ctx context.Context, update TLSUpdate) (domain.TLSProfile, error) {
	existing, found, err := s.repository.Get(ctx)
	if err != nil {
		return domain.TLSProfile{}, databaseError(err)
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.TLSProfile{}, databaseError(err)
	}
	profile := domain.TLSProfile{Mode: update.Mode, Hostname: update.Hostname, CertificatePath: update.CertificatePath, PrivateKeyPath: update.PrivateKeyPath, UpdatedAt: stamp}
	if found {
		profile.ID, profile.CreatedAt = existing.ID, existing.CreatedAt
	} else {
		profile.ID, err = s.ids.NewID()
		if err != nil {
			return domain.TLSProfile{}, databaseError(fmt.Errorf("generate TLS profile id: %w", err))
		}
		profile.CreatedAt = stamp
	}
	if err := validateTLSProfile(profile); err != nil {
		return domain.TLSProfile{}, err
	}
	if err := s.repository.Save(ctx, profile); err != nil {
		return domain.TLSProfile{}, databaseError(err)
	}
	return profile, nil
}

// Disable removes the desired TLS profile so the compiler returns to its
// initial HTTP-only configuration. It does not remove any user files or
// certificate files referenced by the old profile.
func (s *TLSService) Disable(ctx context.Context) error {
	if err := s.repository.Delete(ctx); err != nil {
		return databaseError(err)
	}
	return nil
}

func (s *TLSService) Check(ctx context.Context) (TLSCheckResult, error) {
	profile, found, err := s.repository.Get(ctx)
	if err != nil {
		return TLSCheckResult{}, databaseError(err)
	}
	if !found {
		return TLSCheckResult{}, &Error{Code: CodeTLSConfiguration, Message: "TLS is not configured"}
	}
	checks := []TLSCheck{{Name: "configuration", OK: true, Message: "TLS configuration is valid"}}
	switch profile.Mode {
	case domain.TLSModeAutomatic:
		addresses, lookupErr := s.resolver.LookupHost(ctx, profile.Hostname)
		if lookupErr != nil || len(addresses) == 0 {
			return TLSCheckResult{Checks: append(checks, TLSCheck{Name: "dns", OK: false, Message: "Hostname did not resolve"})}, &Error{Code: CodeDNSCheckFailed, Message: "TLS hostname did not resolve", Cause: lookupErr}
		}
		checks = append(checks, TLSCheck{Name: "dns", OK: true, Message: "Hostname resolves"})
	case domain.TLSModeCustom:
		if err := s.files.CheckPair(profile.CertificatePath, profile.PrivateKeyPath); err != nil {
			return TLSCheckResult{Checks: append(checks, TLSCheck{Name: "certificate_pair", OK: false, Message: "Certificate pair could not be loaded"})}, mapTLSFileError(err)
		}
		checks = append(checks, TLSCheck{Name: "certificate_pair", OK: true, Message: "Certificate and private key are readable and match"})
	case domain.TLSModeInternal:
		checks = append(checks, TLSCheck{Name: "trust", OK: true, Message: "Internal CA trust is required on clients"})
	}
	return TLSCheckResult{Ready: true, Checks: checks}, nil
}

func validateTLSProfile(profile domain.TLSProfile) error {
	if err := profile.Validate(); err != nil {
		return &Error{Code: CodeTLSConfiguration, Message: "TLS configuration is invalid", Cause: err}
	}
	return nil
}

type SystemTLSResolver struct{}

func (SystemTLSResolver) LookupHost(ctx context.Context, hostname string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, hostname)
}

type SystemTLSFileChecker struct{}

func (SystemTLSFileChecker) CheckPair(certificatePath, privateKeyPath string) error {
	if _, err := os.Stat(certificatePath); err != nil {
		return &tlsFileError{kind: CodeTLSCertificate, cause: err}
	}
	if _, err := os.Stat(privateKeyPath); err != nil {
		return &tlsFileError{kind: CodeTLSPrivateKey, cause: err}
	}
	if _, err := cryptotls.LoadX509KeyPair(certificatePath, privateKeyPath); err != nil {
		return &tlsFileError{kind: CodeTLSConfiguration, cause: err}
	}
	return nil
}

type tlsFileError struct {
	kind  ErrorCode
	cause error
}

func (e *tlsFileError) Error() string { return e.cause.Error() }
func (e *tlsFileError) Unwrap() error { return e.cause }

func mapTLSFileError(err error) error {
	var fileError *tlsFileError
	if errors.As(err, &fileError) {
		switch fileError.kind {
		case CodeTLSCertificate:
			return &Error{Code: CodeTLSCertificate, Message: "TLS certificate file was not found or is not readable", Cause: err}
		case CodeTLSPrivateKey:
			return &Error{Code: CodeTLSPrivateKey, Message: "TLS private key file was not found or is not readable", Cause: err}
		}
	}
	return &Error{Code: CodeTLSConfiguration, Message: "TLS certificate and private key could not be loaded", Cause: err}
}
