package buildinfo

import (
	"runtime"
	"testing"
)

func TestCurrentReportsSafeCompleteDevelopmentMetadata(t *testing.T) {
	info := Current()
	if info.Product != "DavDeck" || info.Version == "" || info.GitCommit == "" || info.BuildDate == "" {
		t.Fatalf("metadata = %#v", info)
	}
	if info.GoVersion != runtime.Version() || info.TargetOS != runtime.GOOS || info.TargetArch != runtime.GOARCH {
		t.Fatalf("runtime metadata = %#v", info)
	}
}
