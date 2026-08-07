package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/config"
)

// newInDir points the config package at a temp dir and loads it.
//
//nolint:ireturn // exercises the interface-returning constructor
func newInDir(t *testing.T) (config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	cfg, err := config.New()
	require.NoError(t, err)
	return cfg, dir
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600))
}

func TestNewMissingFile(t *testing.T) {
	cfg, dir := newInDir(t)

	assert.Empty(t, cfg.Model())
	assert.Empty(t, cfg.AspectRatio())
	assert.Empty(t, cfg.OutputFormat())
	_, ok := cfg.OutputCompression()
	assert.False(t, ok)
	assert.Empty(t, cfg.Provider())
	assert.Equal(t, filepath.Join(dir, "config.yaml"), cfg.Path())
}

func TestAllKeys(t *testing.T) {
	cfg, dir := newInDir(t)
	assert.Equal(t,
		[]string{"aspect_ratio", "model", "output_compression", "output_format"},
		cfg.AllKeys(), "empty config lists the bound scalar keys, sorted")

	writeConfig(t, dir, "provider:\n  sort: price\nmodel: m\n")
	reloaded, err := config.New()
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"aspect_ratio", "model", "output_compression", "output_format", "provider.sort"},
		reloaded.AllKeys(), "file keys flatten into the list")
}

func TestLoadsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	writeConfig(t, dir, `
model: google/gemini-2.5-flash-image
aspect_ratio: "16:9"
output_format: webp
output_compression: 80
provider:
  sort: price
`)
	cfg, err := config.New()
	require.NoError(t, err)

	assert.Equal(t, "google/gemini-2.5-flash-image", cfg.Model())
	assert.Equal(t, "16:9", cfg.AspectRatio())
	assert.Equal(t, "webp", cfg.OutputFormat())
	n, ok := cfg.OutputCompression()
	assert.True(t, ok)
	assert.Equal(t, 80, n)
	assert.Equal(t, map[string]any{"sort": "price"}, cfg.Provider())
}

func TestMalformedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	writeConfig(t, dir, "model: [unclosed")

	_, err := config.New()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config")
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	writeConfig(t, dir, "model: from-file\noutput_compression: 10\n")
	t.Setenv("ORIMAGE_MODEL", "from-env")
	t.Setenv("ORIMAGE_OUTPUT_COMPRESSION", "90")
	t.Setenv("ORIMAGE_ASPECT_RATIO", "9:16")
	t.Setenv("ORIMAGE_PROVIDER_SORT", "throughput")

	cfg, err := config.New()
	require.NoError(t, err)

	assert.Equal(t, "from-env", cfg.Model())
	n, ok := cfg.OutputCompression()
	assert.True(t, ok)
	assert.Equal(t, 90, n)
	assert.Equal(t, "9:16", cfg.AspectRatio())
	sort, err := cfg.Get("provider.sort")
	require.NoError(t, err)
	assert.Equal(t, "throughput", sort)
}

func TestEnvAloneSetsCompression(t *testing.T) {
	cfg, _ := newInDir(t)
	_, ok := cfg.OutputCompression()
	require.False(t, ok)

	t.Setenv("ORIMAGE_OUTPUT_COMPRESSION", "55")
	n, ok := cfg.OutputCompression()
	assert.True(t, ok)
	assert.Equal(t, 55, n)
}

func TestSetSaveRoundtrip(t *testing.T) {
	cfg, dir := newInDir(t)

	require.NoError(t, cfg.Set("model", "some/model"))
	require.NoError(t, cfg.Set("output_compression", "70"))
	require.NoError(t, cfg.Set("provider.sort", "price"))
	require.NoError(t, cfg.Save())

	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "config.yaml"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	reloaded, err := config.New()
	require.NoError(t, err)
	assert.Equal(t, "some/model", reloaded.Model())
	n, ok := reloaded.OutputCompression()
	assert.True(t, ok)
	assert.Equal(t, 70, n)
	assert.Equal(t, map[string]any{"sort": "price"}, reloaded.Provider())
}

func TestSavePreservesExistingEntries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	writeConfig(t, dir, "aspect_ratio: \"1:1\"\n")

	cfg, err := config.New()
	require.NoError(t, err)
	require.NoError(t, cfg.Set("model", "some/model"))
	require.NoError(t, cfg.Save())

	reloaded, err := config.New()
	require.NoError(t, err)
	assert.Equal(t, "1:1", reloaded.AspectRatio())
	assert.Equal(t, "some/model", reloaded.Model())
}

func TestSaveNeverPersistsEnv(t *testing.T) {
	cfg, dir := newInDir(t)
	t.Setenv("ORIMAGE_MODEL", "from-env")

	require.NoError(t, cfg.Set("aspect_ratio", "16:9"))
	require.NoError(t, cfg.Save())

	raw, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "from-env")
	assert.Contains(t, string(raw), "aspect_ratio")
}

func TestSetRejectsUnknownKey(t *testing.T) {
	cfg, _ := newInDir(t)
	err := cfg.Set("bogus", "v")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown config key "bogus"`)
}

func TestGetAllowsFileKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	writeConfig(t, dir, "custom_key: hello\n")

	cfg, err := config.New()
	require.NoError(t, err)
	v, err := cfg.Get("custom_key")
	require.NoError(t, err)
	assert.Equal(t, "hello", v)
}

func TestSetRejectsNonIntCompression(t *testing.T) {
	cfg, _ := newInDir(t)
	err := cfg.Set("output_compression", "high")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an integer")
}

func TestBareProviderKeyRejected(t *testing.T) {
	cfg, _ := newInDir(t)

	_, err := cfg.Get("provider")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider.<setting>")

	err = cfg.Set("provider", "x")
	require.Error(t, err)
}

func TestPathHonorsDirOverride(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	cfg, err := config.New()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(os.Getenv(config.EnvConfigDir), "config.yaml"), cfg.Path())
}

func TestPathDefaultsToUserConfigDir(t *testing.T) {
	t.Setenv(config.EnvConfigDir, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // keep the real user config out of the test
	cfg, err := config.New()
	require.NoError(t, err)
	assert.Equal(t, "orimage", filepath.Base(filepath.Dir(cfg.Path())))
	assert.Equal(t, "config.yaml", filepath.Base(cfg.Path()))
}
