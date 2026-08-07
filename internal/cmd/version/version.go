package version

import (
	"fmt"
	"strings"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"

	"github.com/spf13/cobra"
)

// NewCmdVersion creates the "version" subcommand.
func NewCmdVersion(f *cmdutil.Factory, version, buildDate string) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprint(f.IOStreams.Out, cmd.Root().Annotations["versionInfo"])
		},
	}

	return cmd
}

// Format returns the version string for display.
func Format(version, buildDate string) string {
	version = strings.TrimPrefix(version, "v")

	var dateStr string
	if buildDate != "" {
		dateStr = fmt.Sprintf(" (%s)", buildDate)
	}

	return fmt.Sprintf("orgen version %s%s\n", version, dateStr)
}
