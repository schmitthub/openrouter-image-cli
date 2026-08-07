package set

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

func runSetCmd(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdSet(f, nil)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()
	return err
}

func TestSetPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	require.NoError(t, runSetCmd(t, f, "model", "some/model"))

	raw, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "model: some/model")
	assert.Empty(t, stdout.String(), "non-TTY set is silent")
}

func TestSetUnknownKey(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	err := runSetCmd(t, f, "bogus", "v")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown config key "bogus"`)
	assert.Empty(t, stdout.String())
}

func TestSetRejectsBadCompression(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	err := runSetCmd(t, f, "output_compression", "high")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an integer")
	assert.Empty(t, stdout.String())
}

func TestSetArgValidation(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	err := runSetCmd(t, f, "model")
	require.Error(t, err)
	var flagErr *cmdutil.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Empty(t, stdout.String())
}
