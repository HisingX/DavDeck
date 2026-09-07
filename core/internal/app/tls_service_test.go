package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

type memoryTLS struct{ profile *domain.TLSProfile }

func (r *memoryTLS) Get(context.Context) (domain.TLSProfile, bool, error) {
	if r.profile == nil {
		return domain.TLSProfile{}, false, nil
	}
	return *r.profile, true, nil
}
func (r *memoryTLS) Save(_ context.Context, profile domain.TLSProfile) error {
	r.profile = &profile
	return nil
}
func (r *memoryTLS) Delete(context.Context) error {
	r.profile = nil
	return nil
}

type testResolver struct{ err error }

func (r testResolver) LookupHost(context.Context, string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []string{"203.0.113.10"}, nil
}

type testTLSFiles struct{ err error }

func (f testTLSFiles) CheckPair(string, string) error { return f.err }

func TestTLSServiceUpdatesModesAndRunsPreflight(t *testing.T) {
	repository := &memoryTLS{}
	service := NewTLSService(repository, testResolver{}, testTLSFiles{}, fixedID{}, fixedClock{})
	profile, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeAutomatic, Hostname: "dav.example.com"})
	if err != nil || profile.Mode != domain.TLSModeAutomatic {
		t.Fatalf("profile = %#v, err = %v", profile, err)
	}
	result, err := service.Check(context.Background())
	if err != nil || !result.Ready || len(result.Checks) != 2 {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	profile, err = service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeCustom, Hostname: "dav.example.com", CertificatePath: "/cert.pem", PrivateKeyPath: "/key.pem"})
	if err != nil || profile.ID != testUserID || profile.CreatedAt != profile.UpdatedAt {
		t.Fatalf("profile = %#v, err = %v", profile, err)
	}
	if _, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeInternal, Hostname: "https://bad"}); !hasCode(err, CodeTLSConfiguration) {
		t.Fatalf("error = %v", err)
	}
}

func TestTLSPreflightReturnsStableSafeErrors(t *testing.T) {
	repository := &memoryTLS{}
	service := NewTLSService(repository, testResolver{err: errors.New("resolver detail")}, testTLSFiles{}, fixedID{}, fixedClock{})
	if _, err := service.Check(context.Background()); !hasCode(err, CodeTLSConfiguration) {
		t.Fatalf("unconfigured error = %v", err)
	}
	if _, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeAutomatic, Hostname: "dav.example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Check(context.Background()); !hasCode(err, CodeDNSCheckFailed) {
		t.Fatalf("DNS error = %v", err)
	}
	repository = &memoryTLS{}
	missingCertificate := &tlsFileError{kind: CodeTLSCertificate, cause: os.ErrNotExist}
	service = NewTLSService(repository, testResolver{}, testTLSFiles{err: missingCertificate}, fixedID{}, fixedClock{})
	if _, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeCustom, Hostname: "dav.local", CertificatePath: "/missing/cert.pem", PrivateKeyPath: "/missing/key.pem"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Check(context.Background()); !hasCode(err, CodeTLSCertificate) {
		t.Fatalf("certificate error = %v", err)
	}
}

func TestTLSDNSChallengePreflightDoesNotRequireWildcardResolution(t *testing.T) {
	repository := &memoryTLS{}
	service := NewTLSService(repository, testResolver{err: errors.New("wildcard records do not resolve")}, testTLSFiles{}, fixedID{}, fixedClock{})
	providerID := domain.ID("22222222-2222-4222-8222-222222222222")
	if _, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeAutomatic, Hostname: "*.example.com", Challenge: domain.TLSChallengeDNS, DNSProviderID: &providerID}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Check(context.Background())
	if err != nil || !result.Ready || len(result.Checks) != 2 || result.Checks[1].Name != "dns_challenge" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

type testCertificateRenewal struct {
	status      caddyruntime.CertificateStatus
	calls       int
	cancelCalls int
}

func (r *testCertificateRenewal) CertificateStatus(context.Context, string) caddyruntime.CertificateStatus {
	return r.status
}

func (r *testCertificateRenewal) ForceRenewCertificate(context.Context, string) error {
	r.calls++
	return nil
}

func (r *testCertificateRenewal) CancelRenewCertificate(context.Context, string) error {
	r.cancelCalls++
	r.status = caddyruntime.CertificateStatus{State: caddyruntime.CertificateStatusReady}
	return nil
}

func TestTLSServiceRenewsOnlySavedAutomaticCertificates(t *testing.T) {
	repository := &memoryTLS{}
	renewal := &testCertificateRenewal{status: caddyruntime.CertificateStatus{State: caddyruntime.CertificateStatusReady}}
	service := NewTLSService(repository, testResolver{}, testTLSFiles{}, fixedID{}, fixedClock{})
	service.SetCertificateStatusProvider(renewal)
	service.SetCertificateRenewalProvider(renewal)
	if _, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeAutomatic, Hostname: "dav.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Renew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if renewal.calls != 1 {
		t.Fatalf("renewal calls = %d, want 1", renewal.calls)
	}
	renewal.status = caddyruntime.CertificateStatus{State: caddyruntime.CertificateStatusIssuing, Renewal: true}
	if err := service.Renew(context.Background()); !hasCode(err, CodeTLSRenewalInProgress) {
		t.Fatalf("in-progress error = %v", err)
	}
	if err := service.CancelRenew(context.Background()); err != nil {
		t.Fatalf("cancel renewal error = %v", err)
	}
	if renewal.cancelCalls != 1 {
		t.Fatalf("cancel renewal calls = %d", renewal.cancelCalls)
	}
	if _, err := service.Update(context.Background(), TLSUpdate{Mode: domain.TLSModeInternal, Hostname: "dav.local"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Renew(context.Background()); !hasCode(err, CodeTLSConfiguration) {
		t.Fatalf("internal renewal error = %v", err)
	}
}

func TestSystemTLSFileCheckerRejectsMissingAndInvalidPairs(t *testing.T) {
	checker := SystemTLSFileChecker{}
	directory := t.TempDir()
	certificate := filepath.Join(directory, "cert.pem")
	key := filepath.Join(directory, "key.pem")
	if err := checker.CheckPair(certificate, key); err == nil {
		t.Fatal("missing certificate passed preflight")
	}
	if err := os.WriteFile(certificate, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("not a private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checker.CheckPair(certificate, key); err == nil {
		t.Fatal("invalid certificate pair passed preflight")
	}
}
