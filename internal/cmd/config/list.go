package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	appconfig "github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

type ListOptions struct {
	IOStreams *iostreams.IOStreams
	Config    func() (appconfig.Config, error)
}

func NewCmdConfigList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
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
	out := opts.IOStreams.Out
	for _, key := range appconfig.Keys() {
		if key == appconfig.KeyProvider {
			continue
		}
		value, getErr := cfg.Get(key)
		if getErr != nil {
			return fmt.Errorf("reading %s: %w", key, getErr)
		}
		fmt.Fprintf(out, "%s=%s\n", key, value)
	}
	provider := cfg.Provider()
	for _, sub := range slices.Sorted(maps.Keys(provider)) {
		fmt.Fprintf(out, "%s.%s=%v\n", appconfig.KeyProvider, sub, provider[sub])
	}
	return nil
}
