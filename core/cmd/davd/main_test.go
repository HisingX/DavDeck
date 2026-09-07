package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEndpoint(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "runtime", "management.endpoint")
	if err := writeEndpoint(path, "http://127.0.0.1:12345"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "http://127.0.0.1:12345\n" {
		t.Fatalf("endpoint content = %q", body)
	}
}
