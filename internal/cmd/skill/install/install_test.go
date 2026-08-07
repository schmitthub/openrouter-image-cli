package install

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"
	"github.com/schmitthub/openrouter-generate/internal/iostreams"
)

func TestNewCmdInstall(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantsDir   string
		wantsForce bool
		wantsErr   bool
		errMsg     string
	}{
		{
			name:     "directory argument",
			args:     []string{"/tmp/skills"},
			wantsDir: "/tmp/skills",
		},
		{
			name:       "force flag",
			args:       []string{"--force", "/tmp/skills"},
			wantsDir:   "/tmp/skills",
			wantsForce: true,
		},
		{
			name:     "missing argument",
			args:     []string{},
			wantsErr: true,
			errMsg:   "accepts 1 arg(s), received 0",
		},
		{
			name:     "too many arguments",
			args:     []string{"a", "b"},
			wantsErr: true,
			errMsg:   "accepts 1 arg(s), received 2",
		},
		{
			name:     "empty directory",
			args:     []string{""},
			wantsErr: true,
			errMsg:   "directory must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{IOStreams: ios}

			var opts *InstallOptions
			cmd := NewCmdInstall(f, func(o *InstallOptions) error {
				opts = o
				return nil
			})
			cmd.SetArgs(tt.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			_, err := cmd.ExecuteC()
			if tt.wantsErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, opts)
			assert.Equal(t, tt.wantsDir, opts.Dir)
			assert.Equal(t, tt.wantsForce, opts.Force)
		})
	}
}

func Test_runInstall(t *testing.T) {
	t.Run("installs into an empty directory", func(t *testing.T) {
		ios, _, stdout, _ := iostreams.Test()
		dir := t.TempDir()
		opts := &InstallOptions{IOStreams: ios, Dir: dir}

		require.NoError(t, runInstall(opts))

		want := filepath.Join(dir, "orgen", "SKILL.md")
		assert.FileExists(t, want)
		assert.Contains(t, stdout.String(), want)
	})

	t.Run("existing install fails with a force hint", func(t *testing.T) {
		ios, _, _, _ := iostreams.Test()
		dir := t.TempDir()
		require.NoError(t, runInstall(&InstallOptions{IOStreams: ios, Dir: dir}))

		err := runInstall(&InstallOptions{IOStreams: ios, Dir: dir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "use --force")
	})

	t.Run("force replaces an existing install", func(t *testing.T) {
		ios, _, _, _ := iostreams.Test()
		dir := t.TempDir()
		require.NoError(t, runInstall(&InstallOptions{IOStreams: ios, Dir: dir}))

		opts := &InstallOptions{IOStreams: ios, Dir: dir, Force: true}
		require.NoError(t, runInstall(opts))
	})
}
