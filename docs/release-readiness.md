# Release Readiness

Date: 2026-05-27

Scope: this repository as `github.com/philipbankier/technical-visualizer`.

## Current Release Position

`technical-visualizer` is being prepared for v0.1 as an evidence-first local CLI with optional OpenAI image generation.

The public release promise is:

- URL and local inputs produce source-backed evidence.
- The bundle includes `scaffold.html`, `visual-packet.json`, `manifest.json`, and `final.png`.
- Default generation is local.
- OpenAI image generation is explicit and requires `OPENAI_API_KEY`.
- Codex image generation is documented as an agent handoff workflow, not a direct CLI backend.
- Dense markdown sources produce rich packet fields for metrics, tables, timelines, diagrams, entities, and open questions.
- Codex handoff files are generated locally with `--handoff codex`.
- Quick mode prints a manual command and does not run Codex automatically.
- PDF-only sources can produce useful evidence when local Poppler extraction succeeds.
- Content packs are available with `--pack auto` and write `content-pack.json` plus target briefs.
- `--planner openai` is an optional remote content-pack planning path. It requires `--pack auto` and `OPENAI_API_KEY`, is rejected with `--offline`, and falls back to deterministic planning only for `auto` and `hybrid` backends.

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
- ChatGPT or Codex subscription image tools are documented as an agent handoff path, not a direct CLI backend.
- Generated bundles can contain source-derived content.
- Direct local inputs that look like secrets, credentials, or private keys are rejected.
- PDF files and URLs use local Poppler `pdftotext` when installed. PDF-only runs can succeed when extraction produces enough readable text. Scanned or image-only PDFs still fail with a low-evidence message.
- No PDF content is sent remotely unless the user chooses a remote image backend after packet creation, chooses `--planner openai` for remote pack planning, or a future explicit remote parsing mode is added and selected.
- Local `--pack auto` runs do not claim generated pack images in the manifest.
- The CLI is not ready to expose as a hosted service without private-network URL guards, authentication, and source allowlists.

## Not Released Yet

Do not tag `v0.1.0` until the release branch passes local gates and GitHub Actions.
