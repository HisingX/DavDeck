package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Timestamp stores an instant normalized to UTC and serializes as RFC3339Nano.
type Timestamp time.Time

// NewTimestamp normalizes a non-zero time to UTC.
func NewTimestamp(value time.Time) (Timestamp, error) {
	if value.IsZero() {
		return Timestamp{}, invalid(CodeInvalidTimestamp, "timestamp", "must not be zero")
	}
	return Timestamp(value.UTC()), nil
}

// ParseTimestamp parses RFC3339/RFC3339Nano text and normalizes it to UTC.
func ParseTimestamp(value string) (Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return Timestamp{}, invalid(CodeInvalidTimestamp, "timestamp", "must use RFC3339 format")
	}
	return NewTimestamp(parsed)
}

// Time returns the standard-library UTC time value.
func (t Timestamp) Time() time.Time { return time.Time(t) }

func (t Timestamp) String() string {
	if t.Time().IsZero() {
		return ""
	}
	return t.Time().UTC().Format(time.RFC3339Nano)
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if err := validateTimestamp("timestamp", t); err != nil {
		return nil, err
	}
	return json.Marshal(t.String())
}

func (t *Timestamp) UnmarshalJSON(body []byte) error {
	var text string
	if err := json.Unmarshal(body, &text); err != nil {
		return fmt.Errorf("decode timestamp: %w", err)
	}
	parsed, err := ParseTimestamp(text)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

func validateTimestamp(field string, value Timestamp) error {
	instant := value.Time()
	if instant.IsZero() || instant.Location() != time.UTC {
		return invalid(CodeInvalidTimestamp, field, "must be a non-zero UTC timestamp")
	}
	return nil
}

func validateTimeRange(createdField string, created Timestamp, updatedField string, updated Timestamp) error {
	if err := validateTimestamp(createdField, created); err != nil {
		return err
	}
	if err := validateTimestamp(updatedField, updated); err != nil {
		return err
	}
	if updated.Time().Before(created.Time()) {
		return invalid(CodeInvalidTimestamp, updatedField, "must not be before creation time")
	}
	return nil
}
