# orimage

A CLI for generating images through the [OpenRouter image generation API](https://openrouter.ai/docs/api/api-reference/images/generate-an-image).

Built for AI coding agents. Agent harnesses (openclaw in particular) lag behind OpenRouter's
image generation API — new models, parameters like `aspect_ratio`/`resolution`/`seed`, and the
model catalog endpoints aren't exposed. `orimage` gives an agent a complete, current interface
as a plain CLI: deterministic flags, compact JSON output, meaningful exit codes, images written
straight to disk.

## Install

```sh
go install github.com/schmitthub/openrouter-image-cli/cmd/orimage@latest
```

Or grab a signed archive from [releases](https://github.com/schmitthub/openrouter-image-cli/releases).
Every release is attested (SLSA build provenance) and its checksums are signed with cosign:

```sh
gh attestation verify orimage_*.tar.gz --owner schmitthub \
  --signer-workflow schmitthub/openrouter-image-cli/.github/workflows/release-build.yml
```

## Auth

Set `OPENROUTER_API_KEY` — that's it. Commands that don't touch the API (`version`, help) run
without it.

## Usage

Generate an image:

```sh
orimage generate -m google/gemini-2.5-flash-image -p "a red bicycle" -o bike.png
```

Every request option the API supports is a flag:

```sh
orimage generate \
  -m bytedance-seed/seedream-4.5 \
  -p "night market in the rain" \
  -o market.png \
  -n 3 \
  --size 2K \
  --quality high \
  --output-format webp \
  --aspect-ratio 16:9 \
  --seed 42 \
  --input-reference https://example.com/style.png
```

With `-n 3`, files land as `market-1.png`, `market-2.png`, `market-3.png`.

Discover models (agents: use `--json` and pipe to `jq`):

```sh
orimage models list                        # table: id, name, input modalities, streaming
orimage models list --json                 # full catalog, compact JSON
orimage models info qwen/qwen-image-3      # per-provider parameters + pricing
orimage models info qwen/qwen-image-3 --json
```

`models info` shows exactly which parameters each provider accepts (enum values, ranges) and
what each image costs — check it before generating with an unfamiliar model.

## Output

- `generate` prints one `✓ <path>` line per image to stdout; cost goes to stderr, so stdout
  stays parseable.
- `--out` names the file; multiple images insert an index before the extension. Without
  `--out`, files are named `orimage-<timestamp>.<ext>`. The extension follows the response's
  media type.
- Errors from the API surface with status and message (e.g. `openrouter: HTTP 402:
  Insufficient credits`); the exit code is non-zero on any failure.

## License

MIT
