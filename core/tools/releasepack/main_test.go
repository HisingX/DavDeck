package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArchivesAreDeterministicAndChecksummed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "DavDeck-1.0.0-linux-amd64")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "davd"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1_700_000_000, 0).UTC()
	for _, format := range []string{"tar.gz", "zip"} {
		t.Run(format, func(t *testing.T) {
			first := filepath.Join(t.TempDir(), "first."+format)
			second := filepath.Join(t.TempDir(), "second."+format)
			var firstErr, secondErr error
			if format == "tar.gz" {
				firstErr, secondErr = writeTarGzip(root, first, stamp), writeTarGzip(root, second, stamp)
			} else {
				firstErr, secondErr = writeZip(root, first, stamp), writeZip(root, second, stamp)
			}
			if firstErr != nil || secondErr != nil {
				t.Fatalf("archive errors = %v, %v", firstErr, secondErr)
			}
			firstBody, _ := os.ReadFile(first)
			secondBody, _ := os.ReadFile(second)
			if !bytes.Equal(firstBody, secondBody) {
				t.Fatal("archives are not deterministic")
			}
			if err := writeChecksum(first); err != nil {
				t.Fatal(err)
			}
			checksum, err := os.ReadFile(first + ".sha256")
			if err != nil || !bytes.Contains(checksum, []byte(filepath.Base(first))) {
				t.Fatalf("checksum = %q, error = %v", checksum, err)
			}
		})
	}
}
