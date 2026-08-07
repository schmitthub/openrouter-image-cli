package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	appconfig "github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

type GetOptions struct {
	IOStreams *iostreams.IOStreams
	Config    func() (appconfig.Config, error)

	Key string
}

func NewCmdConfigGet(f *cmdutil.Factory, runF func(*GetOptions) error) *cobra.Command {
	opts := &GetOptions{
		IOStreams: f.IOStreams,
		Config:    f.Config,
		Key:       "",
	}

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print the effective value for a config key",
		Example: `  orimage config get model
  orimage config get provider.sort`,
		Args: cmdutil.ExactArgs(1, "expected a config key"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Key = args[0]
			if runF != nil {
				return runF(opts)
			}
			return runGet(opts)
		},
	}

	return cmd
}

func runGet(opts *GetOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	value, err := cfg.Get(opts.Key)
	if err != nil {
		return cmdutil.FlagErrorWrap(err)
	}
	fmt.Fprintln(opts.IOStreams.Out, value)
	return nil
}
