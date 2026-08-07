---
name: orgen
description: Generate or edit images with OpenRouter image models via the orgen CLI, and explore the model catalog, parameters, and pricing.
homepage: https://github.com/schmitthub/openrouter-generate
user-invocable: true
command-dispatch: tool
command-tool: orgen
command-arg-mode: raw
metadata:
  {
    "openclaw":
      {
        "emoji": "🖼️",
        "requires": { "bins": ["orgen"], "env": ["OPENROUTER_API_KEY"] },
        "primaryEnv": "OPENROUTER_API_KEY",
      },
  }
---

# orgen

Image generation CLI for the OpenRouter API. Run `orgen --help` to
discover the command surface, and `orgen <command> --help` for flags.
Requires `OPENROUTER_API_KEY`.

When the user asks to generate an image:

```
orgen generate -m google/gemini-2.5-flash-image -p "a red bicycle" -o bike.png
```

When the user asks to edit, restyle, or combine existing images, pass
each source image as a reference — `--input-reference` accepts a local
file path or an HTTP(S) URL and repeats (max 16):

```
orgen generate -m <model-id> -p "make the sky stormy" --input-reference ./photo.png -o edited.png
orgen generate -m <model-id> -p "put the dog from the first image into the scene from the second" \
  --input-reference ./dog.jpg --input-reference https://example.com/park.png -o combined.png
```

Reference order matters. The OpenRouter API contract does not define
ordering semantics (behavior is model-specific), but models see the
references in array order and bind ordinal phrases in the prompt to
that order — "the first image" is the first `--input-reference`, "the
second image" the next. Verified empirically (gemini-2.5-flash-image):
with the prompt "place the subject from the first image into the scene
from the second image", passing subject-then-scene composited
correctly, while the swapped order made the model treat the scene as
the subject — and in one run return no image at all.

So: refer to references by position in the prompt, and pass them in
exactly that order. If a multi-reference edit ignores or mishandles an
image — or the model returns no image — check prompt ordinals against
reference order and swap the references before rewriting the prompt.

When the user asks what models are available, or what a model supports
and costs:

```
orgen models list --json
orgen models info <model-id> --json
```

Check `models info` before passing generation flags beyond `-m`/`-p`/`-o` —
parameter support varies by model and provider.

When the user cares about which provider serves the request (cost,
speed, a specific vendor), use the provider routing flags — see
`orgen generate --help` for the full set. These map to OpenRouter's
documented `provider` request object, so they behave the same for
every model.

Provider-specific tuning knobs (steps, guidance, and similar) are
passthrough parameters, discovered per endpoint rather than defined by
the API contract. Never guess them: read each endpoint's
`allowed_passthrough_parameters` first, then pass exactly those keys —
the API silently drops keys a provider doesn't advertise, and orgen
warns on stderr when that would happen:

```
orgen models info <model-id> --json   # note provider_slug + allowed_passthrough_parameters
orgen generate -m <model-id> -p "..." --provider-option <slug>.<key>=<value>
```

Keys are case-sensitive on the wire; `--provider-option` preserves
case, but persisted config (`provider.options.<slug>.<key>`) lowercases
keys, so camelCase keys must go on the command line.

When the user settles on a model or output preference they keep
repeating, offer to save it as a persisted default so later runs can
omit the flag:

```
orgen config set model google/gemini-2.5-flash-image
```

`orgen config list` shows every key with its current value; see
`orgen config --help` for the full command surface. Explicit flags
always override saved defaults, and `ORGEN_*` environment variables
sit between the two.

Generated file paths print one per line on stdout; cost goes to stderr.
Failures exit non-zero with the error on stderr.

## Upgrading

Suspect a stale binary when the API rejects something this skill or
`--help` documents, or a documented flag is unknown to the binary.
Confirm before upgrading: compare the installed version against the
latest release —

```
orgen version
gh release view --repo schmitthub/openrouter-generate --json tagName -q .tagName
```

(without `gh`: `curl -fsSL https://api.github.com/repos/schmitthub/openrouter-generate/releases/latest | grep tag_name`).

If they differ, upgrade in place. The install script downloads the
latest release binary and installs to `/usr/local/bin` by default
(falling back to `~/.local/bin` when not writable); `ORGEN_INSTALL_DIR`
overrides the target. To avoid ending up with two copies shadowing each
other on PATH, install over the existing binary's directory:

```
ORGEN_INSTALL_DIR="$(dirname "$(command -v orgen)")" \
  sh -c 'curl -fsSL https://raw.githubusercontent.com/schmitthub/openrouter-generate/main/scripts/install.sh | sh'
```

Then verify: `orgen version` must now report the new version. If it
does not, `command -v orgen` is resolving a different copy — remove or
upgrade that one too.

Finally, this skill file itself is embedded in the binary and installed
from it, so an upgraded binary may carry a newer skill. Refresh the
installed copy by re-running skill install against the skills directory
that contains this file (the parent of this `orgen/` directory);
`--force` replaces the existing `orgen/` skill directory:

```
orgen skill install --force <skills-directory>
```
