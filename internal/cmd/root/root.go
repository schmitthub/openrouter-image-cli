package root

import (
	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"

	generateCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/generate"
	modelsCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/models"
	skillCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/skill"
	versionCmd "github.com/schmitthub/openrouter-image-cli/internal/cmd/version"
)

func NewCmdRoot(f *cmdutil.Factory, version, buildDate string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "orimage",
		Short: "OpenRouter image generation",
		Long:  "Generate images with models served through the OpenRouter API.",
		Example: `  orimage generate --prompt "a red bicycle" --out ./bike.png
`,
		Annotations: map[string]string{
			"versionInfo": versionCmd.Format(version, buildDate),
		},
		Version: f.AppVersion,
	}

	// override cobra's default behaviors
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetFlagErrorFunc(rootFlagErrorFunc)

	cmd.PersistentFlags().Bool("help", false, "Show help for command")

	cmd.AddCommand(versionCmd.NewCmdVersion(f, version, buildDate))
	cmd.AddCommand(generateCmd.NewCmdGenerate(f, nil))
	cmd.AddCommand(modelsCmd.NewCmdModels(f))
	cmd.AddCommand(skillCmd.NewCmdSkill(f))

	return cmd, nil
}
