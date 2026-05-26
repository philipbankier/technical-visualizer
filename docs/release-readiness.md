# Release Readiness

Date: 2026-05-26

Scope: this repository as `github.com/philipbankier/technical-visualizer`.

## Current Release Position

`technical-visualizer` is being prepared for v0.1 as an evidence-first local CLI with optional OpenAI image generation.

The public release promise is:

- URL and local inputs produce source-backed evidence.
- The bundle includes `scaffold.html`, `visual-packet.json`, `manifest.json`, and `final.png`.
- Default generation is local.
- OpenAI image generation is explicit and requires `OPENAI_API_KEY`.
- Codex image generation is documented as an agent handoff workflow, not a direct CLI backend.

## Release Gates

Run from the repository root:

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
```

## Public Safety Claims

- Default backend is `local`.
- Explicit OpenAI generation requires `OPENAI_API_KEY`.
- Generated bundles can contain source-derived content.
- Direct local inputs that look like secrets, credentials, or private keys are rejected.
- PDF files and URLs can be included alongside at least one text, repo, JSON, or docs source. v0.1 emits warnings and no PDF text evidence, so PDF-only runs fail with no evidence gathered.
- The CLI is not ready to expose as a hosted service without private-network URL guards, authentication, and source allowlists.

## Not Released Yet

Do not tag `v0.1.0` until the release branch passes local gates and GitHub Actions.
