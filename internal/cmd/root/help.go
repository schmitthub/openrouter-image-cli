package root

import (
	"github.com/schmitthub/openrouter-generate/internal/cmdutil"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func rootFlagErrorFunc(cmd *cobra.Command, err error) error {
	if err == pflag.ErrHelp {
		return err
	}
	return cmdutil.FlagErrorWrap(err)
}
