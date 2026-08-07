package generate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"
	"github.com/schmitthub/openrouter-generate/internal/config"
	"github.com/schmitthub/openrouter-generate/internal/iostreams"
	"github.com/schmitthub/openrouter-generate/internal/openrouter"
)

func TestNewCmdGenerate(t *testing.T) {
	tests := []struct {
		name      string
		cli       string
		wantsOpts GenerateOptions
		wantsErr  bool
		errMsg    string
	}{
		{
			name: "model and prompt only",
			cli:  `-m google/gemini-2.5-flash-image -p "a red bicycle"`,
			wantsOpts: GenerateOptions{
				Model:  "google/gemini-2.5-flash-image",
				Prompt: "a red bicycle",
			},
		},
		{
			name: "all flags",
			cli: `--model m --prompt p --out img.png -n 3 --size 2K --quality high ` +
				`--output-format webp --output-compression 80 --aspect-ratio 16:9 ` +
				`--resolution 2K --background transparent --seed 42 ` +
				`--input-reference https://example.com/a.png ` +
				`--provider-sort price --provider-order fal,replicate --provider-only fal ` +
				`--provider-ignore openai --no-provider-fallbacks ` +
				`--provider-option black-forest-labs.steps=40 ` +
				`--provider-option black-forest-labs.guidance=3.5 ` +
				`--provider-option google-vertex.cachedContent=projects/x`,
			wantsOpts: GenerateOptions{
				Model: "m", Prompt: "p", Out: "img.png", N: 3, Size: "2K",
				Quality: "high", OutputFormat: "webp",
				OutputCompression: 80, CompressionSet: true,
				AspectRatio: "16:9", Resolution: "2K", Background: "transparent",
				Seed: 42, SeedSet: true,
				InputReferences:     []string{"https://example.com/a.png"},
				ProviderSort:        "price",
				ProviderOrder:       []string{"fal", "replicate"},
				ProviderOnly:        []string{"fal"},
				ProviderIgnore:      []string{"openai"},
				NoProviderFallbacks: true, FallbacksSet: true,
				ProviderOptions: []string{
					"black-forest-labs.steps=40",
					"black-forest-labs.guidance=3.5",
					"google-vertex.cachedContent=projects/x",
				},
				ProviderOptionValues: map[string]map[string]any{
					"black-forest-labs": {"steps": int64(40), "guidance": 3.5},
					"google-vertex":     {"cachedContent": "projects/x"},
				},
			},
		},
		{
			name:     "missing required model",
			cli:      `-p prompt`,
			wantsErr: true,
			errMsg:   `required flag(s) "model" not set`,
		},
		{
			name:     "missing required prompt",
			cli:      `-m model`,
			wantsErr: true,
			errMsg:   `required flag(s) "prompt" not set`,
		},
		{
			name:     "invalid quality",
			cli:      `-m m -p p --quality bogus`,
			wantsErr: true,
			errMsg:   "invalid quality: bogus",
		},
		{
			name:     "invalid output format",
			cli:      `-m m -p p --output-format tiff`,
			wantsErr: true,
			errMsg:   "invalid output-format: tiff",
		},
		{
			name:     "invalid resolution",
			cli:      `-m m -p p --resolution 8K`,
			wantsErr: true,
			errMsg:   "invalid resolution: 8K",
		},
		{
			name:     "invalid background",
			cli:      `-m m -p p --background blurry`,
			wantsErr: true,
			errMsg:   "invalid background: blurry",
		},
		{
			name:     "count out of range",
			cli:      `-m m -p p -n 11`,
			wantsErr: true,
			errMsg:   "invalid count: 11",
		},
		{
			name:     "compression out of range",
			cli:      `-m m -p p --output-compression 101`,
			wantsErr: true,
			errMsg:   "invalid output-compression: 101",
		},
		{
			name:     "invalid provider sort",
			cli:      `-m m -p p --provider-sort cheapest`,
			wantsErr: true,
			errMsg:   "invalid provider-sort: cheapest",
		},
		{
			name:     "provider option missing value",
			cli:      `-m m -p p --provider-option black-forest-labs.steps`,
			wantsErr: true,
			errMsg:   `invalid provider-option "black-forest-labs.steps"`,
		},
		{
			name:     "provider option missing key",
			cli:      `-m m -p p --provider-option black-forest-labs=40`,
			wantsErr: true,
			errMsg:   `invalid provider-option "black-forest-labs=40"`,
		},
		{
			name:     "empty provider order slug",
			cli:      `-m m -p p --provider-order a,,b`,
			wantsErr: true,
			errMsg:   "provider-order slugs must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{IOStreams: ios}
			argv, err := shlex.Split(tt.cli)
			require.NoError(t, err)

			var opts *GenerateOptions
			cmd := NewCmdGenerate(f, func(o *GenerateOptions) error {
				opts = o
				return nil
			})
			cmd.SetArgs(argv)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			_, err = cmd.ExecuteC()
			if tt.wantsErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, opts)
			assert.Equal(t, tt.wantsOpts.Model, opts.Model)
			assert.Equal(t, tt.wantsOpts.Prompt, opts.Prompt)
			assert.Equal(t, tt.wantsOpts.Out, opts.Out)
			assert.Equal(t, tt.wantsOpts.N, opts.N)
			assert.Equal(t, tt.wantsOpts.Size, opts.Size)
			assert.Equal(t, tt.wantsOpts.Quality, opts.Quality)
			assert.Equal(t, tt.wantsOpts.OutputFormat, opts.OutputFormat)
			assert.Equal(t, tt.wantsOpts.OutputCompression, opts.OutputCompression)
			assert.Equal(t, tt.wantsOpts.CompressionSet, opts.CompressionSet)
			assert.Equal(t, tt.wantsOpts.AspectRatio, opts.AspectRatio)
			assert.Equal(t, tt.wantsOpts.Resolution, opts.Resolution)
			assert.Equal(t, tt.wantsOpts.Background, opts.Background)
			assert.Equal(t, tt.wantsOpts.Seed, opts.Seed)
			assert.Equal(t, tt.wantsOpts.SeedSet, opts.SeedSet)
			assert.Equal(t, tt.wantsOpts.InputReferences, opts.InputReferences)
			assert.Equal(t, tt.wantsOpts.ProviderSort, opts.ProviderSort)
			assert.Equal(t, tt.wantsOpts.ProviderOrder, opts.ProviderOrder)
			assert.Equal(t, tt.wantsOpts.ProviderOnly, opts.ProviderOnly)
			assert.Equal(t, tt.wantsOpts.ProviderIgnore, opts.ProviderIgnore)
			assert.Equal(t, tt.wantsOpts.NoProviderFallbacks, opts.NoProviderFallbacks)
			assert.Equal(t, tt.wantsOpts.FallbacksSet, opts.FallbacksSet)
			assert.Equal(t, tt.wantsOpts.ProviderOptions, opts.ProviderOptions)
			assert.Equal(t, tt.wantsOpts.ProviderOptionValues, opts.ProviderOptionValues)
		})
	}
}

// fakeServer starts an httptest server serving n images and returns a client
// factory func pointing at it.
func fakeServer(t *testing.T, n int) func() (*openrouter.Client, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := make([]map[string]string, n)
		for i := range data {
			data[i] = map[string]string{
				"b64_json":   base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "img-%d", i+1)),
				"media_type": "image/png",
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1748372400,
			"data":    data,
			"usage":   map[string]any{"total_tokens": 100, "cost": 0.01},
		})
	}))
	t.Cleanup(srv.Close)
	client := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	return func() (*openrouter.Client, error) { return client, nil }
}

func Test_runGenerate(t *testing.T) {
	t.Run("single image to explicit path", func(t *testing.T) {
		clientF := fakeServer(t, 1)
		dir := t.TempDir()
		out := filepath.Join(dir, "sub", "img.png")

		ios, _, stdout, _ := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: clientF,
			Model:      "m",
			Prompt:     "p",
			Out:        out,
		}
		require.NoError(t, runGenerate(t.Context(), opts))

		content, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, "img-1", string(content))
		assert.Contains(t, stdout.String(), out)
	})

	t.Run("multiple images get indexed paths", func(t *testing.T) {
		clientF := fakeServer(t, 3)
		dir := t.TempDir()
		out := filepath.Join(dir, "img.png")

		ios, _, _, _ := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: clientF,
			Model:      "m",
			Prompt:     "p",
			N:          3,
			Out:        out,
		}
		require.NoError(t, runGenerate(t.Context(), opts))

		for i := 1; i <= 3; i++ {
			content, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("img-%d.png", i)))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("img-%d", i), string(content))
		}
	})

	t.Run("local input reference sent as data URI", func(t *testing.T) {
		dir := t.TempDir()
		refPath := filepath.Join(dir, "ref.png")
		require.NoError(t, os.WriteFile(refPath, pngFixture(), 0o644))

		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1,
				"data": []map[string]string{{
					"b64_json":   base64.StdEncoding.EncodeToString([]byte("img")),
					"media_type": "image/png",
				}},
			})
		}))
		t.Cleanup(srv.Close)
		client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))

		ios, _, _, _ := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: func() (*openrouter.Client, error) { return client, nil },
			Model:      "m",
			Prompt:     "p",
			Out:        filepath.Join(dir, "out.png"),
			InputReferences: []string{
				refPath,
				"https://example.com/style.png",
			},
		}
		require.NoError(t, runGenerate(t.Context(), opts))

		// The API rejects bare strings — every reference must be the
		// {"type":"image_url","image_url":{"url":...}} object form.
		var req struct {
			InputReferences []map[string]any `json:"input_references"`
		}
		require.NoError(t, json.Unmarshal(gotBody, &req))
		wantDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngFixture())
		assert.Equal(t, []map[string]any{
			{"type": "image_url", "image_url": map[string]any{"url": wantDataURI}},
			{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/style.png"}},
		}, req.InputReferences)
	})

	t.Run("provider preferences sent on the wire", func(t *testing.T) {
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				// endpoints lookup from the passthrough warning check
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "black-forest-labs/flux.2-pro",
					"endpoints": []map[string]any{{
						"provider_slug":                  "black-forest-labs",
						"allowed_passthrough_parameters": []string{"steps", "guidance"},
					}},
				})
				return
			}
			gotBody, _ = io.ReadAll(r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1,
				"data": []map[string]string{{
					"b64_json":   base64.StdEncoding.EncodeToString([]byte("img")),
					"media_type": "image/png",
				}},
			})
		}))
		t.Cleanup(srv.Close)
		client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))

		ios, _, _, stderr := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:           ios,
			OpenRouter:          func() (*openrouter.Client, error) { return client, nil },
			Model:               "black-forest-labs/flux.2-pro",
			Prompt:              "p",
			Out:                 filepath.Join(t.TempDir(), "out.png"),
			ProviderSort:        "price",
			ProviderOrder:       []string{"black-forest-labs"},
			ProviderIgnore:      []string{"openai"},
			NoProviderFallbacks: true, FallbacksSet: true,
			ProviderOptionValues: map[string]map[string]any{
				"black-forest-labs": {"steps": int64(40), "guidance": 3.5},
			},
		}
		require.NoError(t, runGenerate(t.Context(), opts))

		var req map[string]any
		require.NoError(t, json.Unmarshal(gotBody, &req))
		assert.Equal(t, map[string]any{
			"sort":            "price",
			"order":           []any{"black-forest-labs"},
			"ignore":          []any{"openai"},
			"allow_fallbacks": false,
			"options": map[string]any{
				"black-forest-labs": map[string]any{"steps": float64(40), "guidance": 3.5},
			},
		}, req["provider"])
		assert.Empty(t, stderr.String(), "advertised keys must not warn")
	})

	t.Run("provider omitted when no routing set", func(t *testing.T) {
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("X-Provider-Name", "Test Provider")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1,
				"data": []map[string]string{{
					"b64_json":   base64.StdEncoding.EncodeToString([]byte("img")),
					"media_type": "image/png",
				}},
			})
		}))
		t.Cleanup(srv.Close)
		client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))

		ios, _, _, _ := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: func() (*openrouter.Client, error) { return client, nil },
			Model:      "m",
			Prompt:     "p",
			Out:        filepath.Join(t.TempDir(), "out.png"),
		}
		require.NoError(t, runGenerate(t.Context(), opts))

		var req map[string]any
		require.NoError(t, json.Unmarshal(gotBody, &req))
		assert.NotContains(t, req, "provider")
	})

	t.Run("warns on unadvertised passthrough options", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "m",
					"endpoints": []map[string]any{{
						"provider_slug":                  "black-forest-labs",
						"allowed_passthrough_parameters": []string{"steps"},
					}},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1,
				"data": []map[string]string{{
					"b64_json":   base64.StdEncoding.EncodeToString([]byte("img")),
					"media_type": "image/png",
				}},
			})
		}))
		t.Cleanup(srv.Close)
		client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))

		ios, _, _, stderr := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: func() (*openrouter.Client, error) { return client, nil },
			Model:      "m",
			Prompt:     "p",
			Out:        filepath.Join(t.TempDir(), "out.png"),
			ProviderOptionValues: map[string]map[string]any{
				"black-forest-labs": {"stepz": int64(1)},
				"no-such-provider":  {"x": int64(2)},
			},
		}
		require.NoError(t, runGenerate(t.Context(), opts))

		out := stderr.String()
		assert.Contains(t, out, `"stepz" is not an advertised passthrough key for black-forest-labs`)
		assert.Contains(t, out, `has no "no-such-provider" endpoint`)
	})

	t.Run("api error surfaces", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"error": {"code": 402, "message": "Insufficient credits"}}`))
		}))
		t.Cleanup(srv.Close)
		client := openrouter.New("k", openrouter.WithBaseURL(srv.URL))

		ios, _, _, _ := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: func() (*openrouter.Client, error) { return client, nil },
			Model:      "m",
			Prompt:     "p",
		}
		err := runGenerate(t.Context(), opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Insufficient credits")
	})

	t.Run("client construction error surfaces", func(t *testing.T) {
		ios, _, _, _ := iostreams.Test()
		opts := &GenerateOptions{
			IOStreams:  ios,
			OpenRouter: func() (*openrouter.Client, error) { return nil, errors.New("no key") },
			Model:      "m",
			Prompt:     "p",
		}
		err := runGenerate(t.Context(), opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no key")
	})
}

func Test_outPath(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		created int64
		ext     string
		i, n    int
		want    string
	}{
		{name: "empty out single", out: "", created: 99, ext: "png", i: 0, n: 1, want: "orgen-99.png"},
		{name: "empty out multi", out: "", created: 99, ext: "webp", i: 1, n: 3, want: "orgen-99-2.webp"},
		{name: "explicit single", out: "a/b.png", created: 99, ext: "png", i: 0, n: 1, want: "a/b.png"},
		{name: "explicit multi", out: "b.png", created: 99, ext: "png", i: 2, n: 3, want: "b-3.png"},
		{name: "explicit multi no ext", out: "b", created: 99, ext: "jpg", i: 0, n: 2, want: "b-1.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, outPath(tt.out, tt.created, tt.ext, tt.i, tt.n))
		})
	}
}

// stubConfig is a canned config.Config for fallback tests.
type stubConfig struct {
	model, aspect, format string
	compression           int
	compressionSet        bool
	providerSort          string
	providerOrder         []string
	providerOnly          []string
	providerIgnore        []string
	allowFallbacks        bool
	allowFallbacksSet     bool
	providerOptions       map[string]map[string]any
}

func (s stubConfig) Model() string                  { return s.model }
func (s stubConfig) AspectRatio() string            { return s.aspect }
func (s stubConfig) OutputFormat() string           { return s.format }
func (s stubConfig) OutputCompression() (int, bool) { return s.compression, s.compressionSet }
func (s stubConfig) Provider() map[string]any       { return nil }
func (s stubConfig) ProviderSort() string           { return s.providerSort }
func (s stubConfig) ProviderOrder() []string        { return s.providerOrder }
func (s stubConfig) ProviderOnly() []string         { return s.providerOnly }
func (s stubConfig) ProviderIgnore() []string       { return s.providerIgnore }
func (s stubConfig) ProviderAllowFallbacks() (bool, bool) {
	return s.allowFallbacks, s.allowFallbacksSet
}
func (s stubConfig) ProviderOptions() map[string]map[string]any { return s.providerOptions }
func (s stubConfig) AllKeys() []string                          { return nil }
func (s stubConfig) Get(string) (string, error)                 { return "", nil }
func (s stubConfig) Set(string, string) error                   { return nil }
func (s stubConfig) Save() error                                { return nil }
func (s stubConfig) Path() string                               { return "" }

func TestGenerateConfigDefaults(t *testing.T) {
	cfg := stubConfig{
		model: "cfg/model", aspect: "16:9", format: "webp",
		compression: 42, compressionSet: true,
		providerSort:   "price",
		providerOrder:  []string{"fal"},
		providerOnly:   []string{"fal", "replicate"},
		providerIgnore: []string{"openai"},
		allowFallbacks: false, allowFallbacksSet: true,
		providerOptions: map[string]map[string]any{
			"black-forest-labs": {"steps": int64(28)},
		},
	}
	tests := []struct {
		name      string
		cli       string
		config    config.Config
		configErr error
		wantsOpts GenerateOptions
		errMsg    string
	}{
		{
			name:   "config fills unset flags",
			cli:    `-p prompt`,
			config: cfg,
			wantsOpts: GenerateOptions{
				Model: "cfg/model", Prompt: "prompt", AspectRatio: "16:9",
				OutputFormat: "webp", OutputCompression: 42, CompressionSet: true,
				ProviderSort:  "price",
				ProviderOrder: []string{"fal"}, ProviderOnly: []string{"fal", "replicate"},
				ProviderIgnore:      []string{"openai"},
				NoProviderFallbacks: true, FallbacksSet: true,
				ProviderOptionValues: map[string]map[string]any{
					"black-forest-labs": {"steps": int64(28)},
				},
			},
		},
		{
			name: "flags beat config",
			cli: `-m flag/model -p prompt --aspect-ratio 1:1 --output-format png --output-compression 90 ` +
				`--provider-sort latency --provider-order direct --no-provider-fallbacks=false ` +
				`--provider-option black-forest-labs.steps=40`,
			config: cfg,
			wantsOpts: GenerateOptions{
				Model: "flag/model", Prompt: "prompt", AspectRatio: "1:1",
				OutputFormat: "png", OutputCompression: 90, CompressionSet: true,
				ProviderSort:  "latency",
				ProviderOrder: []string{"direct"}, ProviderOnly: []string{"fal", "replicate"},
				ProviderIgnore:      []string{"openai"},
				NoProviderFallbacks: false, FallbacksSet: true,
				ProviderOptions: []string{"black-forest-labs.steps=40"},
				ProviderOptionValues: map[string]map[string]any{
					"black-forest-labs": {"steps": int64(40)},
				},
			},
		},
		{
			name:   "empty config still requires model",
			cli:    `-p prompt`,
			config: stubConfig{},
			errMsg: `required flag(s) "model" not set`,
		},
		{
			name:      "config load error surfaces",
			cli:       `-p prompt`,
			configErr: errors.New("boom"),
			errMsg:    "loading config: boom",
		},
		{
			name:   "invalid configured format rejected",
			cli:    `-p prompt`,
			config: stubConfig{model: "m", format: "tiff"},
			errMsg: "invalid output-format: tiff",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: ios,
				Config: func() (config.Config, error) {
					if tt.configErr != nil {
						return nil, tt.configErr
					}
					return tt.config, nil
				},
			}
			argv, err := shlex.Split(tt.cli)
			require.NoError(t, err)

			var opts *GenerateOptions
			cmd := NewCmdGenerate(f, func(o *GenerateOptions) error {
				opts = o
				return nil
			})
			cmd.SetArgs(argv)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			_, err = cmd.ExecuteC()
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantsOpts.Model, opts.Model)
			assert.Equal(t, tt.wantsOpts.AspectRatio, opts.AspectRatio)
			assert.Equal(t, tt.wantsOpts.OutputFormat, opts.OutputFormat)
			assert.Equal(t, tt.wantsOpts.OutputCompression, opts.OutputCompression)
			assert.Equal(t, tt.wantsOpts.CompressionSet, opts.CompressionSet)
			assert.Equal(t, tt.wantsOpts.ProviderSort, opts.ProviderSort)
			assert.Equal(t, tt.wantsOpts.ProviderOrder, opts.ProviderOrder)
			assert.Equal(t, tt.wantsOpts.ProviderOnly, opts.ProviderOnly)
			assert.Equal(t, tt.wantsOpts.ProviderIgnore, opts.ProviderIgnore)
			assert.Equal(t, tt.wantsOpts.NoProviderFallbacks, opts.NoProviderFallbacks)
			assert.Equal(t, tt.wantsOpts.FallbacksSet, opts.FallbacksSet)
			assert.Equal(t, tt.wantsOpts.ProviderOptionValues, opts.ProviderOptionValues)
		})
	}
}
