package generate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/schmitthub/openrouter-generate/internal/cmdutil"
	"github.com/schmitthub/openrouter-generate/internal/config"
	"github.com/schmitthub/openrouter-generate/internal/iostreams"
	"github.com/schmitthub/openrouter-generate/internal/openrouter"
)

// Output formats and their file extensions.
const (
	formatPNG  = "png"
	formatJPEG = "jpeg"
	formatWebP = "webp"
	formatSVG  = "svg"
)

// Provider sorting strategies accepted by the API.
const (
	sortPrice      = "price"
	sortThroughput = "throughput"
	sortLatency    = "latency"
	sortExacto     = "exacto"
)

// Allowed values for the enum flags, mirroring the API's schema.
// https://openrouter.ai/docs/api/api-reference/images/generate-an-image
var (
	qualities   = []string{"auto", "low", "medium", "high"}              //nolint:gochecknoglobals // flag vocabulary
	formats     = []string{formatPNG, formatJPEG, formatWebP, formatSVG} //nolint:gochecknoglobals // flag vocabulary
	resolutions = []string{"512", "1K", "2K", "4K"}                      //nolint:gochecknoglobals // flag vocabulary
	backgrounds = []string{"auto", "transparent", "opaque"}              //nolint:gochecknoglobals // flag vocabulary
)

const (
	minImages      = 1
	maxImages      = 10
	maxCompression = 100
)

type GenerateOptions struct {
	IOStreams  *iostreams.IOStreams
	OpenRouter func() (*openrouter.Client, error)
	Config     func() (config.Config, error)

	// Request options (see openrouter.ImageRequest).
	Model             string
	Prompt            string
	N                 int
	Size              string
	Quality           string
	OutputFormat      string
	OutputCompression int
	CompressionSet    bool
	AspectRatio       string
	Resolution        string
	Background        string
	Seed              int64
	SeedSet           bool
	InputReferences   []string

	// Provider routing preferences (see openrouter.ProviderPreferences).
	ProviderSort        string
	ProviderOrder       []string
	ProviderOnly        []string
	ProviderIgnore      []string
	NoProviderFallbacks bool
	FallbacksSet        bool
	// ProviderOptions holds raw --provider-option <slug>.<key>=<value>
	// entries; ProviderOptionValues is the parsed result layered over
	// config-sourced passthrough options.
	ProviderOptions      []string
	ProviderOptionValues map[string]map[string]any

	// Out is where images are written: a file path, or with N > 1 a
	// pattern where an index is inserted before the extension. Empty
	// derives a name from the response timestamp.
	Out string
}

func NewCmdGenerate(f *cmdutil.Factory, runF func(*GenerateOptions) error) *cobra.Command {
	opts := &GenerateOptions{
		IOStreams:         f.IOStreams,
		OpenRouter:        f.OpenRouter,
		Config:            f.Config,
		Model:             "",
		Prompt:            "",
		N:                 0,
		Size:              "",
		Quality:           "",
		OutputFormat:      "",
		OutputCompression: 0,
		CompressionSet:    false,
		AspectRatio:       "",
		Resolution:        "",
		Background:        "",
		Seed:              0,
		SeedSet:           false,
		InputReferences:   nil,

		ProviderSort:         "",
		ProviderOrder:        nil,
		ProviderOnly:         nil,
		ProviderIgnore:       nil,
		NoProviderFallbacks:  false,
		FallbacksSet:         false,
		ProviderOptions:      nil,
		ProviderOptionValues: nil,

		Out: "",
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate an image",
		Long: `Generate one or more images with an OpenRouter image model and write
them to disk. Requires the OPENROUTER_API_KEY environment variable.`,
		Example: `  orgen generate -m google/gemini-2.5-flash-image -p "a red bicycle"
  orgen generate -m bytedance-seed/seedream-4.5 -p "night market, rain" \
    -o market.png --size 2K --quality high -n 3`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.SeedSet = cmd.Flags().Changed("seed")
			opts.CompressionSet = cmd.Flags().Changed("output-compression")
			opts.FallbacksSet = cmd.Flags().Changed("no-provider-fallbacks")
			if err := applyConfigDefaults(cmd, opts); err != nil {
				return err
			}
			if err := resolveProviderOptions(opts); err != nil {
				return err
			}
			if err := validateOptions(opts); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return runGenerate(cmd.Context(), opts)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&opts.Model, "model", "m", "", "Image model identifier (required unless a default is configured)")
	fl.StringVarP(&opts.Prompt, "prompt", "p", "", "Text description of the desired image (required)")
	fl.StringVarP(&opts.Out, "out", "o", "", "Output file path (default: orgen-<timestamp>.<ext>)")
	fl.IntVarP(&opts.N, "count", "n", 0, "Number of images to generate (1-10)")
	fl.StringVar(&opts.Size, "size", "", `Image size: shorthand ("2K", "4K") or pixels ("2048x2048")`)
	fl.StringVar(&opts.Quality, "quality", "", "Quality: auto|low|medium|high")
	fl.StringVar(&opts.OutputFormat, "output-format", "", "Format: png|jpeg|webp|svg")
	fl.IntVar(&opts.OutputCompression, "output-compression", 0, "Compression level 0-100 (jpeg/webp)")
	fl.StringVar(&opts.AspectRatio, "aspect-ratio", "", `Aspect ratio, e.g. "1:1", "16:9", "9:16"`)
	fl.StringVar(&opts.Resolution, "resolution", "", "Resolution: 512|1K|2K|4K")
	fl.StringVar(&opts.Background, "background", "", "Background: auto|transparent|opaque")
	fl.Int64Var(&opts.Seed, "seed", 0, "Seed for deterministic generation")
	fl.StringArrayVar(&opts.InputReferences, "input-reference", nil,
		"Reference image (file path, URL, or base64) for image-to-image; repeatable, max 16")
	fl.StringVar(&opts.ProviderSort, "provider-sort", "",
		"Provider sorting strategy: price|throughput|latency|exacto")
	fl.StringSliceVar(&opts.ProviderOrder, "provider-order", nil,
		"Provider slugs to attempt in order; repeatable or comma-separated")
	fl.StringSliceVar(&opts.ProviderOnly, "provider-only", nil,
		"Provider slugs to allow; repeatable or comma-separated")
	fl.StringSliceVar(&opts.ProviderIgnore, "provider-ignore", nil,
		"Provider slugs to exclude; repeatable or comma-separated")
	fl.BoolVar(&opts.NoProviderFallbacks, "no-provider-fallbacks", false,
		"Fail with the upstream error instead of falling back to another provider")
	fl.StringArrayVar(&opts.ProviderOptions, "provider-option", nil,
		"Provider passthrough parameter as <slug>.<key>=<value>; repeatable "+
			"(discover keys with 'orgen models info <model>')")

	_ = cmd.MarkFlagRequired("prompt")
	_ = cmd.MarkFlagFilename("out")

	return cmd
}

// applyConfigDefaults fills options the user left unset from persisted
// config. Flags always win; viper already ranks environment variables
// over file entries.
func applyConfigDefaults(cmd *cobra.Command, opts *GenerateOptions) error {
	if opts.Config == nil {
		return nil
	}
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	flags := cmd.Flags()
	defaults := []struct {
		flag string
		dst  *string
		get  func() string
	}{
		{flag: "model", dst: &opts.Model, get: cfg.Model},
		{flag: "aspect-ratio", dst: &opts.AspectRatio, get: cfg.AspectRatio},
		{flag: "output-format", dst: &opts.OutputFormat, get: cfg.OutputFormat},
		{flag: "provider-sort", dst: &opts.ProviderSort, get: cfg.ProviderSort},
	}
	for _, d := range defaults {
		if v := d.get(); v != "" && !flags.Changed(d.flag) {
			*d.dst = v
		}
	}
	if n, ok := cfg.OutputCompression(); ok && !flags.Changed("output-compression") {
		opts.OutputCompression = n
		opts.CompressionSet = true
	}
	applyProviderDefaults(flags, cfg, opts)
	return nil
}

// applyProviderDefaults fills unset provider routing options from
// config. Config-sourced passthrough options become the base that
// --provider-option entries later layer over.
func applyProviderDefaults(flags *pflag.FlagSet, cfg config.Config, opts *GenerateOptions) {
	lists := []struct {
		flag string
		dst  *[]string
		get  func() []string
	}{
		{flag: "provider-order", dst: &opts.ProviderOrder, get: cfg.ProviderOrder},
		{flag: "provider-only", dst: &opts.ProviderOnly, get: cfg.ProviderOnly},
		{flag: "provider-ignore", dst: &opts.ProviderIgnore, get: cfg.ProviderIgnore},
	}
	for _, d := range lists {
		if v := d.get(); len(v) > 0 && !flags.Changed(d.flag) {
			*d.dst = v
		}
	}
	if allow, ok := cfg.ProviderAllowFallbacks(); ok && !flags.Changed("no-provider-fallbacks") {
		opts.NoProviderFallbacks = !allow
		opts.FallbacksSet = true
	}
	opts.ProviderOptionValues = cfg.ProviderOptions()
}

// resolveProviderOptions parses --provider-option entries onto the
// config-sourced passthrough base; flag values win per key. Values are
// coerced so numbers and booleans reach the wire typed.
func resolveProviderOptions(opts *GenerateOptions) error {
	for _, raw := range opts.ProviderOptions {
		spec, value, ok := strings.Cut(raw, "=")
		slug, key, specOK := strings.Cut(spec, ".")
		if !ok || !specOK || slug == "" || key == "" {
			return cmdutil.FlagErrorf(
				"invalid provider-option %q (expected <slug>.<key>=<value>)", raw)
		}
		if opts.ProviderOptionValues == nil {
			opts.ProviderOptionValues = map[string]map[string]any{}
		}
		if opts.ProviderOptionValues[slug] == nil {
			opts.ProviderOptionValues[slug] = map[string]any{}
		}
		opts.ProviderOptionValues[slug][key] = config.CoerceScalar(value)
	}
	return nil
}

// validateOptions rejects values the API would refuse, before any network
// call. Errors are FlagErrors so the root command prints usage.
func validateOptions(opts *GenerateOptions) error {
	if opts.Model == "" {
		return cmdutil.FlagErrorf(`required flag(s) "model" not set`)
	}
	enums := []struct {
		name    string
		value   string
		allowed []string
	}{
		{"quality", opts.Quality, qualities},
		{"output-format", opts.OutputFormat, formats},
		{"resolution", opts.Resolution, resolutions},
		{"background", opts.Background, backgrounds},
		{"provider-sort", opts.ProviderSort, []string{sortPrice, sortThroughput, sortLatency, sortExacto}},
	}
	for _, e := range enums {
		if e.value != "" && !slices.Contains(e.allowed, e.value) {
			return cmdutil.FlagErrorf(
				"invalid %s: %s (expected one of: %s)",
				e.name, e.value, strings.Join(e.allowed, ", "))
		}
	}
	if opts.N != 0 && (opts.N < minImages || opts.N > maxImages) {
		return cmdutil.FlagErrorf("invalid count: %d (expected %d-%d)", opts.N, minImages, maxImages)
	}
	if opts.CompressionSet &&
		(opts.OutputCompression < 0 || opts.OutputCompression > maxCompression) {
		return cmdutil.FlagErrorf(
			"invalid output-compression: %d (expected 0-%d)", opts.OutputCompression, maxCompression)
	}
	if len(opts.InputReferences) > maxInputReferences {
		return cmdutil.FlagErrorf(
			"too many input-reference values: %d (max %d)", len(opts.InputReferences), maxInputReferences)
	}
	if slices.Contains(opts.InputReferences, "") {
		return cmdutil.FlagErrorf("input-reference values must not be empty")
	}
	for _, l := range []struct {
		name  string
		slugs []string
	}{
		{"provider-order", opts.ProviderOrder},
		{"provider-only", opts.ProviderOnly},
		{"provider-ignore", opts.ProviderIgnore},
	} {
		if slices.Contains(l.slugs, "") {
			return cmdutil.FlagErrorf("%s slugs must not be empty", l.name)
		}
	}
	return nil
}

// buildRequest translates CLI options into the wire request.
func buildRequest(opts *GenerateOptions) openrouter.ImageRequest {
	req := openrouter.ImageRequest{
		Model:             opts.Model,
		Prompt:            opts.Prompt,
		N:                 opts.N,
		Size:              opts.Size,
		Quality:           opts.Quality,
		OutputFormat:      opts.OutputFormat,
		OutputCompression: nil,
		AspectRatio:       opts.AspectRatio,
		Resolution:        opts.Resolution,
		Background:        opts.Background,
		Seed:              nil,
		InputReferences:   nil,
		Provider:          buildProviderPreferences(opts),
	}
	for _, ref := range opts.InputReferences {
		req.InputReferences = append(req.InputReferences, openrouter.NewInputReference(ref))
	}
	if opts.CompressionSet {
		req.OutputCompression = &opts.OutputCompression
	}
	if opts.SeedSet {
		req.Seed = &opts.Seed
	}
	return req
}

// buildProviderPreferences assembles the provider routing object, or
// nil when no routing option is set so the field stays off the wire.
func buildProviderPreferences(opts *GenerateOptions) *openrouter.ProviderPreferences {
	prefs := &openrouter.ProviderPreferences{
		Order:          opts.ProviderOrder,
		Only:           opts.ProviderOnly,
		Ignore:         opts.ProviderIgnore,
		Sort:           opts.ProviderSort,
		AllowFallbacks: nil,
		Options:        opts.ProviderOptionValues,
	}
	if opts.FallbacksSet {
		allow := !opts.NoProviderFallbacks
		prefs.AllowFallbacks = &allow
	}
	if len(prefs.Order) == 0 && len(prefs.Only) == 0 && len(prefs.Ignore) == 0 &&
		prefs.Sort == "" && prefs.AllowFallbacks == nil && len(prefs.Options) == 0 {
		return nil
	}
	return prefs
}

// warnUnknownPassthrough warns on stderr when a passthrough option
// names a provider or key the model's endpoints do not advertise in
// allowed_passthrough_parameters — the API drops such keys silently.
// Best-effort: a failed endpoints lookup skips the check and never
// blocks generation.
func warnUnknownPassthrough(ctx context.Context, opts *GenerateOptions, client *openrouter.Client) {
	if len(opts.ProviderOptionValues) == 0 {
		return
	}
	eps, err := client.GetImageModelEndpoints(ctx, opts.Model)
	if err != nil {
		return
	}
	allowed := passthroughIndex(eps)
	cs := opts.IOStreams.ColorScheme()
	for _, slug := range slices.Sorted(maps.Keys(opts.ProviderOptionValues)) {
		keys, ok := allowed[slug]
		if !ok {
			fmt.Fprintf(opts.IOStreams.ErrOut,
				"%s %s has no %q endpoint; its provider options will be ignored\n",
				cs.WarningIcon(), opts.Model, slug)
			continue
		}
		for _, key := range slices.Sorted(maps.Keys(opts.ProviderOptionValues[slug])) {
			if !keys[key] {
				fmt.Fprintf(opts.IOStreams.ErrOut,
					"%s %q is not an advertised passthrough key for %s on %s\n",
					cs.WarningIcon(), key, slug, opts.Model)
			}
		}
	}
}

// passthroughIndex maps each provider slug to the set of passthrough
// keys its endpoint advertises.
func passthroughIndex(eps *openrouter.ModelEndpoints) map[string]map[string]bool {
	allowed := make(map[string]map[string]bool, len(eps.Endpoints))
	for _, ep := range eps.Endpoints {
		keys := make(map[string]bool, len(ep.AllowedPassthroughParameters))
		for _, k := range ep.AllowedPassthroughParameters {
			keys[k] = true
		}
		allowed[ep.ProviderSlug] = keys
	}
	return allowed
}

func runGenerate(ctx context.Context, opts *GenerateOptions) error {
	client, err := opts.OpenRouter()
	if err != nil {
		return err
	}

	refs, err := resolveInputReferences(opts.InputReferences)
	if err != nil {
		return err
	}
	opts.InputReferences = refs

	warnUnknownPassthrough(ctx, opts, client)

	ios := opts.IOStreams
	var resp *openrouter.ImageResponse
	err = ios.RunWithProgress("Generating", func() error {
		var genErr error
		resp, genErr = client.GenerateImage(ctx, buildRequest(opts))
		if genErr != nil {
			return fmt.Errorf("generating image: %w", genErr)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(resp.Data) == 0 {
		return errors.New("openrouter returned no images")
	}

	paths, err := saveImages(resp, opts.Out)
	if err != nil {
		return err
	}

	cs := ios.ColorScheme()
	for _, p := range paths {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.SuccessIcon(), p)
	}
	if resp.ProviderName != "" {
		fmt.Fprintf(ios.ErrOut, "%s\n", cs.Grayf("provider: %s", resp.ProviderName))
	}
	if resp.Usage.Cost > 0 {
		fmt.Fprintf(ios.ErrOut, "%s\n", cs.Grayf("cost: $%.4f (%d tokens)",
			resp.Usage.Cost, resp.Usage.TotalTokens))
	}
	return nil
}
