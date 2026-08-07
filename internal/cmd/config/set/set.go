// Package set implements "orimage config set".
package set

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
)

// setArgCount is the <key> <value> pair "config set" expects.
const setArgCount = 2

type SetOptions struct {
	IOStreams *iostreams.IOStreams
	Config    func() (config.Config, error)

	Key   string
	Value string
}

func NewCmdSet(f *cmdutil.Factory, runF func(*SetOptions) error) *cobra.Command {
	opts := &SetOptions{
		IOStreams: f.IOStreams,
		Config:    f.Config,
		Key:       "",
		Value:     "",
	}

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist a default value for a config key",
		Example: `  orimage config set model google/gemini-2.5-flash-image
  orimage config set aspect_ratio 16:9
  orimage config set provider.sort price`,
		Args: cmdutil.ExactArgs(setArgCount, "expected a config key and a value"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Key, opts.Value = args[0], args[1]
			if runF != nil {
				return runF(opts)
			}
			return runSet(opts)
		},
	}

	return cmd
}

func runSet(opts *SetOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if setErr := cfg.Set(opts.Key, opts.Value); setErr != nil {
		return cmdutil.FlagErrorWrap(setErr)
	}
	if saveErr := cfg.Save(); saveErr != nil {
		return fmt.Errorf("saving config: %w", saveErr)
	}
	if opts.IOStreams.IsStdoutTTY() {
		cs := opts.IOStreams.ColorScheme()
		fmt.Fprintf(opts.IOStreams.Out, "%s %s = %s\n", cs.SuccessIcon(), opts.Key, opts.Value)
	}
	return nil
}
