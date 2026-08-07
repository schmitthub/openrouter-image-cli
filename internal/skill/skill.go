// Package skill embeds the agent skill shipped with orgen
// (https://docs.openclaw.ai/tools/skills) and installs it into a
// user-chosen skills directory.
package skill

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:assets
var assetsFS embed.FS

// Permissions satisfy gosec's G301/G306 ceilings; skills are per-user files.
const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// Files returns the embedded skill tree, rooted at the skill directory
// (e.g. "orgen/SKILL.md").
func Files() (fs.FS, error) {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("opening embedded skill: %w", err)
	}
	return sub, nil
}

// ErrExists reports that the skill's directory already exists in the
// target directory; Install returns it wrapped with the path.
var ErrExists = errors.New("skill already installed")

// Install writes the embedded skill into dir as the skill's own
// subdirectory (e.g. dir/orgen) and returns the written file paths in
// walk (lexical) order. dir may exist (it must then be a directory) or
// is created; entries in dir other than the skill's subdirectory are
// never touched. The skill's subdirectory is owned by the installer: if
// it already exists — in any form — Install fails with ErrExists, and
// with force set it is removed and rewritten from scratch instead, so no
// stale files survive a skill layout change between versions.
func Install(dir string, force bool) ([]string, error) {
	files, err := Files()
	if err != nil {
		return nil, err
	}
	paths, err := listFiles(files)
	if err != nil {
		return nil, err
	}
	if dirErr := ensureDir(dir); dirErr != nil {
		return nil, dirErr
	}
	if claimErr := claimSkillDirs(files, dir, force); claimErr != nil {
		return nil, claimErr
	}

	written := make([]string, 0, len(paths))
	for _, path := range paths {
		target, fileErr := installFile(files, dir, path)
		if fileErr != nil {
			return nil, fileErr
		}
		written = append(written, target)
	}
	return written, nil
}

// ensureDir verifies that dir, if it exists, is a directory. A missing
// dir is fine — Install creates it.
func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

// claimSkillDirs checks the skill's subdirectory under dir (one per
// top-level entry of the embedded tree). An existing entry — directory,
// file, or symlink — fails with ErrExists, or with force set is removed
// so the skill is rewritten from scratch. Removing a symlink removes
// only the link, never its target.
func claimSkillDirs(files fs.FS, dir string, force bool) error {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("reading embedded skill: %w", err)
	}
	for _, entry := range entries {
		target := filepath.Join(dir, entry.Name())
		_, statErr := os.Lstat(target)
		if errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return fmt.Errorf("checking %s: %w", target, statErr)
		}
		if !force {
			return fmt.Errorf("%w: %s", ErrExists, target)
		}
		if rmErr := os.RemoveAll(target); rmErr != nil {
			return fmt.Errorf("removing %s: %w", target, rmErr)
		}
	}
	return nil
}

// listFiles returns the slash-separated paths of every file in the
// embedded skill tree, in walk (lexical) order.
func listFiles(files fs.FS) ([]string, error) {
	var paths []string
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking embedded skill: %w", err)
	}
	return paths, nil
}

// installFile copies one embedded skill file to its target path under dir,
// creating parent directories as needed, and returns the target path.
func installFile(files fs.FS, dir, path string) (string, error) {
	data, readErr := fs.ReadFile(files, path)
	if readErr != nil {
		return "", fmt.Errorf("reading embedded %s: %w", path, readErr)
	}
	target := filepath.Join(dir, filepath.FromSlash(path))
	if mkErr := os.MkdirAll(filepath.Dir(target), dirPerm); mkErr != nil {
		return "", fmt.Errorf("creating %s: %w", filepath.Dir(target), mkErr)
	}
	if writeErr := os.WriteFile(target, data, filePerm); writeErr != nil {
		return "", fmt.Errorf("writing %s: %w", target, writeErr)
	}
	return target, nil
}
