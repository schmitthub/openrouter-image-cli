package config

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	appconfig "github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

// testFactory wires a real config store rooted in a temp dir and
// returns the factory, the stdout buffer, and the config dir.
func testFactory(t *testing.T) (*cmdutil.Factory, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(appconfig.EnvConfigDir, dir)
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: ios,
		Config:    appconfig.New,
	}
	return f, stdout, dir
}

func execute(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()
	return err
}

func TestConfigSetGetListRoundtrip(t *testing.T) {
	f, stdout, dir := testFactory(t)

	require.NoError(t, execute(t, NewCmdConfig(f), "set", "model", "some/model"))
	require.NoError(t, execute(t, NewCmdConfig(f), "set", "provider.sort", "price"))

	require.NoError(t, execute(t, NewCmdConfig(f), "get", "model"))
	assert.Equal(t, "some/model\n", stdout.String())
	stdout.Reset()

	require.NoError(t, execute(t, NewCmdConfig(f), "list"))
	listed := stdout.String()
	assert.Contains(t, listed, "model=some/model\n")
	assert.Contains(t, listed, "aspect_ratio=\n")
	assert.Contains(t, listed, "provider.sort=price\n")
	stdout.Reset()

	require.NoError(t, execute(t, NewCmdConfig(f), "path"))
	assert.Equal(t, filepath.Join(dir, "config.yaml")+"\n", stdout.String())
}

func TestConfigSetUnknownKey(t *testing.T) {
	f, _, _ := testFactory(t)
	err := execute(t, NewCmdConfig(f), "set", "bogus", "v")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown config key "bogus"`)
}

func TestConfigGetUnknownKey(t *testing.T) {
	f, _, _ := testFactory(t)
	err := execute(t, NewCmdConfig(f), "get", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown config key "bogus"`)
}

func TestConfigArgValidation(t *testing.T) {
	f, _, _ := testFactory(t)

	err := execute(t, NewCmdConfig(f), "get")
	require.Error(t, err)
	var flagErr *cmdutil.FlagError
	require.ErrorAs(t, err, &flagErr)

	err = execute(t, NewCmdConfig(f), "set", "model")
	require.Error(t, err)
	require.ErrorAs(t, err, &flagErr)
}

func TestConfigSetSilentWhenNotTTY(t *testing.T) {
	f, stdout, _ := testFactory(t)
	require.NoError(t, execute(t, NewCmdConfig(f), "set", "model", "m"))
	assert.Empty(t, stdout.String())
}
