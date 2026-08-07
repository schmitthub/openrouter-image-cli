package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// maxErrorBody caps how much of an error response body is read.
const maxErrorBody = 1 << 20 // 1 MiB

// doJSON performs an API request with an optional JSON body and decodes the
// JSON response into out. Non-2xx responses return a *APIError.
func (c *Client) doJSON(ctx context.Context, method, path string, reqBody, out any) error {
	_, err := c.doJSONHeader(ctx, method, path, reqBody, out)
	return err
}

// doJSONHeader is doJSON, additionally returning the response headers for
// callers that need response metadata OpenRouter only exposes there
// (e.g. X-Provider-Name).
func (c *Client) doJSONHeader(
	ctx context.Context, method, path string, reqBody, out any,
) (http.Header, error) {
	var body io.Reader
	if reqBody != nil {
		encoded, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("encoding request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if reqBody != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if c.referer != "" {
		// OpenRouter documents this exact header spelling; Go canonicalizes
		// it to Http-Referer on the wire either way (headers are
		// case-insensitive per RFC 9110).
		httpReq.Header.Set("HTTP-Referer", c.referer) //nolint:canonicalheader // documented name
	}
	if c.title != "" {
		httpReq.Header.Set("X-Title", c.title)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling openrouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, parseAPIError(resp)
	}

	if decErr := json.NewDecoder(resp.Body).Decode(out); decErr != nil {
		return nil, fmt.Errorf("decoding response: %w", decErr)
	}
	return resp.Header, nil
}

// getJSON performs a GET request and decodes the JSON response into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, out)
}

// parseAPIError converts a non-2xx response into a *APIError, falling back
// to the bare HTTP status when the body is not the documented error shape.
func parseAPIError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode, Code: 0, Message: ""}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		return apiErr
	}
	var eb errorBody
	if jsonErr := json.Unmarshal(body, &eb); jsonErr == nil {
		apiErr.Code = eb.Error.Code
		apiErr.Message = eb.Error.Message
	}
	return apiErr
}
