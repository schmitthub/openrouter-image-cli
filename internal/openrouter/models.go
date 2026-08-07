package openrouter

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// ParamSpec describes one supported request parameter of an image model.
// Exactly one shape applies per Type: "enum" carries Values, "range" carries
// Min/Max, "boolean" carries neither.
type ParamSpec struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Min    *int     `json:"min,omitempty"`
	Max    *int     `json:"max,omitempty"`
}

// Architecture lists a model's input/output modalities.
type Architecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// ImageModel is one entry of GET /images/models.
type ImageModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Created is a unix timestamp.
	Created             int64                `json:"created"`
	Architecture        Architecture         `json:"architecture"`
	SupportedParameters map[string]ParamSpec `json:"supported_parameters"`
	SupportsStreaming   bool                 `json:"supports_streaming"`
	// Endpoints is the API path of the model's per-provider detail resource.
	Endpoints string `json:"endpoints"`
}

// Price is one pricing line of a provider endpoint.
type Price struct {
	// Billable is what is charged, e.g. "input_image", "output_image".
	Billable string `json:"billable"`
	// Unit is the charged unit, e.g. "image".
	Unit    string  `json:"unit"`
	CostUSD float64 `json:"cost_usd"`
	// Variant qualifies the price, e.g. a "2k" resolution tier.
	Variant string `json:"variant,omitempty"`
}

// ProviderEndpoint is one provider's offering of a model.
type ProviderEndpoint struct {
	ProviderName                 string               `json:"provider_name"`
	ProviderSlug                 string               `json:"provider_slug"`
	ProviderTag                  string               `json:"provider_tag"`
	SupportedParameters          map[string]ParamSpec `json:"supported_parameters"`
	AllowedPassthroughParameters []string             `json:"allowed_passthrough_parameters"`
	SupportsStreaming            bool                 `json:"supports_streaming"`
	Pricing                      []Price              `json:"pricing"`
}

// ModelEndpoints is the response of GET /images/models/{id}/endpoints.
type ModelEndpoints struct {
	ID        string             `json:"id"`
	Endpoints []ProviderEndpoint `json:"endpoints"`
}

// imageModelList is the wire envelope of GET /images/models.
type imageModelList struct {
	Data []ImageModel `json:"data"`
}

// ListImageModels calls GET /images/models and returns every image model,
// in API order.
func (c *Client) ListImageModels(ctx context.Context) ([]ImageModel, error) {
	var list imageModelList
	if err := c.getJSON(ctx, "/images/models", &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

// GetImageModelEndpoints calls GET /images/models/{id}/endpoints for a model
// id like "qwen/qwen-image-3" and returns the per-provider detail.
func (c *Client) GetImageModelEndpoints(ctx context.Context, id string) (*ModelEndpoints, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("model id is empty")
	}
	// Escape each path segment but keep the author/slug separator.
	segments := strings.Split(id, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	path := "/images/models/" + strings.Join(segments, "/") + "/endpoints"

	var eps ModelEndpoints
	if err := c.getJSON(ctx, path, &eps); err != nil {
		return nil, err
	}
	return &eps, nil
}
