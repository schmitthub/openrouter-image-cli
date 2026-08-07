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
	// Provider controls provider routing and carries provider-specific
	// passthrough options.
	Provider *ProviderPreferences `json:"provider,omitempty"`
}

// ProviderPreferences is the "provider" object of an ImageRequest
// (ImageGenerationProviderPreferences in the API schema): routing
// preferences plus provider-specific passthrough configuration.
type ProviderPreferences struct {
	// Order is an ordered list of provider slugs; the router attempts the
	// first available provider and falls back to the next.
	Order []string `json:"order,omitempty"`
	// Only is a list of provider slugs to allow, merged with the
	// account-wide allowed providers.
	Only []string `json:"only,omitempty"`
	// Ignore is a list of provider slugs to exclude, merged with the
	// account-wide ignored providers.
	Ignore []string `json:"ignore,omitempty"`
	// Sort is one of "price", "throughput", "latency", "exacto". When set,
	// no load balancing is performed.
	Sort string `json:"sort,omitempty"`
	// AllowFallbacks, when false, uses only the primary (or Order-listed)
	// provider and surfaces the upstream error instead of failing over.
	// The API default is true.
	AllowFallbacks *bool `json:"allow_fallbacks,omitempty"`
	// Options holds provider-specific passthrough parameters keyed by
	// provider slug. The accepted keys per provider are listed in the
	// endpoint's AllowedPassthroughParameters; unrecognized keys are
	// silently dropped by the API.
	Options map[string]map[string]any `json:"options,omitempty"`
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
	// ProviderName is the provider that served the generation, taken from
	// the X-Provider-Name response header — the JSON body carries no
	// provider identity. Empty when the header is absent.
	ProviderName string `json:"-"`
}

// GenerateImage calls POST /images and returns the generated images.
// Non-2xx responses return a *APIError.
func (c *Client) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error) {
	var imageResp ImageResponse
	header, err := c.doJSONHeader(ctx, http.MethodPost, "/images", req, &imageResp)
	if err != nil {
		return nil, err
	}
	imageResp.ProviderName = header.Get("X-Provider-Name")
	return &imageResp, nil
}
