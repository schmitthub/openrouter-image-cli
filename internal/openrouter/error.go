package openrouter

import "fmt"

// APIError is a non-2xx response from the OpenRouter API.
// https://openrouter.ai/docs/api/reference/errors
type APIError struct {
	// StatusCode is the HTTP status of the response.
	StatusCode int
	// Code is the error code from the response body (usually mirrors the
	// HTTP status).
	Code int
	// Message is the human-readable error message from the response body.
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("openrouter: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("openrouter: HTTP %d: %s", e.StatusCode, e.Message)
}

// errorBody is the wire shape of an OpenRouter error response.
type errorBody struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
