package generate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

// Output formats and their file extensions.
const (
	formatPNG  = "png"
	formatJPEG = "jpeg"
	formatWebP = "webp"
	formatSVG  = "svg"
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

	// Out is where images are written: a file path, or with N > 1 a
	// pattern where an index is inserted before the extension. Empty
	// derives a name from the response timestamp.
	Out string
}

func NewCmdGenerate(f *cmdutil.Factory, runF func(*GenerateOptions) error) *cobra.Command {
	opts := &GenerateOptions{
		IOStreams:         f.IOStreams,
		OpenRouter:        f.OpenRouter,
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
		Out:               "",
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate an image",
		Long: `Generate one or more images with an OpenRouter image model and write
them to disk. Requires the OPENROUTER_API_KEY environment variable.`,
		Example: `  orimage generate -m google/gemini-2.5-flash-image -p "a red bicycle"
  orimage generate -m bytedance-seed/seedream-4.5 -p "night market, rain" \
    -o market.png --size 2K --quality high -n 3`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.SeedSet = cmd.Flags().Changed("seed")
			opts.CompressionSet = cmd.Flags().Changed("output-compression")
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
	fl.StringVarP(&opts.Model, "model", "m", "", "Image model identifier (required)")
	fl.StringVarP(&opts.Prompt, "prompt", "p", "", "Text description of the desired image (required)")
	fl.StringVarP(&opts.Out, "out", "o", "", "Output file path (default: orimage-<timestamp>.<ext>)")
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

	_ = cmd.MarkFlagRequired("model")
	_ = cmd.MarkFlagRequired("prompt")
	_ = cmd.MarkFlagFilename("out")

	return cmd
}

// validateOptions rejects values the API would refuse, before any network
// call. Errors are FlagErrors so the root command prints usage.
func validateOptions(opts *GenerateOptions) error {
	enums := []struct {
		name    string
		value   string
		allowed []string
	}{
		{"quality", opts.Quality, qualities},
		{"output-format", opts.OutputFormat, formats},
		{"resolution", opts.Resolution, resolutions},
		{"background", opts.Background, backgrounds},
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
		InputReferences:   opts.InputReferences,
	}
	if opts.CompressionSet {
		req.OutputCompression = &opts.OutputCompression
	}
	if opts.SeedSet {
		req.Seed = &opts.Seed
	}
	return req
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
	if resp.Usage.Cost > 0 {
		fmt.Fprintf(ios.ErrOut, "%s\n", cs.Grayf("cost: $%.4f (%d tokens)",
			resp.Usage.Cost, resp.Usage.TotalTokens))
	}
	return nil
}
