package path

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"
	"github.com/schmitthub/openrouter-generate/internal/config"
	"github.com/schmitthub/openrouter-generate/internal/iostreams"
)

func TestPathPrintsLocation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	ios, _, stdout, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios, Config: config.New}

	cmd := NewCmdPath(f, nil)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, err := cmd.ExecuteC()

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "config.yaml")+"\n", stdout.String())
}
