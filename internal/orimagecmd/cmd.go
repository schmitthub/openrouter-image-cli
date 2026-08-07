package orimagecmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/schmitthub/openrouter-image-cli/internal/build"
	"github.com/schmitthub/openrouter-image-cli/internal/cmd/factory"
	"github.com/schmitthub/openrouter-image-cli/internal/cmd/root"
	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"

	"github.com/spf13/cobra"
)

type exitCode int

const (
	exitOk    exitCode = 0
	exitError exitCode = 1
)

func Main() exitCode {
	buildDate := build.Date
	buildVersion := build.Version
	ioStreams := iostreams.System()

	stderr := ioStreams.ErrOut

	cmdFactory := factory.New(buildVersion, ioStreams)

	ctx := context.Background()

	rootCmd, err := root.NewCmdRoot(cmdFactory, buildVersion, buildDate)
	if err != nil {
		fmt.Fprintf(stderr, "failed to create root command: %v\n", err)
		return exitError
	}

	if cmd, err := rootCmd.ExecuteContextC(ctx); err != nil {
		printError(stderr, err, cmd)
		return exitError
	}

	return exitOk
}

func printError(out io.Writer, err error, cmd *cobra.Command) {
	fmt.Fprintln(out, err)

	var flagError *cmdutil.FlagError
	if errors.As(err, &flagError) ||
		strings.HasPrefix(err.Error(), "unknown command ") ||
		strings.HasPrefix(err.Error(), "required flag(s)") {
		if !strings.HasSuffix(err.Error(), "\n") {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, cmd.UsageString())
	}
}
