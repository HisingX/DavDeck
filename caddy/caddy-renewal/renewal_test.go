package renewal

import "testing"

func TestNormalizeHostname(t *testing.T) {
	if got, err := normalizeHostname(" DAV.Example.COM. "); err != nil || got != "dav.example.com" {
		t.Fatalf("normalizeHostname() = %q, %v", got, err)
	}
	for _, value := range []string{"", ".example.com", "example.com/path", "example.com\nfoo"} {
		if _, err := normalizeHostname(value); err == nil {
			t.Errorf("normalizeHostname(%q) accepted invalid hostname", value)
		}
	}
}
