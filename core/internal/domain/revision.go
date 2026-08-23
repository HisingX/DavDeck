package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// RevisionValidationStatus records the result of real Caddy validation.
type RevisionValidationStatus string

const (
	RevisionValidationPending RevisionValidationStatus = "PENDING"
	RevisionValidationValid   RevisionValidationStatus = "VALID"
	RevisionValidationInvalid RevisionValidationStatus = "INVALID"
)

func (s RevisionValidationStatus) Valid() bool {
	switch s {
	case RevisionValidationPending, RevisionValidationValid, RevisionValidationInvalid:
		return true
	default:
		return false
	}
}

// RevisionApplyStatus records whether a validated revision reached runtime.
type RevisionApplyStatus string

const (
	RevisionApplyNotApplied RevisionApplyStatus = "NOT_APPLIED"
	RevisionApplyApplied    RevisionApplyStatus = "APPLIED"
	RevisionApplyFailed     RevisionApplyStatus = "FAILED"
)

func (s RevisionApplyStatus) Valid() bool {
	switch s {
	case RevisionApplyNotApplied, RevisionApplyApplied, RevisionApplyFailed:
		return true
	default:
		return false
	}
}

// ConfigRevision records metadata for one generated configuration. Raw
// JSON is excluded from generic JSON serialization to avoid accidental debug
// exposure; dedicated interfaces may expose a sanitized representation later.
type ConfigRevision struct {
	ID               ID
	Number           uint64
	CreatedAt        Timestamp
	ConfigJSON       []byte `json:"-"`
	ConfigHash       string
	ValidationStatus RevisionValidationStatus
	ApplyStatus      RevisionApplyStatus
	AppVersion       string
	ErrorCode        string
	ErrorSummary     string
}

func (r ConfigRevision) Validate() error {
	if err := validateID("id", r.ID); err != nil {
		return err
	}
	if r.Number == 0 {
		return invalid(CodeInvalidRevision, "number", "must be greater than zero")
	}
	if err := validateTimestamp("created_at", r.CreatedAt); err != nil {
		return err
	}
	if !json.Valid(r.ConfigJSON) {
		return invalid(CodeInvalidConfig, "config_json", "must contain valid JSON")
	}
	if !validSHA256(r.ConfigHash) || r.ConfigHash != HashConfigJSON(r.ConfigJSON) {
		return invalid(CodeInvalidConfigHash, "config_hash", "must match the generated JSON SHA-256 digest")
	}
	if !r.ValidationStatus.Valid() {
		return invalid(CodeInvalidRevisionStatus, "validation_status", "contains an unsupported status")
	}
	if !r.ApplyStatus.Valid() {
		return invalid(CodeInvalidRevisionStatus, "apply_status", "contains an unsupported status")
	}
	if r.ValidationStatus != RevisionValidationValid && r.ApplyStatus == RevisionApplyApplied {
		return invalid(CodeInvalidRevisionStatus, "apply_status", "only a valid revision can be applied")
	}
	if strings.TrimSpace(r.AppVersion) == "" || containsControl(r.AppVersion) {
		return invalid(CodeInvalidRevision, "app_version", "must identify the DavDeck version")
	}
	failed := r.ValidationStatus == RevisionValidationInvalid || r.ApplyStatus == RevisionApplyFailed
	if failed && (strings.TrimSpace(r.ErrorCode) == "" || strings.TrimSpace(r.ErrorSummary) == "") {
		return invalid(CodeInvalidRevisionStatus, "error_code", "failed revisions require a safe error code and summary")
	}
	if !failed && (r.ErrorCode != "" || r.ErrorSummary != "") {
		return invalid(CodeInvalidRevisionStatus, "error_code", "successful or pending revisions must not contain errors")
	}
	if containsControl(r.ErrorCode) || containsControl(r.ErrorSummary) {
		return invalid(CodeInvalidRevisionStatus, "error_summary", "must not contain control characters")
	}
	return nil
}

// HashConfigJSON returns the lowercase SHA-256 digest of the exact generated
// configuration bytes.
func HashConfigJSON(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
