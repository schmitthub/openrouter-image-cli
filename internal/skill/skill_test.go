package skill

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiles(t *testing.T) {
	files, err := Files()
	require.NoError(t, err)

	data, err := fs.ReadFile(files, "orgen/SKILL.md")
	require.NoError(t, err)

	content := string(data)
	assert.True(t, strings.HasPrefix(content, "---\n"), "SKILL.md must start with YAML frontmatter")
	assert.Contains(t, content, "name: orgen")
	assert.Contains(t, content, "description:")
}

func TestInstall(t *testing.T) {
	t.Run("writes skill files into the directory", func(t *testing.T) {
		dir := t.TempDir()

		written, err := Install(dir, false)
		require.NoError(t, err)
		require.NotEmpty(t, written)

		want := filepath.Join(dir, "orgen", "SKILL.md")
		assert.Contains(t, written, want)

		onDisk, err := os.ReadFile(want)
		require.NoError(t, err)
		files, err := Files()
		require.NoError(t, err)
		embedded, err := fs.ReadFile(files, "orgen/SKILL.md")
		require.NoError(t, err)
		assert.Equal(t, embedded, onDisk)
	})

	t.Run("creates missing directories", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "skills")

		_, err := Install(dir, false)
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(dir, "orgen", "SKILL.md"))
	})

	t.Run("fails when the skill directory already exists", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "orgen")
		userFile := filepath.Join(skillDir, "user-notes.md")
		require.NoError(t, os.MkdirAll(skillDir, 0o750))
		require.NoError(t, os.WriteFile(userFile, []byte("keep me"), 0o600))

		_, err := Install(dir, false)
		require.ErrorIs(t, err, ErrExists)
		assert.Contains(t, err.Error(), skillDir)

		onDisk, err := os.ReadFile(userFile)
		require.NoError(t, err)
		assert.Equal(t, "keep me", string(onDisk), "existing skill dir must be untouched")
	})

	t.Run("force rewrites the skill directory from scratch", func(t *testing.T) {
		dir := t.TempDir()
		stale := filepath.Join(dir, "orgen", "stale-layout.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o750))
		require.NoError(t, os.WriteFile(stale, []byte("old layout"), 0o600))

		_, err := Install(dir, true)
		require.NoError(t, err)

		assert.NoFileExists(t, stale, "stale files from an old layout must be removed")
		assert.FileExists(t, filepath.Join(dir, "orgen", "SKILL.md"))
	})

	t.Run("force replaces a file occupying the skill directory path", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "orgen"), []byte("a binary"), 0o600))

		_, err := Install(dir, false)
		require.ErrorIs(t, err, ErrExists)

		_, err = Install(dir, true)
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(dir, "orgen", "SKILL.md"))
	})

	t.Run("force on a symlinked skill directory removes only the link", func(t *testing.T) {
		dir := t.TempDir()
		realDir := filepath.Join(dir, "real-skill")
		precious := filepath.Join(realDir, "precious.md")
		require.NoError(t, os.MkdirAll(realDir, 0o750))
		require.NoError(t, os.WriteFile(precious, []byte("keep me"), 0o600))
		require.NoError(t, os.Symlink(realDir, filepath.Join(dir, "orgen")))

		_, err := Install(dir, true)
		require.NoError(t, err)

		onDisk, err := os.ReadFile(precious)
		require.NoError(t, err)
		assert.Equal(t, "keep me", string(onDisk), "symlink target must survive")
		assert.FileExists(t, filepath.Join(dir, "orgen", "SKILL.md"))
	})

	t.Run("fails when dir is an existing file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "document.pdf")
		require.NoError(t, os.WriteFile(file, []byte("data"), 0o600))

		_, err := Install(file, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("preserves entries in dir other than the skill directory", func(t *testing.T) {
		dir := t.TempDir()
		foreignFile := filepath.Join(dir, "notes.txt")
		foreignDir := filepath.Join(dir, "other-skill")
		require.NoError(t, os.WriteFile(foreignFile, []byte("keep me"), 0o600))
		require.NoError(t, os.MkdirAll(foreignDir, 0o750))

		_, err := Install(dir, true)
		require.NoError(t, err)

		onDisk, err := os.ReadFile(foreignFile)
		require.NoError(t, err)
		assert.Equal(t, "keep me", string(onDisk))
		assert.DirExists(t, foreignDir)
	})
}
