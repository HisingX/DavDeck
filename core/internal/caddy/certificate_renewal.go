package caddy

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const (
	certificateRenewalIssuing = "ISSUING"
	certificateRenewalFailed  = "FAILED"
)

type certificateRenewal struct {
	hostname       string
	oldFingerprint string
	state          string
	message        string
	lastErrorCode  string
	cancel         context.CancelFunc
}

type certificateRenewalSnapshot struct {
	state         string
	message       string
	lastErrorCode string
}

type certificateRenewalAdmin interface {
	StartCertificateRenewal(context.Context, string) error
	CertificateRenewalStatus(context.Context, string) (CertificateRenewalStatus, error)
	CancelCertificateRenewal(context.Context, string) error
}

// ForceRenewCertificate starts a renewal for an already-issued automatic
// certificate using the exact automation policy currently active in Caddy.
// The Caddy module invokes CertMagic's force=true path, so no configuration
// mutation or synthetic TLS handshake is involved.
func (m *RuntimeManager) ForceRenewCertificate(ctx context.Context, hostname string) error {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return renewalRuntimeError("Certificate hostname is required", errors.New("empty certificate hostname"))
	}
	if err := ctx.Err(); err != nil {
		return renewalRuntimeError("Certificate renewal was canceled", err)
	}

	m.mu.Lock()
	if !m.runningLocked() {
		m.mu.Unlock()
		return renewalRuntimeError("Caddy is not running", errors.New("runtime is stopped"))
	}
	admin, ok := m.admin.(certificateRenewalAdmin)
	if !ok {
		m.mu.Unlock()
		return renewalRuntimeError("Caddy does not support forced certificate renewal", errors.New("the DavDeck Caddy renewal module is unavailable"))
	}
	if m.renewals == nil {
		m.renewals = make(map[string]*certificateRenewal)
	}
	if active := m.renewals[hostname]; active != nil && active.state == certificateRenewalIssuing {
		m.mu.Unlock()
		return renewalRuntimeError("Certificate renewal is already in progress", errors.New("renewal already active"))
	}
	storagePath := m.storagePath
	if storagePath == "" {
		storagePath = defaultCaddyStoragePath()
	}
	certificatePath := acmeCertificatePath(storagePath, hostname)
	oldCertificate, err := readCertificate(certificatePath)
	if err != nil {
		m.mu.Unlock()
		return renewalRuntimeError("The managed certificate could not be loaded for renewal", err)
	}
	renewalContext, cancel := context.WithCancel(context.Background())
	renewal := &certificateRenewal{
		hostname:       hostname,
		oldFingerprint: certificateFingerprint(oldCertificate),
		state:          certificateRenewalIssuing,
		message:        "Caddy is forcing renewal through the saved ACME policy",
		cancel:         cancel,
	}
	m.renewals[hostname] = renewal
	m.mu.Unlock()

	if err := admin.StartCertificateRenewal(ctx, hostname); err != nil {
		m.mu.Lock()
		if m.renewals[hostname] == renewal {
			delete(m.renewals, hostname)
		}
		m.mu.Unlock()
		cancel()
		return renewalRuntimeError("Unable to start certificate renewal in Caddy", err)
	}
	go m.monitorCertificateRenewal(renewalContext, renewal, certificatePath)
	return nil
}

func (m *RuntimeManager) monitorCertificateRenewal(ctx context.Context, renewal *certificateRenewal, certificatePath string) {
	pollInterval := m.renewalPollInterval
	if pollInterval <= 0 {
		pollInterval = defaultCertificateRenewalPollInterval
	}
	timeout := m.renewalTimeout
	if timeout <= 0 {
		timeout = defaultCertificateRenewalTimeout
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			m.failCertificateRenewal(renewal, errors.New("renewal did not produce a new certificate before the timeout"))
			return
		case <-ticker.C:
			certificate, err := readCertificate(certificatePath)
			if err == nil && certificateFingerprint(certificate) != renewal.oldFingerprint {
				_ = m.finishCertificateRenewal(renewal, nil)
				return
			}
			statusContext, statusCancel := context.WithTimeout(ctx, 5*time.Second)
			status, statusErr := m.certificateRenewalStatus(statusContext, renewal.hostname)
			statusCancel()
			if statusErr != nil {
				continue
			}
			switch status.State {
			case certificateRenewalFailed:
				m.failCertificateRenewal(renewal, errors.New("Caddy reported certificate renewal failure"))
				return
			case "CANCELED":
				m.clearCertificateRenewal(renewal)
				return
			case "SUCCEEDED":
				_ = m.finishCertificateRenewal(renewal, nil)
				return
			}
		}
	}
}

func (m *RuntimeManager) certificateRenewalStatus(ctx context.Context, hostname string) (CertificateRenewalStatus, error) {
	admin, ok := m.admin.(certificateRenewalAdmin)
	if !ok {
		return CertificateRenewalStatus{}, errors.New("Caddy renewal status is unavailable")
	}
	return admin.CertificateRenewalStatus(ctx, hostname)
}

func (m *RuntimeManager) finishCertificateRenewal(renewal *certificateRenewal, renewalErr error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewals[renewal.hostname] != renewal {
		return context.Canceled
	}
	if renewalErr != nil {
		return m.recordCertificateRenewalFailureLocked(renewal, renewalErr)
	}
	delete(m.renewals, renewal.hostname)
	if renewal.cancel != nil {
		renewal.cancel()
	}
	return nil
}

func (m *RuntimeManager) clearCertificateRenewal(renewal *certificateRenewal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewals[renewal.hostname] == renewal {
		delete(m.renewals, renewal.hostname)
		if renewal.cancel != nil {
			renewal.cancel()
		}
	}
}

func (m *RuntimeManager) failCertificateRenewal(renewal *certificateRenewal, err error) {
	if admin, ok := m.admin.(certificateRenewalAdmin); ok {
		cancelContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = admin.CancelCertificateRenewal(cancelContext, renewal.hostname)
		cancel()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewals[renewal.hostname] == renewal {
		_ = m.recordCertificateRenewalFailureLocked(renewal, err)
	}
}

func (m *RuntimeManager) recordCertificateRenewalFailureLocked(renewal *certificateRenewal, err error) error {
	renewal.state = certificateRenewalFailed
	renewal.message = "Caddy could not renew the managed certificate; check the logs"
	renewal.lastErrorCode = CertificateStatusACMEErrorCode
	if renewal.cancel != nil {
		renewal.cancel()
	}
	if m.logger != nil {
		m.logger.Error("managed certificate renewal failed", "hostname", renewal.hostname, "error_code", CertificateStatusACMEErrorCode)
	}
	return err
}

// CancelRenewCertificate stops an active one-shot renewal while preserving
// the saved TLS profile and the previous certificate on disk.
func (m *RuntimeManager) CancelRenewCertificate(ctx context.Context, hostname string) error {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	m.mu.Lock()
	renewal := m.renewals[hostname]
	admin, ok := m.admin.(certificateRenewalAdmin)
	if renewal == nil || renewal.state != certificateRenewalIssuing {
		m.mu.Unlock()
		return renewalRuntimeError("Certificate renewal is not in progress", errors.New("renewal is not active"))
	}
	m.mu.Unlock()
	if !ok {
		return renewalRuntimeError("Caddy does not support canceling certificate renewal", errors.New("the DavDeck Caddy renewal module is unavailable"))
	}
	if err := admin.CancelCertificateRenewal(ctx, hostname); err != nil {
		return renewalRuntimeError("Unable to cancel certificate renewal", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewals[hostname] == renewal {
		delete(m.renewals, hostname)
		if renewal.cancel != nil {
			renewal.cancel()
		}
	}
	return nil
}

func (m *RuntimeManager) cancelRenewalsLocked() {
	for hostname, renewal := range m.renewals {
		if renewal != nil && renewal.cancel != nil {
			renewal.cancel()
		}
		delete(m.renewals, hostname)
	}
}

func (m *RuntimeManager) certificateRenewalSnapshot(hostname string) *certificateRenewalSnapshot {
	renewal := m.renewals[strings.ToLower(strings.TrimSpace(hostname))]
	if renewal == nil {
		return nil
	}
	return &certificateRenewalSnapshot{state: renewal.state, message: renewal.message, lastErrorCode: renewal.lastErrorCode}
}

func renewalRuntimeError(message string, cause error) error {
	return &RuntimeError{Code: CodeCaddyReloadFailed, Message: message, Cause: cause}
}

func certificateFingerprint(certificate *x509.Certificate) string {
	if certificate == nil {
		return ""
	}
	hash := sha256.Sum256(certificate.Raw)
	return hex.EncodeToString(hash[:])
}
