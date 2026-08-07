// Package config implements the "orimage config" command group for
// reading and persisting generation defaults.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	appconfig "github.com/schmitthub/openrouter-image-cli/internal/config"
)

func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <command>",
		Short: "Manage persisted generation defaults",
		Long: fmt.Sprintf(`Read and persist default settings for image generation.

Settings live in config.yaml under $ORIMAGE_CONFIG_DIR, or the user
config directory under "orimage" when unset. Keys: %s; provider
routing settings nest as provider.<setting>. Every key can be
overridden by an ORIMAGE_* environment variable (dots become
underscores), and command-line flags override both.`,
			strings.Join(appconfig.Keys(), ", ")),
		Example: `  orimage config set model google/gemini-2.5-flash-image
  orimage config get model
  orimage config list`,
	}

	cmd.AddCommand(NewCmdConfigGet(f, nil))
	cmd.AddCommand(NewCmdConfigSet(f, nil))
	cmd.AddCommand(NewCmdConfigList(f, nil))
	cmd.AddCommand(NewCmdConfigPath(f, nil))

	return cmd
}
