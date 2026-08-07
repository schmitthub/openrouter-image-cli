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

// ImageRequest is the request body for POST /images.
// https://openrouter.ai/docs/api/api-reference/images/generate-an-image
// Zero-valued optional fields are omitted from the wire request so the
// API's own defaults apply.
type ImageRequest struct {
	// Model is the image generation model identifier (required),
	// e.g. "google/gemini-2.5-flash-image".
	Model string `json:"model"`
	// Prompt describes the desired image (required, non-empty).
	Prompt string `json:"prompt"`
	// N is the number of images to generate (1-10).
	N int `json:"n,omitempty"`
	// Size is a dimension shorthand ("2K", "4K") or explicit pixels
	// ("2048x2048").
	Size string `json:"size,omitempty"`
	// Quality is one of "auto", "low", "medium", "high".
	Quality string `json:"quality,omitempty"`
	// OutputFormat is one of "png", "jpeg", "webp", "svg".
	OutputFormat string `json:"output_format,omitempty"`
	// OutputCompression is a 0-100 compression level for jpeg/webp.
	OutputCompression *int `json:"output_compression,omitempty"`
	// AspectRatio is e.g. "1:1", "16:9", "9:16".
	AspectRatio string `json:"aspect_ratio,omitempty"`
	// Resolution is one of "512", "1K", "2K", "4K".
	Resolution string `json:"resolution,omitempty"`
	// Background is one of "auto", "transparent", "opaque".
	Background string `json:"background,omitempty"`
	// Seed makes generation deterministic where the model supports it.
	Seed *int64 `json:"seed,omitempty"`
	// InputReferences are base64 data or HTTP(S) URLs of reference images
	// for image-to-image generation (max 16).
	InputReferences []string `json:"input_references,omitempty"`
}

// ImageData is one generated image in an ImageResponse.
type ImageData struct {
	// B64JSON is the base64-encoded image payload.
	B64JSON string `json:"b64_json"`
	// MediaType is the payload's MIME type, e.g. "image/png".
	MediaType string `json:"media_type"`
}

// Usage reports token accounting and cost for a generation.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// Cost is in USD credits.
	Cost float64 `json:"cost"`
}

// ImageResponse is the success body of POST /images.
type ImageResponse struct {
	// Created is a unix timestamp.
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
	Usage   Usage       `json:"usage"`
}

// GenerateImage calls POST /images and returns the generated images.
// Non-2xx responses return a *APIError.
func (c *Client) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encoding image request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/images", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building image request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
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

	var imageResp ImageResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&imageResp); decErr != nil {
		return nil, fmt.Errorf("decoding image response: %w", decErr)
	}
	return &imageResp, nil
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
