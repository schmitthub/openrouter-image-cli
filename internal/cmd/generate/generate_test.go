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

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
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
				`--input-reference https://example.com/a.png`,
			wantsOpts: GenerateOptions{
				Model: "m", Prompt: "p", Out: "img.png", N: 3, Size: "2K",
				Quality: "high", OutputFormat: "webp",
				OutputCompression: 80, CompressionSet: true,
				AspectRatio: "16:9", Resolution: "2K", Background: "transparent",
				Seed: 42, SeedSet: true,
				InputReferences: []string{"https://example.com/a.png"},
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

		var req struct {
			InputReferences []string `json:"input_references"`
		}
		require.NoError(t, json.Unmarshal(gotBody, &req))
		want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngFixture())
		assert.Equal(t, []string{want, "https://example.com/style.png"}, req.InputReferences)
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
		{name: "empty out single", out: "", created: 99, ext: "png", i: 0, n: 1, want: "orimage-99.png"},
		{name: "empty out multi", out: "", created: 99, ext: "webp", i: 1, n: 3, want: "orimage-99-2.webp"},
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
