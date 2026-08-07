package root_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/cmd/factory"
	"github.com/schmitthub/openrouter-image-cli/internal/cmd/root"
	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

// execWith runs args through the real command tree on an existing
// production factory, capturing stdout.
func execWith(t *testing.T, f *cmdutil.Factory, stdout *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	cmd, err := root.NewCmdRoot(f, "test", "")
	require.NoError(t, err)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err = cmd.ExecuteC()
	return stdout.String(), err
}

// execRoot runs args end to end — production factory (factory.New), real
// command tree, real filesystem. Each call builds a fresh factory, so
// state only survives through the config file, like separate CLI runs.
func execRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ios, _, stdout, _ := iostreams.Test()
	return execWith(t, factory.New("test", ios), stdout, args...)
}

func TestIntegrationConfigRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)

	_, err := execRoot(t, "config", "set", "model", "some/model")
	require.NoError(t, err)
	_, err = execRoot(t, "config", "set", "provider.sort", "price")
	require.NoError(t, err)

	out, err := execRoot(t, "config", "get", "model")
	require.NoError(t, err)
	assert.Equal(t, "some/model\n", out)

	out, err = execRoot(t, "config", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "model=some/model\n")
	assert.Contains(t, out, "provider.sort=price\n")

	out, err = execRoot(t, "config", "path")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "config.yaml")+"\n", out)
}

func TestIntegrationSetNeverPersistsEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("aspect_ratio: \"16:9\"\n"), 0o600))
	t.Setenv("ORIMAGE_MODEL", "env/override")

	_, err := execRoot(t, "config", "set", "output_format", "webp")
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "aspect_ratio")
	assert.Contains(t, string(raw), "output_format: webp")
	assert.NotContains(t, string(raw), "env/override",
		"an ORIMAGE_* override must never be written to disk by config set")

	out, err := execRoot(t, "config", "get", "model")
	require.NoError(t, err)
	assert.Equal(t, "env/override\n", out, "reads still see the env override")
}

func TestIntegrationGenerateUsesConfiguredModel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	t.Setenv("OPENROUTER_API_KEY", "")

	_, err := execRoot(t, "generate", "-p", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "model" not set`,
		"no flag, no config: model is required")

	_, err = execRoot(t, "config", "set", "model", "some/model")
	require.NoError(t, err)

	_, err = execRoot(t, "generate", "-p", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENROUTER_API_KEY",
		"configured model satisfies validation; failure moves past flags to the missing key")
}

func TestIntegrationFactoryMemoizesStore(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	ios, _, stdout, _ := iostreams.Test()
	f := factory.New("test", ios)

	_, err := execWith(t, f, stdout, "config", "set", "model", "memoized/model")
	require.NoError(t, err)
	stdout.Reset()

	out, err := execWith(t, f, stdout, "config", "get", "model")
	require.NoError(t, err)
	assert.Equal(t, "memoized/model\n", out,
		"one factory shares one store across commands")
}
