# technical-visualizer

`technical-visualizer` is a Go CLI for turning source-backed technical context into a visualization bundle.

URL inputs are the primary path. Pass a GitHub repo URL, docs page URL, Markdown URL, or PDF URL when you want the tool to fetch source context for you. Local paths work for private repos, offline work, and tests.

The CLI writes:

- `scaffold.html`: audit-friendly HTML scaffold
- `visual-packet.json`: renderer-facing source packet
- `final.png`: image output, or a deterministic local fallback PNG
- `manifest.json`: source, backend, renderer, warnings, and output hashes

## Install

```bash
go install github.com/philipbankier/technical-visualizer/cmd/visualize@latest
```

From a checkout:

```bash
go build -trimpath -o /tmp/visualize ./cmd/visualize
/tmp/visualize --version
```

## Usage

```bash
visualize --out visualize-output https://github.com/example/service
```

Local, no remote image call:

```bash
visualize --backend local --renderer html --out /tmp/visualize-smoke ./testdata/sample-repo ./testdata/notes.md
```

Offline, no remote source fetch and no remote image call:

```bash
visualize --offline --out visualize-output ./docs/architecture.md
```

OpenAI image backend:

```bash
OPENAI_API_KEY=... visualize --backend openai --renderer image --out visualize-output https://github.com/example/service
```

Backend status:

```bash
visualize doctor
```

## Inputs

Supported v1 inputs:

- GitHub repo URLs
- Docs site URLs
- Markdown files and URLs
- JSON visual packet files
- PDF files and URLs, with bounded text extraction warnings
- Local repo paths

Direct local inputs that look like secrets, credentials, or private keys are rejected. Repo scans skip secret-like files, generated directories, symlinks, large files, and binary-looking files by default.

## Backends

`local` is the default. It writes a deterministic fallback PNG and never calls a remote image service.

`openai` calls the OpenAI Images API with `gpt-image-2`. It requires `OPENAI_API_KEY` and writes the returned PNG to `final.png`. The default image size is `1536x1024`, the supported landscape size for GPT image generation.

`auto` and `hybrid` try OpenAI when credentials are available, then fall back to local output. Use them only when remote generation is acceptable.

Codex is doctor-only for generation in v1. `visualize doctor` checks whether the local Codex CLI is available, but `--backend codex` is rejected until a documented image-generation CLI or tool bridge is proven.

## Privacy

The default backend is local so source-derived content is not uploaded just because `OPENAI_API_KEY` is present.

Use `--offline` to avoid remote source fetching and remote image generation. Explicit `--backend openai --offline` is rejected before bundle files are written.

Secrets are read from environment variables. Do not place API keys in source files.

Generated bundles and cache directories are created with private permissions by default because they can contain source-derived content.

## Development

Use Go 1.26.3 or newer for release checks.

```bash
script/lint
script/test
script/smoke
script/security
```

Release gates:

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test -race -count=1 ./...
go build -trimpath -o /tmp/visualize ./cmd/visualize
/tmp/visualize doctor
rm -rf /tmp/visualize-smoke
/tmp/visualize --backend local --renderer html --offline --out /tmp/visualize-smoke ./testdata/sample-repo ./testdata/notes.md
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

The current release unit is this directory as a standalone repository root. Do not publish the wider parent workspace as this tool's GitHub repo.
