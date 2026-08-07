---
name: orimage
description: Generate or edit images with OpenRouter image models via the orimage CLI, and explore the model catalog, parameters, and pricing.
homepage: https://github.com/schmitthub/openrouter-image-cli
user-invocable: true
command-dispatch: tool
command-tool: orimage
command-arg-mode: raw
metadata:
  {
    "openclaw":
      {
        "emoji": "🖼️",
        "requires": { "bins": ["orimage"], "env": ["OPENROUTER_API_KEY"] },
        "primaryEnv": "OPENROUTER_API_KEY",
      },
  }
---

# orimage

Image generation CLI for the OpenRouter API. Run `orimage --help` to
discover the command surface, and `orimage <command> --help` for flags.
Requires `OPENROUTER_API_KEY`.

When the user asks to generate an image:

```
orimage generate -m google/gemini-2.5-flash-image -p "a red bicycle" -o bike.png
```

When the user asks to edit, restyle, or combine existing images, pass
each source image as a reference — `--input-reference` accepts a local
file path or an HTTP(S) URL and repeats (max 16):

```
orimage generate -m <model-id> -p "make the sky stormy" --input-reference ./photo.png -o edited.png
orimage generate -m <model-id> -p "put the dog from the first image into the scene from the second" \
  --input-reference ./dog.jpg --input-reference https://example.com/park.png -o combined.png
```

When the user asks what models are available, or what a model supports
and costs:

```
orimage models list --json
orimage models info <model-id> --json
```

Check `models info` before passing generation flags beyond `-m`/`-p`/`-o` —
parameter support varies by model and provider.

When the user settles on a model or output preference they keep
repeating, offer to save it as a persisted default so later runs can
omit the flag:

```
orimage config set model google/gemini-2.5-flash-image
```

`orimage config --help` lists the keys; `config list` shows current
values. Explicit flags always override saved defaults, and `ORIMAGE_*`
environment variables sit between the two.

Generated file paths print one per line on stdout; cost goes to stderr.
Failures exit non-zero with the error on stderr.
