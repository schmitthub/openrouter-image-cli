package info

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

const infoBody = `{"id": "qwen/qwen-image-3", "endpoints": [{
	"provider_name": "Alibaba Cloud Int.",
	"provider_slug": "alibaba",
	"supported_parameters": {
		"aspect_ratio": {"type": "enum", "values": ["1:1", "16:9"]},
		"n": {"type": "range", "min": 1, "max": 6},
		"seed": {"type": "boolean"}
	},
	"supports_streaming": false,
	"pricing": [
		{"billable": "output_image", "unit": "image", "cost_usd": 0.03, "variant": "1k"},
		{"billable": "input_image", "unit": "image", "cost_usd": 0.003}
	]
}]}`

func fakeClient(t *testing.T) func() (*openrouter.Client, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(infoBody))
	}))
	t.Cleanup(srv.Close)
	client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	return func() (*openrouter.Client, error) { return client, nil }
}

func Test_runInfo(t *testing.T) {
	t.Run("human output", func(t *testing.T) {
		ios, _, stdout, _ := iostreams.Test()
		opts := &InfoOptions{IOStreams: ios, OpenRouter: fakeClient(t), Model: "qwen/qwen-image-3"}

		require.NoError(t, runInfo(t.Context(), opts))

		out := stdout.String()
		assert.Contains(t, out, "qwen/qwen-image-3")
		assert.Contains(t, out, "Alibaba Cloud Int. (alibaba)")
		assert.Contains(t, out, "aspect_ratio: 1:1, 16:9")
		assert.Contains(t, out, "n: 1-6")
		assert.Contains(t, out, "seed: boolean")
		assert.Contains(t, out, "output_image (1k): $0.03 per image")
		assert.Contains(t, out, "input_image: $0.003 per image")
	})

	t.Run("json output", func(t *testing.T) {
		ios, _, stdout, _ := iostreams.Test()
		opts := &InfoOptions{
			IOStreams: ios, OpenRouter: fakeClient(t),
			Model: "qwen/qwen-image-3", JSON: true,
		}

		require.NoError(t, runInfo(t.Context(), opts))

		assert.Contains(t, stdout.String(), `"provider_slug": "alibaba"`)
	})
}
