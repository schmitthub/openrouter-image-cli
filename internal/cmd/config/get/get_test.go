package get

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

func TestGetPrintsValue(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	t.Setenv("ORIMAGE_MODEL", "env/model")
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	cmd := NewCmdGet(f, nil)
	cmd.SetArgs([]string{"model"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()

	require.NoError(t, err)
	assert.Equal(t, "env/model\n", stdout.String())
}

func TestGetUnknownKey(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	cmd := NewCmdGet(f, nil)
	cmd.SetArgs([]string{"bogus"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown config key "bogus"`)
	var flagErr *cmdutil.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Empty(t, stdout.String())
}

func TestGetArgValidation(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	var captured *GetOptions
	cmd := NewCmdGet(f, func(o *GetOptions) error { captured = o; return nil })
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()

	require.Error(t, err)
	var flagErr *cmdutil.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Nil(t, captured)
	assert.Empty(t, stdout.String())
}
