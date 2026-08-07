package list

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"
	"github.com/schmitthub/openrouter-generate/internal/iostreams"
	"github.com/schmitthub/openrouter-generate/internal/openrouter"
)

// pricingWorkers bounds the concurrent per-model endpoint lookups that
// back the cost column; the list API itself carries no pricing.
const pricingWorkers = 8

type ListOptions struct {
	IOStreams  *iostreams.IOStreams
	OpenRouter func() (*openrouter.Client, error)

	JSON bool
}

// listedModel augments a catalog entry with the flattened endpoint
// pricing fetched separately for the cost column.
type listedModel struct {
	openrouter.ImageModel

	Pricing []openrouter.Price `json:"pricing,omitempty"`
}

func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IOStreams:  f.IOStreams,
		OpenRouter: f.OpenRouter,
		JSON:       false,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available image models",
		Long: `List the image models available through the OpenRouter API
(GET /images/models), with each model's output pricing gathered from its
per-provider endpoints. Requires the OPENROUTER_API_KEY environment variable.`,
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
	var models []listedModel
	var priceFailures int
	err = ios.RunWithProgress("Fetching models", func() error {
		catalog, listErr := client.ListImageModels(ctx)
		if listErr != nil {
			return fmt.Errorf("listing image models: %w", listErr)
		}
		models, priceFailures = fetchPricing(ctx, client, catalog)
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
	return renderRows(ios, models, priceFailures)
}

// fetchPricing looks up every model's per-provider endpoints and
// flattens their pricing entries onto the model. Lookups run
// concurrently; a failed lookup leaves that model's pricing nil and is
// only counted, never fatal — the list still renders.
func fetchPricing(
	ctx context.Context, client *openrouter.Client, catalog []openrouter.ImageModel,
) ([]listedModel, int) {
	models := make([]listedModel, len(catalog))
	var failures atomic.Int64
	sem := make(chan struct{}, pricingWorkers)
	var wg sync.WaitGroup
	for i, m := range catalog {
		models[i] = listedModel{ImageModel: m, Pricing: nil}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			eps, err := client.GetImageModelEndpoints(ctx, m.ID)
			if err != nil {
				failures.Add(1)
				return
			}
			for _, ep := range eps.Endpoints {
				models[i].Pricing = append(models[i].Pricing, ep.Pricing...)
			}
		})
	}
	wg.Wait()
	return models, int(failures.Load())
}

func renderRows(ios *iostreams.IOStreams, models []listedModel, priceFailures int) error {
	if pagerErr := ios.StartPager(); pagerErr != nil {
		fmt.Fprintf(ios.ErrOut, "failed to start pager: %v\n", pagerErr)
	}
	defer ios.StopPager()

	if !ios.IsStdoutTTY() {
		// Piped: plain tab-separated rows, no header, machine-friendly.
		for _, m := range models {
			fmt.Fprintf(ios.Out, "%s\t%s\t%s\t%v\t%s\n",
				m.ID, m.Name, strings.Join(m.Architecture.InputModalities, ","),
				m.SupportsStreaming, formatCost(m.Pricing))
		}
		return nil
	}

	cs := ios.ColorScheme()
	const tabWidth, padding = 4, 2
	tw := tabwriter.NewWriter(ios.Out, 0, tabWidth, padding, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tINPUT\tSTREAMING\tCOST")
	for _, m := range models {
		streaming := cs.FailureIcon()
		if m.SupportsStreaming {
			streaming = cs.SuccessIcon()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			m.ID, m.Name, strings.Join(m.Architecture.InputModalities, ","),
			streaming, formatCost(m.Pricing))
	}
	if flushErr := tw.Flush(); flushErr != nil {
		return fmt.Errorf("writing table: %w", flushErr)
	}
	fmt.Fprintln(ios.ErrOut, cs.Grayf("%d models", len(models)))
	if priceFailures > 0 {
		fmt.Fprintln(ios.ErrOut, cs.Grayf("pricing unavailable for %d models", priceFailures))
	}
	return nil
}

// formatCost summarizes a model's output pricing as "$<min>/<unit>",
// e.g. "$0.014/img" or "$0.06/MP", appending "+" before the slash when
// higher-priced tiers or providers exist ("$0.03+/MP"). Models without
// output pricing (absent or empty in the API) render "-".
func formatCost(entries []openrouter.Price) string {
	var cheapest openrouter.Price
	found := false
	distinct := make(map[float64]bool)
	for _, p := range entries {
		if p.Billable != "output_image" {
			continue
		}
		distinct[p.CostUSD] = true
		if !found || p.CostUSD < cheapest.CostUSD {
			cheapest = p
			found = true
		}
	}
	if !found {
		return "-"
	}
	cost := "$" + strconv.FormatFloat(cheapest.CostUSD, 'f', -1, 64)
	if len(distinct) > 1 {
		cost += "+"
	}
	return cost + "/" + costUnit(cheapest.Unit)
}

// costUnit abbreviates a pricing unit for the table.
func costUnit(unit string) string {
	switch unit {
	case "image":
		return "img"
	case "megapixel":
		return "MP"
	case "token":
		return "tok"
	default:
		return unit
	}
}
