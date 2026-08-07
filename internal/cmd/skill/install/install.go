package install

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/skill"
)

type InstallOptions struct {
	IOStreams *iostreams.IOStreams

	// Dir is the skills directory the embedded skill is written into,
	// as its own subdirectory.
	Dir string

	// Force removes an existing orimage/ skill directory in Dir and
	// rewrites it from scratch.
	Force bool
}

func NewCmdInstall(f *cmdutil.Factory, runF func(*InstallOptions) error) *cobra.Command {
	opts := &InstallOptions{
		IOStreams: f.IOStreams,
		Dir:       "",
		Force:     false,
	}

	cmd := &cobra.Command{
		Use:   "install <directory>",
		Short: "Write the orimage/ skill directory into a skills directory",
		Long: `Install the agent skill embedded in the orimage binary by writing its
skill directory, orimage/, into the given skills directory:

  <directory>/
  └── orimage/
      └── SKILL.md

<directory> is created if missing; if it exists it must be a directory,
and entries in it other than orimage/ are never touched. If
<directory>/orimage already exists the install fails; pass --force to
delete it and rewrite the current skill from scratch (its file layout
may change between orimage versions).`,
		Example: `  orimage skill install ~/.agents/skills
  orimage skill install --force .agents/skills`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Dir = args[0]
			if opts.Dir == "" {
				return cmdutil.FlagErrorf("directory must not be empty")
			}
			if runF != nil {
				return runF(opts)
			}
			return runInstall(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Force, "force", false, "Delete an existing orimage/ skill directory and rewrite it")

	return cmd
}

func runInstall(opts *InstallOptions) error {
	ios := opts.IOStreams
	var written []string
	err := ios.RunWithProgress("Installing skill", func() error {
		var installErr error
		written, installErr = skill.Install(opts.Dir, opts.Force)
		if installErr != nil {
			return fmt.Errorf("installing skill: %w", installErr)
		}
		return nil
	})
	if errors.Is(err, skill.ErrExists) {
		return fmt.Errorf("%w (use --force to reinstall)", err)
	}
	if err != nil {
		return err
	}

	if !ios.IsStdoutTTY() {
		// Piped: bare written paths, machine-friendly.
		for _, path := range written {
			fmt.Fprintln(ios.Out, path)
		}
		return nil
	}

	cs := ios.ColorScheme()
	for _, path := range written {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.SuccessIcon(), path)
	}
	fmt.Fprintln(ios.ErrOut, cs.Grayf("installed orimage skill into %s (%d file(s))", opts.Dir, len(written)))
	return nil
}
