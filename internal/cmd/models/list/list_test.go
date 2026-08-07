package list

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-generate/internal/iostreams"
	"github.com/schmitthub/openrouter-generate/internal/openrouter"
)

// fakeClient serves the model list plus per-model endpoints routes.
// endpointBodies maps model id -> endpoints response; missing ids 404 so
// tests can exercise pricing-lookup failures.
func fakeClient(t *testing.T, endpointBodies map[string]string) func() (*openrouter.Client, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/endpoints") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/images/models/"), "/endpoints")
			body, ok := endpointBodies[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error": {"code": 404, "message": "not found"}}`))
				return
			}
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(listBody))
	}))
	t.Cleanup(srv.Close)
	client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	return func() (*openrouter.Client, error) { return client, nil }
}

const listBody = `{"data": [
	{"id": "a/one", "name": "One", "architecture": {"input_modalities": ["text"]}, "supports_streaming": true},
	{"id": "b/two", "name": "Two", "architecture": {"input_modalities": ["text", "image"]}, "supports_streaming": false}
]}`

func endpointBodies() map[string]string {
	return map[string]string{
		"a/one": `{"id": "a/one", "endpoints": [
			{"provider_slug": "p1", "pricing": [{"billable": "output_image", "unit": "image", "cost_usd": 0.04}]},
			{"provider_slug": "p2", "pricing": [{"billable": "output_image", "unit": "megapixel", "cost_usd": 0.06}]}
		]}`,
		"b/two": `{"id": "b/two", "endpoints": [
			{"provider_slug": "p1", "pricing": [
				{"billable": "input_image", "unit": "token", "cost_usd": 0.000001},
				{"billable": "output_image", "unit": "token", "cost_usd": 0.00003}
			]}
		]}`,
	}
}

func Test_runList(t *testing.T) {
	t.Run("table output on tty includes costs", func(t *testing.T) {
		ios, _, stdout, stderr := iostreams.Test()
		ios.SetStdoutTTY(true)
		opts := &ListOptions{IOStreams: ios, OpenRouter: fakeClient(t, endpointBodies())}

		require.NoError(t, runList(t.Context(), opts))

		out := stdout.String()
		assert.Contains(t, out, "COST")
		assert.Contains(t, out, "$0.04+/img", "cheapest output price wins, + marks pricier tiers")
		assert.Contains(t, out, "$0.00003/tok")
		assert.Contains(t, out, "a/one")
		assert.Contains(t, out, "text,image")
		assert.Contains(t, out, "✓")
		assert.Contains(t, stderr.String(), "2 models")
		assert.NotContains(t, stderr.String(), "pricing unavailable")
	})

	t.Run("plain output when piped appends cost", func(t *testing.T) {
		ios, _, stdout, stderr := iostreams.Test()
		opts := &ListOptions{IOStreams: ios, OpenRouter: fakeClient(t, endpointBodies())}

		require.NoError(t, runList(t.Context(), opts))

		out := stdout.String()
		assert.NotContains(t, out, "ID\t")
		assert.Contains(t, out, "a/one\tOne\ttext\ttrue\t$0.04+/img\n")
		assert.Contains(t, out, "b/two\tTwo\ttext,image\tfalse\t$0.00003/tok\n")
		assert.Empty(t, stderr.String())
	})

	t.Run("json output includes pricing", func(t *testing.T) {
		ios, _, stdout, _ := iostreams.Test()
		opts := &ListOptions{IOStreams: ios, OpenRouter: fakeClient(t, endpointBodies()), JSON: true}

		require.NoError(t, runList(t.Context(), opts))

		out := stdout.String()
		assert.Contains(t, out, `"id":"b/two"`)
		assert.Contains(t, out, `"pricing":[`)
		assert.Contains(t, out, `"cost_usd":0.04`)
		assert.NotContains(t, out, "NAME")
	})

	t.Run("pricing lookup failure renders dash and warns", func(t *testing.T) {
		ios, _, stdout, stderr := iostreams.Test()
		ios.SetStdoutTTY(true)
		// only a/one has an endpoints route; b/two's lookup 404s
		bodies := map[string]string{"a/one": endpointBodies()["a/one"]}
		opts := &ListOptions{IOStreams: ios, OpenRouter: fakeClient(t, bodies)}

		require.NoError(t, runList(t.Context(), opts))

		var twoRow string
		for l := range strings.SplitSeq(stdout.String(), "\n") {
			if strings.Contains(l, "b/two") {
				twoRow = l
			}
		}
		assert.Contains(t, twoRow, "-")
		assert.Contains(t, stderr.String(), "pricing unavailable for 1 models")
	})
}
