// Package skill is the parent command for managing the agent skill
// bundled with the orimage binary.
package skill

import (
	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"

	installCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/skill/install"
)

func NewCmdSkill(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill <command>",
		Short: "Manage the bundled agent skill",
	}

	cmd.AddCommand(installCmd.NewCmdInstall(f, nil))

	return cmd
}
