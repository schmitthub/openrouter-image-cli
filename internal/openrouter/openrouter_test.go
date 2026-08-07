package openrouter_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

func TestGenerateImage(t *testing.T) {
	var gotPath, gotMethod, gotAuth string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = json.Marshal(decodeBody(r))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"created": 1748372400,
			"data": [{"b64_json": "aGVsbG8=", "media_type": "image/png"}],
			"usage": {"prompt_tokens": 0, "completion_tokens": 4175, "total_tokens": 4175, "cost": 0.04}
		}`))
	}))
	defer srv.Close()

	client := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	seed := int64(42)
	resp, err := client.GenerateImage(t.Context(), openrouter.ImageRequest{
		Model:  "bytedance-seed/seedream-4.5",
		Prompt: "a red panda astronaut",
		N:      2,
		Size:   "2K",
		Seed:   &seed,
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/images", gotPath)
	assert.Equal(t, "Bearer test-key", gotAuth)
	// zero-valued optionals must be omitted from the wire request
	assert.JSONEq(t, `{
		"model": "bytedance-seed/seedream-4.5",
		"prompt": "a red panda astronaut",
		"n": 2,
		"size": "2K",
		"seed": 42
	}`, string(gotBody))

	assert.Equal(t, int64(1748372400), resp.Created)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "aGVsbG8=", resp.Data[0].B64JSON)
	assert.Equal(t, "image/png", resp.Data[0].MediaType)
	assert.InDelta(t, 0.04, resp.Usage.Cost, 1e-9)
}

func TestGenerateImage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error": {"code": 402, "message": "Insufficient credits"}}`))
	}))
	defer srv.Close()

	client := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	_, err := client.GenerateImage(t.Context(), openrouter.ImageRequest{Model: "m", Prompt: "p"})

	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.StatusCode)
	assert.Equal(t, 402, apiErr.Code)
	assert.Equal(t, "Insufficient credits", apiErr.Message)
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv(openrouter.EnvAPIKey, "")
	_, err := openrouter.NewFromEnv()
	require.Error(t, err)
	assert.Contains(t, err.Error(), openrouter.EnvAPIKey)

	t.Setenv(openrouter.EnvAPIKey, "k")
	client, err := openrouter.NewFromEnv()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

// decodeBody parses the request body as generic JSON for assertion.
func decodeBody(r *http.Request) map[string]any {
	var m map[string]any
	_ = json.NewDecoder(r.Body).Decode(&m)
	return m
}
