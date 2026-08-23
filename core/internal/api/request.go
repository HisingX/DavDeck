package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

func decodeJSON(writer http.ResponseWriter, request *http.Request, destination any) *APIError {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &APIError{Code: ErrorInvalidRequest, Message: "Content-Type must be application/json"}
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return &APIError{Code: ErrorInvalidRequest, Message: "Request body is too large"}
		}
		return &APIError{Code: ErrorInvalidRequest, Message: "Request body must contain valid JSON", Details: map[string]any{"reason": fmt.Sprintf("%v", err)}}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &APIError{Code: ErrorInvalidRequest, Message: "Request body must contain one JSON value"}
	}
	return nil
}
