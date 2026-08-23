package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validUser(t *testing.T) User {
	t.Helper()
	created := testTimestamp(t, "2026-08-20T00:00:00Z")
	updated := testTimestamp(t, "2026-08-20T00:01:00Z")
	return User{
		ID: testID, Username: "Alice", UsernameNormalized: "alice",
		PasswordHash: "$2a$12$example-hash", Enabled: true,
		CreatedAt: created, UpdatedAt: updated,
	}
}

func TestUserValidation(t *testing.T) {
	t.Parallel()
	if err := validUser(t).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name string
		edit func(*User)
		code ErrorCode
	}{
		{"empty username", func(user *User) { user.Username = "" }, CodeInvalidUsername},
		{"control character", func(user *User) { user.Username = "alice\n" }, CodeInvalidUsername},
		{"wrong normalization", func(user *User) { user.UsernameNormalized = "Alice" }, CodeInvalidUsername},
		{"missing hash", func(user *User) { user.PasswordHash = "" }, CodeInvalidPasswordHash},
		{"updated before created", func(user *User) { user.UpdatedAt = testTimestamp(t, "2026-08-19T23:59:00Z") }, CodeInvalidTimestamp},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			user := validUser(t)
			testCase.edit(&user)
			assertValidationCode(t, user.Validate(), testCase.code)
		})
	}
}

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()
	if got := NormalizeUsername("  ÄLICE  "); got != "älice" {
		t.Fatalf("NormalizeUsername() = %q", got)
	}
}

func TestUserJSONNeverIncludesPasswordHash(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(validUser(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "PasswordHash") || strings.Contains(string(body), "$2a$") {
		t.Fatalf("serialized user leaked password hash: %s", body)
	}
}

func TestShareValidationAndAbsolutePaths(t *testing.T) {
	t.Parallel()
	created := testTimestamp(t, "2026-08-20T00:00:00Z")
	share := Share{ID: testID, Name: "Photos", Slug: "family-photos", Path: "/srv/photos", Enabled: true, CreatedAt: created, UpdatedAt: created}
	if err := share.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"/srv/photos", `C:\`, `C:\DavDeck\Photos`, "D:/DavDeck/照片"} {
		if !IsAbsolutePath(value) {
			t.Errorf("IsAbsolutePath(%q) = false", value)
		}
	}
	for _, value := range []string{"srv/photos", "/srv/../etc", `C:Photos`, `C:\DavDeck\..\Windows`, `C:\DavDeck/Photos`, `\\server\share`, ""} {
		if IsAbsolutePath(value) {
			t.Errorf("IsAbsolutePath(%q) = true", value)
		}
	}
	for _, slug := range []string{"../photos", "Family Photos", "family_photos", "family--photos", "photos/2026"} {
		invalidShare := share
		invalidShare.Slug = slug
		assertValidationCode(t, invalidShare.Validate(), CodeInvalidShareSlug)
	}
	share.Path = "relative/photos"
	assertValidationCode(t, share.Validate(), CodeInvalidSharePath)
}

func TestSharePermissionValues(t *testing.T) {
	t.Parallel()
	stamp := testTimestamp(t, "2026-08-20T00:00:00Z")
	for _, permission := range []Permission{PermissionNone, PermissionRead, PermissionReadWrite} {
		value := SharePermission{ShareID: testID, UserID: testOtherID, Permission: permission, CreatedAt: stamp, UpdatedAt: stamp}
		if err := value.Validate(); err != nil {
			t.Errorf("permission %q: %v", permission, err)
		}
	}
	invalidPermission := SharePermission{ShareID: testID, UserID: testOtherID, Permission: "WRITE", CreatedAt: stamp, UpdatedAt: stamp}
	assertValidationCode(t, invalidPermission.Validate(), CodeInvalidPermission)
}

func assertValidationCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
	if validationError.Code != code {
		t.Fatalf("code = %q, want %q", validationError.Code, code)
	}
}
