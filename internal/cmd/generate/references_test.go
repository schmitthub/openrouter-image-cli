package generate

import (
	"bytes"
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pngFixture returns bytes carrying a real PNG signature so content
// sniffing resolves image/png.
func pngFixture() []byte {
	return []byte("\x89PNG\r\n\x1a\n0123456789")
}

func Test_resolveInputReferences(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "ref.png")
	require.NoError(t, os.WriteFile(pngPath, pngFixture(), 0o644))

	t.Run("local file becomes data URI", func(t *testing.T) {
		refs, err := resolveInputReferences([]string{pngPath})

		require.NoError(t, err)
		want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngFixture())
		assert.Equal(t, []string{want}, refs)
	})

	t.Run("urls and data URIs pass through", func(t *testing.T) {
		in := []string{
			"https://example.com/style.png",
			"http://example.com/a.jpg",
			"data:image/png;base64,AAAA",
		}

		refs, err := resolveInputReferences(in)

		require.NoError(t, err)
		assert.Equal(t, in, refs)
	})

	t.Run("raw base64 passes through", func(t *testing.T) {
		refs, err := resolveInputReferences([]string{"iVBORw0KGgoAAAANSUhEUg=="})

		require.NoError(t, err)
		assert.Equal(t, []string{"iVBORw0KGgoAAAANSUhEUg=="}, refs)
	})

	t.Run("directory errors", func(t *testing.T) {
		_, err := resolveInputReferences([]string{dir})

		assert.ErrorContains(t, err, "is a directory")
	})

	t.Run("symlink to file is followed", func(t *testing.T) {
		link := filepath.Join(dir, "link.png")
		require.NoError(t, os.Symlink(pngPath, link))

		refs, err := resolveInputReferences([]string{link})

		require.NoError(t, err)
		want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngFixture())
		assert.Equal(t, []string{want}, refs)
	})

	t.Run("missing path reports stat and decode errors", func(t *testing.T) {
		_, err := resolveInputReferences([]string{filepath.Join(dir, "nope.png")})

		require.ErrorIs(t, err, fs.ErrNotExist)
		require.ErrorContains(t, err, "no such file")
		require.ErrorContains(t, err, "not valid base64")
	})

	t.Run("broken symlink errors", func(t *testing.T) {
		link := filepath.Join(dir, "dangling")
		require.NoError(t, os.Symlink(filepath.Join(dir, "gone.png"), link))

		_, err := resolveInputReferences([]string{link})

		require.ErrorIs(t, err, fs.ErrNotExist)
		require.ErrorContains(t, err, "not valid base64")
	})

	t.Run("unreadable parent dir surfaces the stat error", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("permission checks are bypassed for root")
		}
		locked := filepath.Join(dir, "locked")
		require.NoError(t, os.Mkdir(locked, 0o755))
		// "logo" is deliberately valid base64: without the fs.ErrNotExist
		// classification this ref would silently pass through as raw base64.
		inside := filepath.Join(locked, "logo")
		require.NoError(t, os.WriteFile(inside, pngFixture(), 0o644))
		require.NoError(t, os.Chmod(locked, 0o000))
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

		_, err := resolveInputReferences([]string{inside})

		require.ErrorIs(t, err, fs.ErrPermission)
		assert.NotContains(t, err.Error(), "no such file")
	})

	t.Run("long raw base64 skips the filesystem probe", func(t *testing.T) {
		payload := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("x"), 4000))

		refs, err := resolveInputReferences([]string{payload})

		require.NoError(t, err)
		assert.Equal(t, []string{payload}, refs)
	})

	t.Run("empty is nil", func(t *testing.T) {
		refs, err := resolveInputReferences(nil)

		require.NoError(t, err)
		assert.Nil(t, refs)
	})
}

func Test_detectMediaType(t *testing.T) {
	assert.Equal(t, "image/png", detectMediaType("x.bin", pngFixture()))
	assert.Contains(t, detectMediaType("x.svg", []byte("<svg></svg>")), "image/svg")
}

func Test_validateOptions_inputReferences(t *testing.T) {
	t.Run("too many", func(t *testing.T) {
		refs := make([]string, 17)
		for i := range refs {
			refs[i] = "https://example.com/r.png"
		}
		opts := &GenerateOptions{Model: "m", Prompt: "p", InputReferences: refs}

		err := validateOptions(opts)

		assert.ErrorContains(t, err, "too many input-reference values: 17 (max 16)")
	})

	t.Run("empty value", func(t *testing.T) {
		opts := &GenerateOptions{Model: "m", Prompt: "p", InputReferences: []string{""}}

		err := validateOptions(opts)

		assert.ErrorContains(t, err, "input-reference values must not be empty")
	})
}
