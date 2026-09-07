package caddy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCertificateStatusReportsRuntimeAndReadyStates(t *testing.T) {
	storagePath := t.TempDir()
	manager := &RuntimeManager{storagePath: storagePath}

	waiting := manager.CertificateStatus(context.Background(), "dav.example.com")
	if waiting.State != CertificateStatusWaitingForRuntime {
		t.Fatalf("cold state = %q, want %q", waiting.State, CertificateStatusWaitingForRuntime)
	}
	if waiting.StoragePath != storagePath {
		t.Fatalf("storage path = %q, want %q", waiting.StoragePath, storagePath)
	}

	manager.command = &exec.Cmd{Process: &os.Process{}}
	manager.done = make(chan error)
	issuing := manager.CertificateStatus(context.Background(), "dav.example.com")
	if issuing.State != CertificateStatusIssuing {
		t.Fatalf("running state = %q, want %q", issuing.State, CertificateStatusIssuing)
	}

	sourceCertificate, _ := writeTestCertificatePair(t, t.TempDir(), "dav.example.com")
	certificatePath := acmeCertificatePath(storagePath, "dav.example.com")
	if err := os.MkdirAll(filepath.Dir(certificatePath), 0o700); err != nil {
		t.Fatal(err)
	}
	certificate, err := os.ReadFile(sourceCertificate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certificatePath, certificate, 0o600); err != nil {
		t.Fatal(err)
	}

	ready := manager.CertificateStatus(context.Background(), "dav.example.com")
	if ready.State != CertificateStatusReady {
		t.Fatalf("ready state = %q, want %q", ready.State, CertificateStatusReady)
	}
	if ready.NotAfter == nil {
		t.Fatal("ready status did not include certificate expiry")
	}
}

func TestCertificateStatusDoesNotExposePrivateKeyPath(t *testing.T) {
	manager := &RuntimeManager{storagePath: t.TempDir()}
	status := manager.CertificateStatus(context.Background(), "dav.example.com")
	if status.CertificatePath == "" || filepath.Ext(status.CertificatePath) != ".crt" {
		t.Fatalf("certificate path = %q, want public .crt path", status.CertificatePath)
	}
}

func TestCertificateStatusReportsACMELogErrorsAsFailed(t *testing.T) {
	manager := &RuntimeManager{
		storagePath: t.TempDir(),
		certificateErrorReader: func(hostname string) bool {
			return hostname == "dav.example.com"
		},
	}
	status := manager.CertificateStatus(context.Background(), "dav.example.com")
	if status.State != CertificateStatusFailed {
		t.Fatalf("state = %q, want %q", status.State, CertificateStatusFailed)
	}
	if status.LastErrorCode != CertificateStatusACMEErrorCode {
		t.Fatalf("error code = %q, want %q", status.LastErrorCode, CertificateStatusACMEErrorCode)
	}
}
