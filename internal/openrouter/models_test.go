package openrouter_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

func TestListImageModels(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data": [{
			"id": "qwen/qwen-image-3",
			"name": "Qwen: Qwen Image 3",
			"created": 1785894548,
			"architecture": {"input_modalities": ["text", "image"], "output_modalities": ["image"]},
			"supported_parameters": {
				"resolution": {"type": "enum", "values": ["1K", "2K"]},
				"n": {"type": "range", "min": 1, "max": 6},
				"seed": {"type": "boolean"}
			},
			"supports_streaming": false,
			"endpoints": "/api/v1/images/models/qwen/qwen-image-3/endpoints"
		}]}`))
	}))
	defer srv.Close()

	client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	models, err := client.ListImageModels(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "/images/models", gotPath)
	require.Len(t, models, 1)
	m := models[0]
	assert.Equal(t, "qwen/qwen-image-3", m.ID)
	assert.Equal(t, []string{"text", "image"}, m.Architecture.InputModalities)
	assert.Equal(t, []string{"1K", "2K"}, m.SupportedParameters["resolution"].Values)
	require.NotNil(t, m.SupportedParameters["n"].Min)
	assert.Equal(t, 1, *m.SupportedParameters["n"].Min)
	assert.Equal(t, "boolean", m.SupportedParameters["seed"].Type)
}

func TestGetImageModelEndpoints(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id": "qwen/qwen-image-3", "endpoints": [{
			"provider_name": "Alibaba Cloud Int.",
			"provider_slug": "alibaba",
			"provider_tag": "alibaba",
			"supported_parameters": {"seed": {"type": "boolean"}},
			"allowed_passthrough_parameters": [],
			"supports_streaming": false,
			"pricing": [
				{"billable": "output_image", "unit": "image", "cost_usd": 0.03, "variant": "1k"}
			]
		}]}`))
	}))
	defer srv.Close()

	client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	eps, err := client.GetImageModelEndpoints(t.Context(), "qwen/qwen-image-3")
	require.NoError(t, err)

	assert.Equal(t, "/images/models/qwen/qwen-image-3/endpoints", gotPath)
	assert.Equal(t, "qwen/qwen-image-3", eps.ID)
	require.Len(t, eps.Endpoints, 1)
	ep := eps.Endpoints[0]
	assert.Equal(t, "alibaba", ep.ProviderSlug)
	require.Len(t, ep.Pricing, 1)
	assert.InDelta(t, 0.03, ep.Pricing[0].CostUSD, 1e-9)
	assert.Equal(t, "1k", ep.Pricing[0].Variant)
}

func TestGetImageModelEndpoints_EmptyID(t *testing.T) {
	client := openrouter.New("k")
	_, err := client.GetImageModelEndpoints(t.Context(), "  ")
	require.Error(t, err)
}
