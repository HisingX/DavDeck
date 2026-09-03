package caddy

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	CertificateStatusNotConfigured     = "NOT_CONFIGURED"
	CertificateStatusWaitingForApply   = "WAITING_FOR_APPLY"
	CertificateStatusWaitingForRuntime = "WAITING_FOR_RUNTIME"
	CertificateStatusIssuing           = "ISSUING"
	CertificateStatusReady             = "READY"
	CertificateStatusExpired           = "EXPIRED"
	CertificateStatusFailed            = "FAILED"
	CertificateStatusUnknown           = "UNKNOWN"
	CertificateStatusACMEErrorCode     = "ACME_CERTIFICATE_ISSUANCE_FAILED"

	defaultACMEIssuerStorageKey = "acme-v02.api.letsencrypt.org-directory"
)

// CertificateStatus is the safe, public-certificate-only view of Caddy's
// managed certificate state. Private key contents and paths are never read or
// returned here.
type CertificateStatus struct {
	State           string     `json:"state"`
	StoragePath     string     `json:"storage_path"`
	CertificatePath string     `json:"certificate_path,omitempty"`
	Message         string     `json:"message"`
	NotBefore       *time.Time `json:"not_before,omitempty"`
	NotAfter        *time.Time `json:"not_after,omitempty"`
	LastErrorCode   string     `json:"last_error_code,omitempty"`
	Renewal         bool       `json:"renewal,omitempty"`
}

// CertificateErrorReader reports whether the runtime log boundary has seen a
// recent ACME error for hostname. It is deliberately a narrow callback so the
// Caddy package does not depend on the logging storage implementation.
type CertificateErrorReader func(hostname string) bool

// CertificateStatus reports the certificate that Caddy manages for hostname.
// ACME management is asynchronous, so a healthy Caddy process without a
// certificate is reported as ISSUING rather than as a successful apply.
func (m *RuntimeManager) CertificateStatus(ctx context.Context, hostname string) CertificateStatus {
	if err := ctx.Err(); err != nil {
		return CertificateStatus{State: CertificateStatusUnknown, Message: "Certificate status is unavailable"}
	}

	m.mu.Lock()
	storagePath := m.storagePath
	running := m.runningLocked()
	lastErrorCode := string(m.lastErrorCode)
	errorReader := m.certificateErrorReader
	renewal := m.certificateRenewalSnapshot(hostname)
	m.mu.Unlock()
	if storagePath == "" {
		storagePath = defaultCaddyStoragePath()
	}

	certificatePath := acmeCertificatePath(storagePath, hostname)
	result := CertificateStatus{
		State:           CertificateStatusWaitingForRuntime,
		StoragePath:     storagePath,
		CertificatePath: certificatePath,
		Message:         "Start Caddy and apply the configuration to request the certificate",
		LastErrorCode:   lastErrorCode,
	}

	certificate, err := readCertificate(certificatePath)
	if renewal != nil {
		result.Renewal = true
		if err == nil {
			notBefore, notAfter := certificate.NotBefore.UTC(), certificate.NotAfter.UTC()
			result.NotBefore, result.NotAfter = &notBefore, &notAfter
		}
		result.State = renewal.state
		result.Message = renewal.message
		result.LastErrorCode = renewal.lastErrorCode
		return result
	}
	switch {
	case err == nil:
		notBefore, notAfter := certificate.NotBefore.UTC(), certificate.NotAfter.UTC()
		result.NotBefore, result.NotAfter = &notBefore, &notAfter
		if !certificate.NotAfter.After(time.Now()) {
			result.State = CertificateStatusExpired
			result.Message = "The managed certificate has expired"
			return result
		}
		result.State = CertificateStatusReady
		result.Message = "The managed certificate is available"
		return result
	case !errors.Is(err, os.ErrNotExist):
		result.State = CertificateStatusFailed
		result.Message = "The managed certificate could not be read"
		return result
	case lastErrorCode != "":
		result.State = CertificateStatusFailed
		result.Message = "Caddy reported a runtime error while managing the certificate"
		return result
	case errorReader != nil && errorReader(hostname):
		result.State = CertificateStatusFailed
		result.LastErrorCode = CertificateStatusACMEErrorCode
		result.Message = "Caddy reported an ACME certificate issuance error"
		return result
	case running:
		result.State = CertificateStatusIssuing
		result.Message = "Caddy is requesting or renewing the certificate"
		return result
	default:
		return result
	}
}

func readCertificate(path string) (*x509.Certificate, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(contents)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("certificate PEM block was not found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func acmeCertificatePath(storagePath, hostname string) string {
	key := safeStorageKey(hostname)
	return filepath.Join(storagePath, "certificates", defaultACMEIssuerStorageKey, key, key+".crt")
}

var unsafeStorageKeyCharacters = regexp.MustCompile(`[^\w@.-]`)

// safeStorageKey mirrors CertMagic's key normalization for the hostname
// portion of the local storage path. Keeping this local avoids adding Caddy's
// full module as a dependency of the daemon core.
func safeStorageKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		" ", "_",
		"+", "_plus_",
		"*", "wildcard_",
		":", "-",
		"..", "",
	).Replace(value)
	return unsafeStorageKeyCharacters.ReplaceAllString(value, "")
}

func defaultCaddyStoragePath() string {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "caddy")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "./caddy"
	}
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("AppData"); appData != "" {
			return filepath.Join(appData, "Caddy")
		}
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Caddy")
	default:
		return filepath.Join(home, ".local", "share", "caddy")
	}
	return filepath.Join(home, "Caddy")
}
