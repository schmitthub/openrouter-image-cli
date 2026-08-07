// Package config is the parent command for reading and persisting
// generation defaults.
package config

import (
	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"

	getCmd "github.com/schmitthub/openrouter-generate/internal/cmd/config/get"
	listCmd "github.com/schmitthub/openrouter-generate/internal/cmd/config/list"
	pathCmd "github.com/schmitthub/openrouter-generate/internal/cmd/config/path"
	setCmd "github.com/schmitthub/openrouter-generate/internal/cmd/config/set"
)

func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <command>",
		Short: "Manage persisted generation defaults",
		Long: `Read and persist default settings for image generation.

Settings live in config.yaml under $ORGEN_CONFIG_DIR, or the user
config directory under "orgen" when unset. Every key can be
overridden by an ORGEN_* environment variable (dots become
underscores), and command-line flags override both. Run "config list"
to see the available keys.`,
		Example: `  orgen config set model google/gemini-2.5-flash-image
  orgen config get model
  orgen config list`,
	}

	cmd.AddCommand(getCmd.NewCmdGet(f, nil))
	cmd.AddCommand(setCmd.NewCmdSet(f, nil))
	cmd.AddCommand(listCmd.NewCmdList(f, nil))
	cmd.AddCommand(pathCmd.NewCmdPath(f, nil))

	return cmd
}
