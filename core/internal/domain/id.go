package domain

import "strings"

// ID is a canonical lowercase UUID string used as an opaque entity identifier.
type ID string

// ParseID validates and returns a canonical UUID identifier.
func ParseID(value string) (ID, error) {
	id := ID(value)
	if err := validateID("id", id); err != nil {
		return "", err
	}
	return id, nil
}

func validateID(field string, value ID) error {
	text := string(value)
	if len(text) != 36 || text[8] != '-' || text[13] != '-' || text[18] != '-' || text[23] != '-' || text != strings.ToLower(text) {
		return invalid(CodeInvalidID, field, "must be a canonical lowercase UUID")
	}
	for index, character := range text {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return invalid(CodeInvalidID, field, "must be a canonical lowercase UUID")
		}
	}
	return nil
}
