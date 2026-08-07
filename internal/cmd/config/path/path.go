// Package path implements "orimage config path".
package path

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

type PathOptions struct {
	IOStreams *iostreams.IOStreams
	Config    func() (config.Config, error)
}

func NewCmdPath(f *cmdutil.Factory, runF func(*PathOptions) error) *cobra.Command {
	opts := &PathOptions{
		IOStreams: f.IOStreams,
		Config:    f.Config,
	}

	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print the config file location",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return runPath(opts)
		},
	}

	return cmd
}

func runPath(opts *PathOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	fmt.Fprintln(opts.IOStreams.Out, cfg.Path())
	return nil
}
