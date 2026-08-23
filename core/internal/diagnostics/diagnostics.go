// Package diagnostics composes safe, actionable health checks without exposing
// raw configuration, filesystem paths, credentials, or internal error text.
package diagnostics

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"davdeck.dev/davdeck/core/internal/buildinfo"
)

type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

type Result struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  Status `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type Report struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	AppVersion    string         `json:"app_version"`
	Build         buildinfo.Info `json:"build"`
	OS            string         `json:"os"`
	Architecture  string         `json:"architecture"`
	Sanitized     bool           `json:"sanitized"`
	Overall       Status         `json:"overall"`
	Results       []Result       `json:"results"`
}

type Check interface {
	ID() string
	Run(context.Context) Result
}

type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	checks     []Check
	clock      Clock
	appVersion string
	mu         sync.RWMutex
	latest     *Report
}

func NewService(checks []Check, clock Clock, appVersion string) *Service {
	ordered := append([]Check(nil), checks...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ID() < ordered[j].ID() })
	return &Service{checks: ordered, clock: clock, appVersion: appVersion}
}

func (s *Service) Run(ctx context.Context) Report {
	results := make([]Result, 0, len(s.checks))
	for _, check := range s.checks {
		if err := ctx.Err(); err != nil {
			results = append(results, Result{ID: check.ID(), Title: check.ID(), Status: StatusSkip, Code: "CHECK_CANCELLED", Message: "Check was cancelled"})
			continue
		}
		results = append(results, runSafely(ctx, check))
	}
	build := buildinfo.Current()
	build.Version = s.appVersion
	report := Report{SchemaVersion: 1, GeneratedAt: s.clock.Now().UTC().Format(time.RFC3339Nano), AppVersion: s.appVersion, Build: build, OS: runtime.GOOS, Architecture: runtime.GOARCH, Sanitized: true, Overall: overallStatus(results), Results: results}
	s.mu.Lock()
	s.latest = &report
	s.mu.Unlock()
	return report
}

func (s *Service) Latest() (Report, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest == nil {
		return Report{}, false
	}
	report := *s.latest
	report.Results = append([]Result(nil), s.latest.Results...)
	return report, true
}

func runSafely(ctx context.Context, check Check) (result Result) {
	result = Result{ID: check.ID(), Title: check.ID(), Status: StatusFail, Code: "CHECK_FAILED", Message: "Check failed unexpectedly"}
	defer func() {
		if recover() != nil {
			result = Result{ID: check.ID(), Title: check.ID(), Status: StatusFail, Code: "CHECK_PANIC", Message: "Check failed unexpectedly"}
		}
		if result.ID == "" {
			result.ID = check.ID()
		}
		if result.Title == "" {
			result.Title = result.ID
		}
		if result.Message == "" {
			result.Message = "No diagnostic detail was provided"
		}
	}()
	return check.Run(ctx)
}

func overallStatus(results []Result) Status {
	overall := StatusPass
	for _, result := range results {
		if result.Status == StatusFail {
			return StatusFail
		}
		if result.Status == StatusWarn || result.Status == StatusSkip {
			overall = StatusWarn
		}
	}
	return overall
}

func (r Report) Summary() string {
	return fmt.Sprintf("DavDeck diagnostics: %s (%d checks)", r.Overall, len(r.Results))
}
