# orimage

A CLI for generating images through the [OpenRouter image generation API](https://openrouter.ai/docs/api/api-reference/images/generate-an-image).

Built for AI coding agents. Agent harnesses (openclaw in particular) don't cover OpenRouter's
dedicated image generation API — their image tools expose only a subset of models, or route
generation through the chat completions API instead. `orimage` gives an agent the full image
API — every model in the catalog, every request parameter — as a plain CLI: deterministic
flags, compact JSON output, meaningful exit codes, images written straight to disk.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/schmitthub/openrouter-image-cli/main/scripts/install.sh | sh
```

Installs the latest release to `/usr/local/bin` (checksum-verified; linux/darwin,
amd64/arm64). Pin a version with `ORIMAGE_VERSION=v2026.8.3`, change the target with
`ORIMAGE_INSTALL_DIR=~/bin`.

Or with Go:

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
  --input-reference ./style.png
```

With `-n 3`, files land as `market-1.png`, `market-2.png`, `market-3.png`.

`--input-reference` (repeatable, max 16) accepts a local file path, an HTTP(S) URL, or raw
base64. Local files are read and sent as base64 data URIs — no need to host them anywhere.
URLs are fetched by OpenRouter server-side, so they must be publicly reachable.

Discover models (agents: use `--json` and pipe to `jq`):

```sh
orimage models list                        # table: id, name, input modalities, streaming
orimage models list --json                 # full catalog, compact JSON
orimage models info qwen/qwen-image-3      # per-provider parameters + pricing
orimage models info qwen/qwen-image-3 --json
```

`models info` shows exactly which parameters each provider accepts (enum values, ranges) and
what each image costs — check it before generating with an unfamiliar model.

## Configuration

Persist defaults so you can stop repeating flags:

```sh
orimage config set model google/gemini-2.5-flash-image
orimage config set aspect_ratio 16:9
orimage config list                        # effective values, key=value per line
orimage config path                        # where config.yaml lives
```

Settings live in `config.yaml` under your OS config directory (`$XDG_CONFIG_HOME/orimage` on
Linux), or `$ORIMAGE_CONFIG_DIR` when set. Keys: `model`, `aspect_ratio`, `output_format`,
`output_compression`, plus provider routing settings nested as `provider.<setting>`.

Every key can be overridden per-invocation by an `ORIMAGE_*` environment variable — dots
become underscores, so `provider.sort` reads `ORIMAGE_PROVIDER_SORT`. Precedence, highest
first: command-line flags, environment variables, config file.

## Agent skill

The binary embeds an [agent skill](https://docs.openclaw.ai/tools/skills) that teaches
coding agents how to drive `orimage`. Install it into any skills directory:

```sh
orimage skill install ~/.agents/skills      # personal skills
orimage skill install .agents/skills        # project skills
```

This writes the skill's directory, `orimage/`, into the given path (result:
`<directory>/orimage/SKILL.md`), creating the path if needed. Everything else in
the directory is left alone. If `<directory>/orimage` already exists the install
fails; pass `--force` to delete and rewrite it — e.g. after upgrading `orimage`,
since the skill's file layout can change between versions.

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
