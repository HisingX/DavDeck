package caddy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type renewalTestAdmin struct {
	mu       sync.Mutex
	status   CertificateRenewalStatus
	started  []string
	canceled []string
}

func (a *renewalTestAdmin) Health(context.Context) error         { return nil }
func (a *renewalTestAdmin) Reload(context.Context, []byte) error { return nil }
func (a *renewalTestAdmin) Stop(context.Context) error           { return nil }
func (a *renewalTestAdmin) StartCertificateRenewal(_ context.Context, hostname string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.started = append(a.started, hostname)
	a.status = CertificateRenewalStatus{State: CertificateStatusIssuing, Message: "issuing"}
	return nil
}
func (a *renewalTestAdmin) CertificateRenewalStatus(context.Context, string) (CertificateRenewalStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status, nil
}
func (a *renewalTestAdmin) CancelCertificateRenewal(_ context.Context, hostname string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.canceled = append(a.canceled, hostname)
	a.status = CertificateRenewalStatus{State: "CANCELED", Message: "canceled"}
	return nil
}

type renewalTestValidator struct{}

func (renewalTestValidator) Validate(context.Context, []byte) error { return nil }

func TestForceRenewalUsesActiveCaddyOperationAndDetectsNewCertificate(t *testing.T) {
	storagePath := t.TempDir()
	oldCertificatePath, _ := writeTestCertificatePair(t, t.TempDir(), "dav.example.com")
	newCertificatePath, _ := writeTestCertificatePair(t, t.TempDir(), "dav.example.com")
	managedCertificatePath := acmeCertificatePath(storagePath, "dav.example.com")
	if err := os.MkdirAll(filepath.Dir(managedCertificatePath), 0o700); err != nil {
		t.Fatal(err)
	}
	oldCertificate, err := os.ReadFile(oldCertificatePath)
	if err != nil {
		t.Fatal(err)
	}
	newCertificate, err := os.ReadFile(newCertificatePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedCertificatePath, oldCertificate, 0o600); err != nil {
		t.Fatal(err)
	}

	admin := &renewalTestAdmin{}
	manager := NewRuntimeManager("", "", renewalTestValidator{}, admin, nil, nil)
	manager.storagePath = storagePath
	manager.renewalTimeout = time.Second
	manager.renewalPollInterval = time.Millisecond
	manager.command = &exec.Cmd{Process: &os.Process{}} // only non-nil process state is required by runningLocked
	manager.done = make(chan error)

	if err := manager.ForceRenewCertificate(context.Background(), "DAV.EXAMPLE.COM"); err != nil {
		t.Fatal(err)
	}
	admin.mu.Lock()
	if len(admin.started) != 1 || admin.started[0] != "dav.example.com" {
		t.Fatalf("started = %#v", admin.started)
	}
	admin.mu.Unlock()
	if err := os.WriteFile(managedCertificatePath, newCertificate, 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.CertificateStatus(context.Background(), "dav.example.com")
		if !status.Renewal && status.State == CertificateStatusReady {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	status := manager.CertificateStatus(context.Background(), "dav.example.com")
	t.Fatalf("status = %#v, want ready after renewal", status)
}

func TestForceRenewalReportsCaddyFailure(t *testing.T) {
	storagePath := t.TempDir()
	sourceCertificate, _ := writeTestCertificatePair(t, t.TempDir(), "dav.example.com")
	managedCertificatePath := acmeCertificatePath(storagePath, "dav.example.com")
	if err := os.MkdirAll(filepath.Dir(managedCertificatePath), 0o700); err != nil {
		t.Fatal(err)
	}
	certificate, err := os.ReadFile(sourceCertificate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedCertificatePath, certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	admin := &renewalTestAdmin{}
	manager := NewRuntimeManager("", "", renewalTestValidator{}, admin, nil, nil)
	manager.storagePath = storagePath
	manager.renewalTimeout = time.Second
	manager.renewalPollInterval = time.Millisecond
	manager.command = &exec.Cmd{Process: &os.Process{}}
	manager.done = make(chan error)
	if err := manager.ForceRenewCertificate(context.Background(), "dav.example.com"); err != nil {
		t.Fatal(err)
	}
	admin.mu.Lock()
	admin.status = CertificateRenewalStatus{State: "FAILED", Message: "failed"}
	admin.mu.Unlock()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.CertificateStatus(context.Background(), "dav.example.com")
		if status.Renewal && status.State == CertificateStatusFailed {
			if status.LastErrorCode != CertificateStatusACMEErrorCode {
				t.Fatalf("error code = %q", status.LastErrorCode)
			}
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("renewal failure was not reported")
}

func TestCancelRenewalPreservesProfileAndCertificate(t *testing.T) {
	storagePath := t.TempDir()
	sourceCertificate, _ := writeTestCertificatePair(t, t.TempDir(), "dav.example.com")
	managedCertificatePath := acmeCertificatePath(storagePath, "dav.example.com")
	if err := os.MkdirAll(filepath.Dir(managedCertificatePath), 0o700); err != nil {
		t.Fatal(err)
	}
	certificate, err := os.ReadFile(sourceCertificate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedCertificatePath, certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	admin := &renewalTestAdmin{}
	manager := NewRuntimeManager("", "", renewalTestValidator{}, admin, nil, nil)
	manager.storagePath = storagePath
	manager.renewalPollInterval = time.Millisecond
	manager.command = &exec.Cmd{Process: &os.Process{}}
	manager.done = make(chan error)
	if err := manager.ForceRenewCertificate(context.Background(), "dav.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := manager.CancelRenewCertificate(context.Background(), "dav.example.com"); err != nil {
		t.Fatal(err)
	}
	status := manager.CertificateStatus(context.Background(), "dav.example.com")
	if status.Renewal || status.State != CertificateStatusReady {
		t.Fatalf("status = %#v, want ready without renewal", status)
	}
	admin.mu.Lock()
	defer admin.mu.Unlock()
	if len(admin.canceled) != 1 || admin.canceled[0] != "dav.example.com" {
		t.Fatalf("canceled = %#v", admin.canceled)
	}
}

func TestForceRenewalRequiresDavDeckCaddyModule(t *testing.T) {
	manager := NewRuntimeManager("", "", renewalTestValidator{}, &renewalLegacyAdmin{}, nil, nil)
	manager.storagePath = t.TempDir()
	manager.command = &exec.Cmd{Process: &os.Process{}}
	manager.done = make(chan error)
	if err := manager.ForceRenewCertificate(context.Background(), "dav.example.com"); err == nil {
		t.Fatal("renewal started without the Caddy renewal module")
	}
}

type renewalLegacyAdmin struct{}

func (renewalLegacyAdmin) Health(context.Context) error         { return nil }
func (renewalLegacyAdmin) Reload(context.Context, []byte) error { return nil }
func (renewalLegacyAdmin) Stop(context.Context) error           { return nil }
