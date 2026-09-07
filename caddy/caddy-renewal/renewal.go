package renewal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
)

const (
	renewalEndpoint       = "/davdeck/tls/renew"
	renewalCancelEndpoint = renewalEndpoint + "/cancel"

	renewalStateIssuing   = "ISSUING"
	renewalStateCanceling = "CANCELING"
	renewalStateCanceled  = "CANCELED"
	renewalStateSucceeded = "SUCCEEDED"
	renewalStateFailed    = "FAILED"

	renewalErrorCode = "ACME_CERTIFICATE_ISSUANCE_FAILED"
)

// certificateRenewer is implemented by DavDeck's patched caddytls app. It is
// deliberately kept as a small interface so this admin module can still be
// unit-tested against the stock Caddy dependency.
type certificateRenewer interface {
	ForceRenewCertificate(context.Context, string) error
}

type renewalAdmin struct {
	mu   sync.Mutex
	jobs map[string]*renewalJob
}

type renewalJob struct {
	hostname        string
	state           string
	message         string
	errorCode       string
	cancelRequested bool
	cancel          context.CancelFunc
}

type renewalRequest struct {
	Hostname string `json:"hostname"`
}

type renewalResponse struct {
	Hostname  string `json:"hostname"`
	State     string `json:"state"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

func init() {
	caddy.RegisterModule(renewalAdmin{})
}

// CaddyModule returns the module information.
func (renewalAdmin) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "admin.api.davdeck_renewal",
		New: func() caddy.Module { return new(renewalAdmin) },
	}
}

// Provision initializes the in-memory operation registry. The registry is
// owned by the admin endpoint and survives normal Caddy config reloads.
func (a *renewalAdmin) Provision(caddy.Context) error {
	a.jobs = make(map[string]*renewalJob)
	return nil
}

// Routes returns DavDeck's loopback-only Caddy Admin API endpoints.
func (a *renewalAdmin) Routes() []caddy.AdminRoute {
	return []caddy.AdminRoute{
		{Pattern: renewalEndpoint, Handler: caddy.AdminHandlerFunc(a.handleRenewal)},
		{Pattern: renewalCancelEndpoint, Handler: caddy.AdminHandlerFunc(a.handleCancel)},
	}
}

func (a *renewalAdmin) handleRenewal(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodPost:
		request, err := decodeRenewalRequest(r)
		if err != nil {
			return caddy.APIError{HTTPStatus: http.StatusBadRequest, Err: err}
		}
		hostname, err := normalizeHostname(request.Hostname)
		if err != nil {
			return caddy.APIError{HTTPStatus: http.StatusBadRequest, Err: err}
		}
		return a.startRenewal(w, r, hostname)
	case http.MethodGet:
		hostname, err := normalizeHostname(r.URL.Query().Get("hostname"))
		if err != nil {
			return caddy.APIError{HTTPStatus: http.StatusBadRequest, Err: err}
		}
		return a.writeStatus(w, hostname)
	default:
		return caddy.APIError{HTTPStatus: http.StatusMethodNotAllowed, Err: fmt.Errorf("method not allowed: %s", r.Method)}
	}
}

func (a *renewalAdmin) handleCancel(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return caddy.APIError{HTTPStatus: http.StatusMethodNotAllowed, Err: fmt.Errorf("method not allowed: %s", r.Method)}
	}
	request, err := decodeRenewalRequest(r)
	if err != nil {
		return caddy.APIError{HTTPStatus: http.StatusBadRequest, Err: err}
	}
	hostname, err := normalizeHostname(request.Hostname)
	if err != nil {
		return caddy.APIError{HTTPStatus: http.StatusBadRequest, Err: err}
	}

	a.mu.Lock()
	job := a.jobs[hostname]
	if job == nil {
		a.mu.Unlock()
		return caddy.APIError{HTTPStatus: http.StatusNotFound, Err: fmt.Errorf("renewal operation not found")}
	}
	if job.state != renewalStateIssuing && job.state != renewalStateCanceling {
		response := snapshot(job)
		a.mu.Unlock()
		return writeRenewalResponse(w, http.StatusOK, response)
	}
	job.cancelRequested = true
	job.state = renewalStateCanceling
	job.message = "Certificate renewal cancellation requested"
	cancel := job.cancel
	response := snapshot(job)
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return writeRenewalResponse(w, http.StatusAccepted, response)
}

func (a *renewalAdmin) startRenewal(w http.ResponseWriter, r *http.Request, hostname string) error {
	tlsApp, err := activeCertificateRenewer()
	if err != nil {
		return caddy.APIError{HTTPStatus: http.StatusServiceUnavailable, Err: err}
	}

	a.mu.Lock()
	if job := a.jobs[hostname]; job != nil && (job.state == renewalStateIssuing || job.state == renewalStateCanceling) {
		response := snapshot(job)
		a.mu.Unlock()
		return writeRenewalResponse(w, http.StatusConflict, response)
	}
	parent := caddy.ActiveContext().Context
	if parent == nil {
		parent = context.Background()
	}
	operationContext, cancel := context.WithCancel(parent)
	job := &renewalJob{
		hostname: hostname,
		state:    renewalStateIssuing,
		message:  "Caddy is renewing the managed certificate",
		cancel:   cancel,
	}
	a.jobs[hostname] = job
	response := snapshot(job)
	a.mu.Unlock()

	go a.runRenewal(job, tlsApp, operationContext)
	return writeRenewalResponse(w, http.StatusAccepted, response)
}

func (a *renewalAdmin) runRenewal(job *renewalJob, tlsApp certificateRenewer, ctx context.Context) {
	err := tlsApp.ForceRenewCertificate(ctx, job.hostname)

	a.mu.Lock()
	defer a.mu.Unlock()
	if job.cancelRequested || errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		job.state = renewalStateCanceled
		job.message = "Certificate renewal canceled"
		job.errorCode = ""
		return
	}
	if err != nil {
		job.state = renewalStateFailed
		job.message = "Caddy could not renew the managed certificate; check the Caddy logs"
		job.errorCode = renewalErrorCode
		return
	}
	job.state = renewalStateSucceeded
	job.message = "The managed certificate was renewed successfully"
	job.errorCode = ""
}

func (a *renewalAdmin) writeStatus(w http.ResponseWriter, hostname string) error {
	a.mu.Lock()
	job := a.jobs[hostname]
	if job == nil {
		a.mu.Unlock()
		return caddy.APIError{HTTPStatus: http.StatusNotFound, Err: fmt.Errorf("renewal operation not found")}
	}
	response := snapshot(job)
	a.mu.Unlock()
	return writeRenewalResponse(w, http.StatusOK, response)
}

func activeCertificateRenewer() (certificateRenewer, error) {
	app, err := caddy.ActiveContext().AppIfConfigured("tls")
	if err != nil {
		return nil, fmt.Errorf("TLS app is unavailable: %v", err)
	}
	renewer, ok := app.(certificateRenewer)
	if !ok {
		return nil, errors.New("this Caddy build does not support forced certificate renewal")
	}
	return renewer, nil
}

func decodeRenewalRequest(r *http.Request) (renewalRequest, error) {
	var request renewalRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return renewalRequest{}, fmt.Errorf("invalid renewal request: %w", err)
	}
	return request, nil
}

func normalizeHostname(value string) (string, error) {
	hostname := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if hostname == "" || len(hostname) > 253 || strings.HasPrefix(hostname, ".") || strings.ContainsAny(hostname, "/?#\\\"'\t\r\n") {
		return "", errors.New("hostname is invalid")
	}
	return hostname, nil
}

func snapshot(job *renewalJob) renewalResponse {
	return renewalResponse{Hostname: job.hostname, State: job.state, Message: job.message, ErrorCode: job.errorCode}
}

func writeRenewalResponse(w http.ResponseWriter, status int, response renewalResponse) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(response)
}

var (
	_ caddy.Module      = (*renewalAdmin)(nil)
	_ caddy.Provisioner = (*renewalAdmin)(nil)
	_ caddy.AdminRouter = (*renewalAdmin)(nil)
)
