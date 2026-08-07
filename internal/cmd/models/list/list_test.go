package list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

func fakeClient(t *testing.T, body string) func() (*openrouter.Client, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	return func() (*openrouter.Client, error) { return client, nil }
}

const listBody = `{"data": [
	{"id": "a/one", "name": "One", "architecture": {"input_modalities": ["text"]}, "supports_streaming": true},
	{"id": "b/two", "name": "Two", "architecture": {"input_modalities": ["text", "image"]}, "supports_streaming": false}
]}`

func Test_runList(t *testing.T) {
	t.Run("table output", func(t *testing.T) {
		ios, _, stdout, _ := iostreams.Test()
		opts := &ListOptions{IOStreams: ios, OpenRouter: fakeClient(t, listBody)}

		require.NoError(t, runList(t.Context(), opts))

		out := stdout.String()
		assert.Contains(t, out, "ID")
		assert.Contains(t, out, "a/one")
		assert.Contains(t, out, "text,image")
		assert.Contains(t, out, "true")
	})

	t.Run("json output", func(t *testing.T) {
		ios, _, stdout, _ := iostreams.Test()
		opts := &ListOptions{IOStreams: ios, OpenRouter: fakeClient(t, listBody), JSON: true}

		require.NoError(t, runList(t.Context(), opts))

		assert.Contains(t, stdout.String(), `"id": "b/two"`)
		assert.NotContains(t, stdout.String(), "NAME")
	})
}
