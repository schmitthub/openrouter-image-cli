package list

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

type ListOptions struct {
	IOStreams  *iostreams.IOStreams
	OpenRouter func() (*openrouter.Client, error)

	JSON bool
}

func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IOStreams:  f.IOStreams,
		OpenRouter: f.OpenRouter,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available image models",
		Long: `List the image models available through the OpenRouter API
(GET /images/models). Requires the OPENROUTER_API_KEY environment variable.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return runList(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output the full model list as JSON")

	return cmd
}

func runList(ctx context.Context, opts *ListOptions) error {
	client, err := opts.OpenRouter()
	if err != nil {
		return err
	}

	models, err := client.ListImageModels(ctx)
	if err != nil {
		return fmt.Errorf("listing image models: %w", err)
	}

	ios := opts.IOStreams
	if opts.JSON {
		out, jsonErr := cmdutil.JSONStringify(models, false)
		if jsonErr != nil {
			return fmt.Errorf("encoding models: %w", jsonErr)
		}
		fmt.Fprint(ios.Out, out)
		return nil
	}

	const tabWidth, padding = 4, 2
	tw := tabwriter.NewWriter(ios.Out, 0, tabWidth, padding, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tINPUT\tSTREAMING")
	for _, m := range models {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n",
			m.ID, m.Name, strings.Join(m.Architecture.InputModalities, ","), m.SupportsStreaming)
	}
	if flushErr := tw.Flush(); flushErr != nil {
		return fmt.Errorf("writing table: %w", flushErr)
	}
	return nil
}
