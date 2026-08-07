package openrouter

import (
	"context"
	"net/http"
)

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
	// InputReferences are reference images for image-to-image generation
	// (max 16). The API requires each entry as a structured object, not a
	// bare string; build entries with NewInputReference.
	InputReferences []InputReference `json:"input_references,omitempty"`
}

// InputReference is one reference image in an ImageRequest. The API accepts
// only this object form: {"type":"image_url","image_url":{"url":...}}.
type InputReference struct {
	// Type is always "image_url".
	Type string `json:"type"`
	// ImageURL wraps the reference location.
	ImageURL ImageURL `json:"image_url"`
}

// ImageURL locates a reference image: an HTTP(S) URL or a base64 data URI.
type ImageURL struct {
	URL string `json:"url"`
}

// NewInputReference wraps an HTTP(S) URL or base64 data URI in the object
// form the API requires for input_references.
func NewInputReference(url string) InputReference {
	return InputReference{
		Type:     "image_url",
		ImageURL: ImageURL{URL: url},
	}
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
	var imageResp ImageResponse
	if err := c.doJSON(ctx, http.MethodPost, "/images", req, &imageResp); err != nil {
		return nil, err
	}
	return &imageResp, nil
}
