package dnspod

import (
	"testing"
	"time"

	"github.com/libdns/libdns"
)

func TestWithRecordIDPreservesRequestedRecord(t *testing.T) {
	requested := Record{base: libdns.RR{
		Type: "TXT",
		Name: "_acme-challenge.testdav",
		Data: "acme-challenge-value",
		TTL:  600 * time.Second,
	}}

	got := (&Provider{}).withRecordID(requested, "123456")
	if got.(Record).ID != "123456" {
		t.Fatalf("record ID = %q, want %q", got.(Record).ID, "123456")
	}
	if got.RR() != requested.RR() {
		t.Fatalf("record data was not preserved: got %#v, want %#v", got.RR(), requested.RR())
	}
}
