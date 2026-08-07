package info

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

type InfoOptions struct {
	IOStreams  *iostreams.IOStreams
	OpenRouter func() (*openrouter.Client, error)

	Model string
	JSON  bool
}

func NewCmdInfo(f *cmdutil.Factory, runF func(*InfoOptions) error) *cobra.Command {
	opts := &InfoOptions{
		IOStreams:  f.IOStreams,
		OpenRouter: f.OpenRouter,
	}

	cmd := &cobra.Command{
		Use:   "info <model-id>",
		Short: "Show a model's providers, parameters, and pricing",
		Long: `Show per-provider details for one image model
(GET /images/models/{id}/endpoints): supported parameters, streaming
support, and pricing. Requires the OPENROUTER_API_KEY environment variable.`,
		Example: `  orimage models info qwen/qwen-image-3`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Model = args[0]
			if runF != nil {
				return runF(opts)
			}
			return runInfo(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output the full detail as JSON")

	return cmd
}

func runInfo(ctx context.Context, opts *InfoOptions) error {
	client, err := opts.OpenRouter()
	if err != nil {
		return err
	}

	eps, err := client.GetImageModelEndpoints(ctx, opts.Model)
	if err != nil {
		return fmt.Errorf("fetching model endpoints: %w", err)
	}

	ios := opts.IOStreams
	if opts.JSON {
		out, jsonErr := cmdutil.JSONStringify(eps, false)
		if jsonErr != nil {
			return fmt.Errorf("encoding model detail: %w", jsonErr)
		}
		fmt.Fprint(ios.Out, out)
		return nil
	}

	cs := ios.ColorScheme()
	fmt.Fprintln(ios.Out, cs.Bold(eps.ID))
	for _, ep := range eps.Endpoints {
		fmt.Fprintf(ios.Out, "\n%s (%s)\n", cs.Bold(ep.ProviderName), ep.ProviderSlug)
		fmt.Fprintf(ios.Out, "  streaming: %v\n", ep.SupportsStreaming)
		printParams(ios, ep.SupportedParameters)
		printPricing(ios, ep.Pricing)
	}
	return nil
}

// printParams renders the parameter specs sorted by name for stable output.
func printParams(ios *iostreams.IOStreams, params map[string]openrouter.ParamSpec) {
	if len(params) == 0 {
		return
	}
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintln(ios.Out, "  parameters:")
	for _, name := range names {
		fmt.Fprintf(ios.Out, "    %s: %s\n", name, formatSpec(params[name]))
	}
}

// formatSpec renders one ParamSpec: enum values, a min-max range, or the
// bare type name.
func formatSpec(spec openrouter.ParamSpec) string {
	switch {
	case spec.Type == "enum":
		return strings.Join(spec.Values, ", ")
	case spec.Type == "range" && spec.Min != nil && spec.Max != nil:
		return fmt.Sprintf("%d-%d", *spec.Min, *spec.Max)
	default:
		return spec.Type
	}
}

func printPricing(ios *iostreams.IOStreams, pricing []openrouter.Price) {
	if len(pricing) == 0 {
		return
	}
	fmt.Fprintln(ios.Out, "  pricing:")
	for _, p := range pricing {
		variant := ""
		if p.Variant != "" {
			variant = " (" + p.Variant + ")"
		}
		fmt.Fprintf(ios.Out, "    %s%s: $%g per %s\n", p.Billable, variant, p.CostUSD, p.Unit)
	}
}
