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

	ios := opts.IOStreams
	var models []openrouter.ImageModel
	err = ios.RunWithProgress("Fetching models", func() error {
		var listErr error
		models, listErr = client.ListImageModels(ctx)
		if listErr != nil {
			return fmt.Errorf("listing image models: %w", listErr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if opts.JSON {
		out, jsonErr := cmdutil.JSONStringify(models, true)
		if jsonErr != nil {
			return fmt.Errorf("encoding models: %w", jsonErr)
		}
		fmt.Fprint(ios.Out, out)
		return nil
	}

	if pagerErr := ios.StartPager(); pagerErr != nil {
		fmt.Fprintf(ios.ErrOut, "failed to start pager: %v\n", pagerErr)
	}
	defer ios.StopPager()

	if !ios.IsStdoutTTY() {
		// Piped: plain tab-separated rows, no header, machine-friendly.
		for _, m := range models {
			fmt.Fprintf(ios.Out, "%s\t%s\t%s\t%v\n",
				m.ID, m.Name, strings.Join(m.Architecture.InputModalities, ","), m.SupportsStreaming)
		}
		return nil
	}

	cs := ios.ColorScheme()
	const tabWidth, padding = 4, 2
	tw := tabwriter.NewWriter(ios.Out, 0, tabWidth, padding, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tINPUT\tSTREAMING")
	for _, m := range models {
		streaming := cs.FailureIcon()
		if m.SupportsStreaming {
			streaming = cs.SuccessIcon()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			m.ID, m.Name, strings.Join(m.Architecture.InputModalities, ","), streaming)
	}
	if flushErr := tw.Flush(); flushErr != nil {
		return fmt.Errorf("writing table: %w", flushErr)
	}
	fmt.Fprintln(ios.ErrOut, cs.Grayf("%d models", len(models)))
	return nil
}
