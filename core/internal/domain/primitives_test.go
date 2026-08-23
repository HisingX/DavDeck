package domain

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

const (
	testID      ID = "018f47a2-9b3c-7def-8123-456789abcdef"
	testOtherID ID = "118f47a2-9b3c-7def-8123-456789abcdef"
)

var (
	_ Validator = User{}
	_ Validator = Share{}
	_ Validator = SharePermission{}
	_ Validator = TLSProfile{}
	_ Validator = ServerSettings{}
	_ Validator = ConfigRevision{}
)

func testTimestamp(t *testing.T, text string) Timestamp {
	t.Helper()
	value, err := ParseTimestamp(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestParseID(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		value string
		valid bool
	}{
		{string(testID), true},
		{"018F47A2-9B3C-7DEF-8123-456789ABCDEF", false},
		{"018f47a29b3c7def8123456789abcdef", false},
		{"not-an-id", false},
	} {
		_, err := ParseID(testCase.value)
		if (err == nil) != testCase.valid {
			t.Errorf("ParseID(%q) error = %v, valid = %v", testCase.value, err, testCase.valid)
		}
	}
}

func TestTimestampNormalizesAndSerializesUTC(t *testing.T) {
	t.Parallel()
	value, err := ParseTimestamp("2026-08-20T08:30:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != "2026-08-20T00:30:00Z" {
		t.Fatalf("String() = %q", got)
	}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Timestamp
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Time().Equal(value.Time()) || decoded.Time().Location() != time.UTC {
		t.Fatalf("decoded timestamp = %v", decoded.Time())
	}
}

func TestEntityRejectsNonUTCTimestamp(t *testing.T) {
	t.Parallel()
	user := validUser(t)
	user.CreatedAt = Timestamp(time.Date(2026, 8, 20, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60)))
	assertValidationCode(t, user.Validate(), CodeInvalidTimestamp)
}

func TestValidationErrorCarriesStableCodeAndField(t *testing.T) {
	t.Parallel()
	_, err := ParseID("bad")
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("error type = %T", err)
	}
	if validationError.Code != CodeInvalidID || validationError.Field != "id" {
		t.Fatalf("validation error = %#v", validationError)
	}
}
