// Package models is the parent command for the image model catalog
// endpoints (/images/models and /images/models/{id}/endpoints).
package models

import (
	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"

	infoCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/models/info"
	listCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/models/list"
)

func NewCmdModels(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models <command>",
		Short: "Explore available image models",
	}

	cmd.AddCommand(listCmd.NewCmdList(f, nil))
	cmd.AddCommand(infoCmd.NewCmdInfo(f, nil))

	return cmd
}
