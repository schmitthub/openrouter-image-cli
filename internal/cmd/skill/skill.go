// Package skill is the parent command for managing the agent skill
// bundled with the orgen binary.
package skill

import (
	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"

	installCmd "github.com/schmitthub/openrouter-generate/internal/cmd/skill/install"
)

func NewCmdSkill(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill <command>",
		Short: "Manage the bundled agent skill",
	}

	cmd.AddCommand(installCmd.NewCmdInstall(f, nil))

	return cmd
}
