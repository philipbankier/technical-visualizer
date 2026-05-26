# technical-visualizer

`technical-visualizer` is a Go CLI that turns URLs or local technical sources into an auditable visualization bundle.

The v0.1 release is evidence-first. It gathers bounded, source-backed context, writes `visual-packet.json`, builds an inspectable `scaffold.html`, records the run in `manifest.json`, and writes `final.png` as either an OpenAI-generated image or a local fallback preview.

## Quickstart

```bash
go install github.com/philipbankier/technical-visualizer/cmd/visualize@latest
visualize --out visualize-output https://github.com/philipbankier/technical-visualizer
```

Inspect `visualize-output/scaffold.html` first. It is the auditable local visualization scaffold. Then check `visualize-output/manifest.json` for sources, backend choice, warnings, audit metadata, and output hashes.

Run backend checks with:

```bash
visualize doctor
```

## Local And Offline

Use local mode when source content should stay on the machine:

```bash
visualize --backend local --renderer html --offline --out /tmp/visualize-smoke ./testdata/sample-repo ./testdata/notes.md
```

Offline mode skips remote source fetching and remote image generation. It only works with local inputs.

## OpenAI Image Generation

```bash
OPENAI_API_KEY=... visualize --backend openai --renderer image --out visualize-output https://github.com/example/service
```

`--backend openai` requires `OPENAI_API_KEY`. It calls the OpenAI Images API with `gpt-image-2`, sends source-derived visual packet content and scaffold HTML to OpenAI, and writes the returned image to `final.png`. Use it only for content that can be processed remotely.

If OpenAI generation fails, explicit `--backend openai` fails. The `auto` and `hybrid` backends can fall back to a local preview and record a manifest warning.

## Codex And ChatGPT Subscriptions

ChatGPT and Codex subscription access is separate from OpenAI API credentials.

- `--backend openai` is the direct Go CLI API path and requires `OPENAI_API_KEY`.
- A ChatGPT or Codex subscription is separate from API billing and does not cover OpenAI API image calls made by this CLI.
- Codex can be used as an agent around this tool.
- In supported Codex environments, Codex may have built-in image generation without `OPENAI_API_KEY`.
- v0.1 does not support `visualize --backend codex`.

## Codex Handoff Package

Use `--handoff codex` when you want the CLI to prepare an agent-ready package:

```bash
visualize --backend local --renderer html --handoff codex --out visualize-output ./research-knowledge-base.md
```

This writes `handoff/codex-prompt.md`, `handoff/image-brief.md`, `handoff/qa-checklist.md`, and `handoff/style.md`.

For a fast manual handoff, add `--quick`:

```bash
visualize --backend local --renderer html --handoff codex --quick --out visualize-output ./research-knowledge-base.md
```

Quick mode prints a POSIX shell command you can run in interactive Codex. The CLI does not run Codex for you and does not support `--backend codex`.

Use `--backend openai` when you want the Go binary itself to make the image API call. Use `--handoff codex` when you want an agent to inspect the bundle and use tools available in that agent environment.

## Output Files

- `scaffold.html`: auditable local visualization scaffold
- `visual-packet.json`: source-backed renderer packet
- `manifest.json`: sources, backend, warnings, audit metadata, and output hashes
- `final.png`: OpenAI image output or deterministic local fallback preview
- `handoff/`: optional Codex prompt, image brief, QA checklist, and style notes from `--handoff codex`

In v0.1, `scaffold.html` is the main local artifact to inspect. Local `final.png` is a deterministic preview, not a finished design renderer.

## Backends

`--renderer html` is scaffold-first mode. It always writes the local fallback `final.png`, records the selected backend as `local`, and does not call remote image generation. Use `--renderer image` or the default `hybrid` renderer when you want the selected backend to generate `final.png`.

| Backend  | Remote source fetch      | Remote image call                                               | Behavior                                              |
| -------- | ------------------------ | --------------------------------------------------------------- | ----------------------------------------------------- |
| `local`  | yes, unless `--offline`  | no                                                              | writes scaffold and local fallback preview            |
| `openai` | yes, unless local input  | yes, when renderer is `image` or `hybrid`                       | calls OpenAI Images API and fails if generation fails  |
| `auto`   | yes, unless `--offline`  | yes, when credentials exist and renderer is `image` or `hybrid` | tries OpenAI, falls back locally with warning          |
| `hybrid` | yes, unless `--offline`  | yes, when credentials exist and renderer is `image` or `hybrid` | tries OpenAI, falls back locally with warning          |

Codex handoff is separate from backend selection. Use `--handoff codex` to write an optional local handoff package for an interactive agent.

See [docs/backends.md](docs/backends.md) for backend details.

## Inputs

Supported v0.1 inputs:

- GitHub repo URLs
- Docs site URLs
- Markdown files and URLs
- JSON files as evidence inputs
- PDF files and URLs, but only alongside at least one text, repo, JSON, or docs source. v0.1 emits warnings and no PDF text evidence, so PDF-only runs fail with no evidence gathered.
- Local repo paths

Direct local inputs that look like secrets, credentials, or private keys are rejected. Repo scans skip secret-like files, generated directories, symlinks, large files, and binary-looking files by default.

## Privacy

The default backend is `local`. Source-derived content is not uploaded just because `OPENAI_API_KEY` is present.

Use `--offline` to avoid remote source fetching and remote image generation. Explicit `--backend openai --offline` is rejected before bundle files are written.

Generated bundles and cache directories use private permissions by default because they can contain source-derived content. Secrets are read from environment variables. Do not place API keys in source files.

The CLI is not ready to expose as a hosted service without private-network URL guards, authentication, and source allowlists.

## Development

Use Go 1.26.3 or newer for release checks.

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
```

The current release unit is this directory as a standalone repository root. Do not publish the wider parent workspace as this tool's GitHub repo.
