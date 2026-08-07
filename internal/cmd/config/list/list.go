// Package list implements "orimage config list".
package list

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

type ListOptions struct {
	IOStreams *iostreams.IOStreams
	Config    func() (config.Config, error)
}

func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IOStreams: f.IOStreams,
		Config:    f.Config,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print all config keys with their effective values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return runList(opts)
		},
	}

	return cmd
}

func runList(opts *ListOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	for _, key := range cfg.AllKeys() {
		value, getErr := cfg.Get(key)
		if getErr != nil {
			return fmt.Errorf("reading %s: %w", key, getErr)
		}
		fmt.Fprintf(opts.IOStreams.Out, "%s=%s\n", key, value)
	}
	return nil
}
