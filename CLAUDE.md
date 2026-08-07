# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`orimage` — a Go CLI that calls the OpenRouter API to generate images and writes them to disk.
Module path `github.com/schmitthub/openrouter-image-cli`, binary `orimage`.

Working commands: `generate` (image generation), `models list` / `models info`
(model catalog), `config get/set/list/path` (persisted generation defaults —
see `internal/config`), `skill install` (write the embedded agent skill — see
`internal/skill` — into a skills directory), `version`.

## Commands

```sh
make build            # -> bin/orimage, ldflags-stamped version/revision
make test             # go test ./... (uses gotestsum --format testdox if installed)
make test-verbose
make cover            # coverage.out + per-func summary
make lint             # golangci-lint run --config .golangci.yml
make fmt              # golangci-lint fmt (goimports, gofumpt, golines @ 120)
make tidy
make test-ci          # CI mode: -race -count=1 -coverprofile=coverage.out
make pre-commit       # run all pre-commit hooks (mirrors CI gates)
make release-check    # goreleaser snapshot dry-run into dist/ (no tag/sign/publish)
make release VERSION=v0.1.0 MESSAGE="..."   # tag + push -> triggers release workflow
```

Single test: `go test ./internal/query -run TestName -v`

`bin/` is on `PATH` via `.envrc` (direnv), so `orimage` resolves after `make build`.
`.envrc` also does `dotenv` — `.env` holds `OPENROUTER_API_KEY` and is gitignored.

## Architecture

Layered after the `gh` CLI's structure. Flow:

```
cmd/orimage/orimage.go        main(), exits with the code from …
internal/orimagecmd/cmd.go    Main(): builds IOStreams -> Factory -> root cmd, executes,
                              maps errors to exit codes, prints usage on *cmdutil.FlagError
internal/cmd/factory          constructs the Factory (dependency container)
internal/cmd/root             assembles the cobra tree; the only place subcommands register
internal/cmd/<name>           one package per subcommand
```

**Factory injection.** `cmdutil.Factory` carries `AppVersion`, `IOStreams`, the deferred
`OpenRouter` client constructor, and the lazy `Config` loader, and is threaded
into every `NewCmdX(f *cmdutil.Factory, runF func(*XOptions) error)` constructor. Add new
dependencies (e.g. an OpenRouter client constructor) as fields on `Factory`, populate them in
`internal/cmd/factory.New`, and copy them onto the command's `Options` struct in the constructor.

**The `runF` seam.** Every command constructor takes a `runF` override. When non-nil the command
calls it instead of the real `runX(opts)`. Tests pass a `runF` that captures `opts`, so flag
parsing/validation is testable without executing the command. Production callers pass `nil`.
Keep this shape for new commands.

**Error handling.** Wrap flag/arg errors with `cmdutil.FlagErrorf` / `FlagErrorWrap` — only those
(plus `unknown command` / `required flag(s)`) cause the root to print the usage string.
`root.rootFlagErrorFunc` does this automatically for cobra's own flag parse errors.
`forbidigo` bans `panic`; return errors up the stack.

**`internal/iostreams`** — copied from `gh`. TTY detection, color/truecolor support, pager,
spinner, `Test()` returning `(ios, stdin, stdout, stderr)` buffers. Commands must write through
`opts.IOStreams.Out` / `.ErrOut`, never `os.Stdout` directly, or they become untestable.

**`internal/query`** — generic read-only ORM over slices of json-tagged structs. Fields are
addressed by dotted json-tag paths (`"repo.owner.id"`), validated against the struct type *and*
against the set of keys actually present in the decoded source, so a typo'd path errors instead
of silently matching zero values. `Set[T]` -> `.Query()` -> chained `.Where(path, op, value)`
(immutable, returns a new `Query`) -> terminal `All`/`Count`/`CountBy`/`CountByNested`.
Intended as the query engine for OpenRouter JSON responses; currently unused by any command.

**`internal/openrouter`** — hand-rolled OpenRouter API client (no SDK dep). `New(key, opts...)`
/ `NewFromEnv()` (reads `OPENROUTER_API_KEY`); shared `doJSON`/`getJSON` helpers in `http.go`
handle auth/attribution headers and error parsing; non-2xx becomes `*APIError`
(status/code/message). Endpoints: `GenerateImage` (POST `/images`, base64 payloads),
`ListImageModels` (GET `/images/models`), `GetImageModelEndpoints`
(GET `/images/models/{id}/endpoints` — per-provider params + pricing; there is no bare
`/images/models/{id}` resource). `WithBaseURL` exists for httptest servers. The Factory
exposes the client as a deferred `OpenRouter func() (*openrouter.Client, error)` so keyless
commands (version, help) still run.

**`internal/config`** — viper-backed persisted settings behind a `Config` interface
(`New()` returns the interface; commands mock it in tests). Keys: `model`,
`aspect_ratio`, `output_format`, `output_compression`, `provider.<setting>`. File:
`config.yaml` under `$ORIMAGE_CONFIG_DIR` or `<os.UserConfigDir>/orimage` (XDG).
Precedence: flags > `ORIMAGE_*` env vars (viper `AutomaticEnv`, dots→underscores) >
file. Two viper instances: `main` (file+env, serves reads) and `file` (file+explicit
`Set`s only) so `Save` never persists env overrides. The Factory exposes it as
`Config func() (config.Config, error)` via `sync.OnceValues` — lazy, memoized, and
keyless commands never touch the filesystem. `generate` fills unset flags from config
in `applyConfigDefaults`; `--model` is enforced in `validateOptions` (not
`MarkFlagRequired`) so a configured default can satisfy it. The package never prints —
errors return to callers.

**`internal/build`** — `Version`/`Date`/`Revision` set via `-ldflags -X`, with a
`debug.ReadBuildInfo()` fallback for `go install` builds.

## CI / release pipeline

Mirrors `schmitthub/clawker` (minus its embed/BPF machinery):

- `pr.yml` (PRs -> main): security + lint + test via reusable workflows.
  `main.yml` (push to main): security + test. `lint.yml` only runs on PRs
  because `new-from-merge-base` needs a merge base with main.
- `security.yml`: semgrep (pinned container image; version must match the
  comment in `.pre-commit-config.yaml`), gitleaks (raw CLI, no baseline),
  govulncheck (manual install — the official action clobbers the checkout),
  dependency-review (PRs only).
- `release.yml` (on `v*` tags): validates semver + tag-on-main + a green Main
  run for the tagged SHA, then calls `release-build.yml` (goreleaser + cosign
  keyless sign of checksums.txt + SLSA attestation of archives and binaries).
  `release-build.yml` is the SLSA-anchored identity — verify with
  `gh attestation verify --signer-workflow schmitthub/openrouter-image-cli/.github/workflows/release-build.yml`.
- Cut a release with `make release VERSION=vX.Y.Z MESSAGE="..."` — it enforces
  clean tree / on-main / synced / CI-green locally before tagging.
- GitHub rulesets (created by `scripts/setup-repo-rulesets.sh`, idempotent):
  `protect-main` (PRs + required status checks; repo admins bypass) and
  `protect-release-tags` (only admins create `v*` tags; tags immutable).
  The required-check names in that script must track the job names in `pr.yml`.
- Pre-commit hooks (`bash scripts/install-hooks.sh`) mirror the CI gates:
  gitleaks, semgrep (Go + workflow YAML), go-mod-tidy, golangci-lint,
  govulncheck (via `scripts/govulncheck.sh`, pinned to go.mod's toolchain),
  and `make test`.

## Conventions

- `golangci-lint` runs with `default: all` (see `.golangci.yml` for the disable list).
  `new-from-merge-base: main` means only issues in the diff against `main` are reported.
  The inherited scaffold code (`internal/iostreams`, `internal/query`) carries ~87 pre-existing
  findings that are baselined once committed — do not treat a clean `make lint` on a fresh diff
  as evidence those packages are clean.
- `exhaustruct` is on: struct literals must set every field, except for the types allowlisted in
  `.golangci.yml` (`cobra.Command` is allowlisted).
- `tagliatelle` enforces snake_case json/yaml/toml tags.
- `funlen` 60 lines / 40 statements; `cyclop` max complexity 15; `golines` wraps at 120.
- Test files are exempt from `errcheck`, `funlen`, `wrapcheck`, `gosec`, `dupl`, `goconst`,
  `noctx`, `testpackage`.
- `wrapcheck` is on: wrap errors crossing package boundaries with `fmt.Errorf("...: %w", err)`.

## Open work

- Streaming (`stream: true`, SSE partial images) is not implemented in
  `internal/openrouter`; neither is the `provider` routing object.
- `models list/info --json` emit compact single-line JSON (machine-oriented).
- Homebrew tap publishing is stubbed out in `.goreleaser.yaml` (commented `homebrew_casks`
  block) — needs a `schmitthub/homebrew-tap` repo and a `HOMEBREW_TAP_GITHUB_TOKEN` secret
  wired through `release.yml` -> `release-build.yml` before enabling.

## Environment

Development happens inside a clawker container with an egress firewall. Outbound hosts are
allowlisted in `.clawker.yaml` under `security.firewall` — `openrouter.ai` permits only
`/docs` and `/api` paths (`path_default: deny`). Machine-local additions go in
`.clawker.local.yaml` (gitignored); rules useful to contributors belong in `.clawker.yaml`.
Firewall edits require `clawker firewall refresh` on the host to take effect.
Surface any blocked host/path to the user rather than routing around it.
