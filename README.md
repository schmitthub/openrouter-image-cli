# orgen

A CLI for generating images through the [OpenRouter image generation API](https://openrouter.ai/docs/api/api-reference/images/generate-an-image).

Built for AI coding agents. Agent harnesses (openclaw in particular) don't cover OpenRouter's
dedicated image generation API — their image tools expose only a subset of models, or route
generation through the chat completions API instead. `orgen` gives an agent the full image
API — every model in the catalog, every request parameter — as a plain CLI: deterministic
flags, compact JSON output, meaningful exit codes, images written straight to disk.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/schmitthub/openrouter-generate/main/scripts/install.sh | sh
```

Installs the latest release to `/usr/local/bin` (checksum-verified; linux/darwin,
amd64/arm64). Pin a version with `ORGEN_VERSION=v2026.8.3`, change the target with
`ORGEN_INSTALL_DIR=~/bin`.

Or with Go:

```sh
go install github.com/schmitthub/openrouter-generate/cmd/orgen@latest
```

Or grab a signed archive from [releases](https://github.com/schmitthub/openrouter-generate/releases).
Every release is attested (SLSA build provenance) and its checksums are signed with cosign:

```sh
gh attestation verify orgen_*.tar.gz --owner schmitthub \
  --signer-workflow schmitthub/openrouter-generate/.github/workflows/release-build.yml
```

## Auth

Set `OPENROUTER_API_KEY` — that's it. Commands that don't touch the API (`version`, help) run
without it.

## Usage

Generate an image:

```sh
orgen generate -m google/gemini-2.5-flash-image -p "a red bicycle" -o bike.png
```

Every request option the API supports is a flag:

```sh
orgen generate \
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
orgen models list                        # table: id, name, input modalities, streaming
orgen models list --json                 # full catalog, compact JSON
orgen models info qwen/qwen-image-3      # per-provider parameters + pricing
orgen models info qwen/qwen-image-3 --json
```

`models info` shows exactly which parameters each provider accepts (enum values, ranges) and
what each image costs — check it before generating with an unfamiliar model.

### Provider routing

Models are often served by several providers. Routing flags control which one gets the
request:

```sh
orgen generate -m black-forest-labs/flux.2-pro -p "a dramatic portrait" \
  --provider-sort price \        # price | throughput | latency | exacto
  --provider-order fal,replicate # try these in order; also: --provider-only, --provider-ignore
```

`--no-provider-fallbacks` fails with the upstream error instead of falling back to the next
eligible provider.

Provider-specific parameters that OpenRouter merely passes through go via
`--provider-option <slug>.<key>=<value>` (repeatable). Don't invent keys: each endpoint
advertises what it accepts as `allowed_passthrough_parameters` in `models info`:

```sh
orgen models info black-forest-labs/flux.2-pro --json \
  | jq '.endpoints[] | {slug: .provider_slug, passthrough: .allowed_passthrough_parameters}'
orgen generate -m black-forest-labs/flux.2-pro -p "a dramatic portrait" \
  --provider-option black-forest-labs.steps=40 \
  --provider-option black-forest-labs.guidance=3
```

Values that parse as numbers or booleans are sent typed. When an option names a provider or
key the model's endpoints don't advertise, `orgen` warns on stderr — the API silently drops
unrecognized keys — but never blocks the request; the check is skipped if the endpoint
lookup fails.

## Configuration

Persist defaults so you can stop repeating flags:

```sh
orgen config set model google/gemini-2.5-flash-image
orgen config set aspect_ratio 16:9
orgen config list                        # effective values, key=value per line
orgen config path                        # where config.yaml lives
```

Settings live in `config.yaml` under your OS config directory (`$XDG_CONFIG_HOME/orgen` on
Linux), or `$ORGEN_CONFIG_DIR` when set. Keys: `model`, `aspect_ratio`, `output_format`,
`output_compression`, plus provider routing settings nested as `provider.<setting>`:
`provider.sort`, `provider.order`, `provider.only`, `provider.ignore`,
`provider.allow_fallbacks`, and passthrough defaults as `provider.options.<slug>.<key>`.
Slug lists are comma-separated (`orgen config set provider.order fal,replicate`); in the
YAML file a sequence works too. One caveat: config keys are case-insensitive (stored
lowercased), so a case-sensitive passthrough key like Google's `cachedContent` can't be
persisted — pass it with `--provider-option`, which preserves case.

Every key can be overridden per-invocation by an `ORGEN_*` environment variable — dots
become underscores, so `provider.sort` reads `ORGEN_PROVIDER_SORT`. Precedence, highest
first: command-line flags, environment variables, config file.

## Agent skill

The binary embeds an [agent skill](https://docs.openclaw.ai/tools/skills) that teaches
coding agents how to drive `orgen`. Install it into any skills directory:

```sh
orgen skill install ~/.agents/skills      # personal skills
orgen skill install .agents/skills        # project skills
```

This writes the skill's directory, `orgen/`, into the given path (result:
`<directory>/orgen/SKILL.md`), creating the path if needed. Everything else in
the directory is left alone. If `<directory>/orgen` already exists the install
fails; pass `--force` to delete and rewrite it — e.g. after upgrading `orgen`,
since the skill's file layout can change between versions.

## Output

- `generate` prints one `✓ <path>` line per image to stdout; cost goes to stderr, so stdout
  stays parseable.
- `--out` names the file; multiple images insert an index before the extension. Without
  `--out`, files are named `orgen-<timestamp>.<ext>`. The extension follows the response's
  media type.
- Errors from the API surface with status and message (e.g. `openrouter: HTTP 402:
  Insufficient credits`); the exit code is non-zero on any failure.

## License

MIT
