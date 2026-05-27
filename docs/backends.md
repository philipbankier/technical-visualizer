# Backends

`technical-visualizer` separates source gathering from image generation. Source gathering creates the evidence bundle, packet, scaffold, and manifest. The selected backend decides how `final.png` is written.

`--renderer html` is scaffold-first mode. It always writes the local fallback `final.png`, records the selected backend as `local`, and does not call remote image generation. Use `--renderer image` or the default `hybrid` renderer when you want the selected backend to generate `final.png`.

## `local`

`local` never calls a remote image provider. It writes `scaffold.html`, `visual-packet.json`, `manifest.json`, and a deterministic fallback `final.png` preview.

Remote URL inputs may still be fetched unless `--offline` is set.

PDF text extraction is also local. When Poppler `pdftotext` is installed, PDF files and fetched PDF URLs can produce `pdf_text` evidence. PDF-only runs can succeed when the extracted text has enough readable content. Scanned or image-only PDFs still fail with a low-evidence message instead of creating weak evidence.

## `openai`

`openai` calls the OpenAI Images API with `gpt-image-2` when the renderer is `image` or `hybrid`. It requires `OPENAI_API_KEY`. It sends source-derived prompt content and scaffold HTML to OpenAI, writes the returned image to `final.png`, and fails if the remote generation call fails.

Use this backend only when remote processing is acceptable for the source material.

The current PDF parser does not send PDF bytes or extracted PDF content to a remote parsing service. Source-derived packet content can still be sent to OpenAI when this backend generates `final.png`.

## `auto`

`auto` uses OpenAI when `OPENAI_API_KEY` is set and the renderer is `image` or `hybrid`. If OpenAI is unavailable or generation fails, it writes the local fallback preview and records a manifest warning.

Without `OPENAI_API_KEY`, `auto` stays local.

## `hybrid`

`hybrid` follows the same fallback behavior as `auto`: it tries OpenAI when credentials exist and the renderer is `image` or `hybrid`, then falls back to the local preview with a warning if OpenAI cannot produce an image.

## Codex Agent Handoff

Codex is not a direct backend in v0.1. `--handoff codex` is not a backend. It writes local prompt and brief files that an interactive Codex session can use.

```bash
visualize --backend local --renderer html --handoff codex --quick --out visualize-output https://github.com/example/service
```

The generated bundle includes `handoff/codex-prompt.md`, `handoff/image-brief.md`, `handoff/qa-checklist.md`, and `handoff/style.md`. Quick mode prints a POSIX shell command for manual use in interactive Codex.

Supported Codex environments may provide built-in image generation without `OPENAI_API_KEY`, but that is an agent workflow. The CLI does not support `visualize --backend codex` in v0.1.

## Offline

`--offline` skips remote source fetching and remote image generation. `--backend openai --offline` is rejected because it asks for mutually exclusive behavior.

`visualize doctor` reports Poppler as `Poppler pdftotext: available (local, version...)` when PDF extraction is available, or `Poppler pdftotext: unavailable (install Poppler for PDF-only sources)` when it is not.
