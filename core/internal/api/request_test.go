package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONValidation(t *testing.T) {
	t.Parallel()
	type input struct {
		Name string `json:"name"`
	}
	for _, testCase := range []struct {
		name        string
		contentType string
		body        string
		valid       bool
	}{
		{"valid", "application/json; charset=utf-8", `{"name":"DavDeck"}`, true},
		{"wrong content type", "text/plain", `{}`, false},
		{"unknown field", "application/json", `{"unknown":true}`, false},
		{"multiple values", "application/json", `{} {}`, false},
		{"invalid", "application/json", `{`, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", testCase.contentType)
			var value input
			err := decodeJSON(httptest.NewRecorder(), request, &value)
			if (err == nil) != testCase.valid {
				t.Fatalf("error = %#v, valid = %v", err, testCase.valid)
			}
		})
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	t.Parallel()
	body := bytes.Repeat([]byte(" "), maxRequestBodyBytes+1)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	var value map[string]any
	err := decodeJSON(httptest.NewRecorder(), request, &value)
	if err == nil || err.Message != "Request body is too large" {
		t.Fatalf("error = %#v", err)
	}
}
