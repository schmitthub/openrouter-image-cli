package list

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

func TestListShowsAllKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("model: some/model\nprovider:\n  sort: price\n"), 0o600))
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	cmd := NewCmdList(f, nil)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()

	require.NoError(t, err)
	assert.Equal(t,
		"aspect_ratio=\nmodel=some/model\noutput_compression=\noutput_format=\nprovider.sort=price\n",
		stdout.String())
}

func TestListEmptyConfig(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	cmd := NewCmdList(f, nil)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()

	require.NoError(t, err)
	assert.Equal(t, "aspect_ratio=\nmodel=\noutput_compression=\noutput_format=\n", stdout.String())
}
