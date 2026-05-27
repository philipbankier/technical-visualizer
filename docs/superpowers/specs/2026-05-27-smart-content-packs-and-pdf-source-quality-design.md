# Smart Content Packs And PDF Source Quality Design

Date: 2026-05-27

Status: approved for implementation planning.

Branch: `codex/content-rich-codex-handoff`

## Context

`technical-visualizer` currently turns URLs or local technical sources into an auditable bundle:

- `visual-packet.json`
- `scaffold.html`
- `manifest.json`
- `final.png`
- optional `handoff/` files for Codex

The current bundle is intentionally evidence-first. The previous content-rich handoff work made dense markdown sources much more useful for Codex and image-generation workflows, but two related gaps remain:

- Users want more than one image. A dense LinkedIn-style technical infographic is useful, but the same source should also be able to produce lower-density social images, blog/README heroes, and other adjacent content.
- PDF-only inputs still fail when the current PDF gatherers emit warning-only evidence. This is now a product problem because papers and PDFs are common source material for the content packs users want.

The design should keep the current local-first, source-backed model. It should not turn Codex into a backend, bundle heavy native dependencies, or replace the simple pipeline with a parallel architecture.

## Goals

- Add a first-class content pack concept: one source-backed story spine, multiple target image briefs and outputs.
- Preserve `visual-packet.json` as the contract between source gathering and downstream rendering.
- Fix PDF source quality enough that PDF-only runs can produce useful packets when local extraction succeeds.
- Make the smooth path one command when a capable backend exists, while preserving a complete handoff path for Codex/subscription users.
- Keep implementation phased and testable.

## Non-Goals

- Do not add `--backend codex`.
- Do not auto-run Codex from the Go CLI.
- Do not bundle Poppler, MuPDF, or PDF Oxide native libraries.
- Do not make PDF Oxide the default extractor in the first implementation.
- Do not use remote LLM/PDF parsing unless the user explicitly requests a remote mode.
- Do not pretend local placeholder previews are polished final images.
- Do not make each pack target reopen or reinterpret original sources independently.

## Recommended Product Model

A content pack is a first-class optional bundle artifact. It is not just several unrelated prompts.

The pack starts from one shared source-backed story spine derived from `visual-packet.json`. Each pack target then carries its own platform, intent, density, aspect ratio, output path, and brief.

Default `--pack auto` should feel smart, but inspectable:

```bash
visualize --pack auto --out out https://github.com/example/project
```

The generated `content-pack.json` should explain what targets were chosen and why. In the first implementation, the target set can be deterministic. Later, an LLM planner can generate or revise the same schema.

Target state is per target:

- `planned`: plan and brief exist.
- `handoff_ready`: Codex or agent prompt exists.
- `generated`: image file exists.
- `verified`: image file exists and passes basic checks.
- `failed`: target failed, with reason.

Overall pack status is derived from targets:

- `planned`: all targets are planned only.
- `handoff-ready`: at least one target has handoff files and no target is generated.
- `partial`: some targets generated or verified and others failed or remain handoff-ready.
- `complete`: all targets are generated or verified.

## Initial Pack Targets

Start with a small fixed set before allowing custom target definitions:

| Target           | Intent                                | Density | Suggested aspect |
| ---------------- | ------------------------------------- | ------- | ---------------- |
| `linkedin-dense` | Technical deep-dive infographic       | high    | 16:9 or 1.91:1   |
| `social-teaser`  | Pretty, lower-density social preview  | low     | 1:1              |
| `blog-og`        | Clear article/README/open-graph hero  | medium  | 1.91:1           |

This keeps the first release useful without making platform support the product. Later versions can add `story-cover`, `carousel-cover`, or user-defined targets after the schema is proven.

## PDF Source Quality

PDF extraction belongs upstream of packet building. A successful PDF extractor should emit normal evidence items with page-aware refs so the existing rich extractor can build useful packets.

The first implementation should keep PDF support inside `internal/source`, likely in a new `pdf.go` file, rather than creating an `internal/source/pdf` subpackage. Current PDF stubs already live in `internal/source/local.go` and `internal/source/web.go`, and keeping helpers package-local avoids exporting safety and redaction internals too early.

Recommended extractor policy:

1. Prefer Poppler `pdftotext` when it is available on the user machine.
2. Use a conservative pure-Go fallback only if fixture testing proves it adds real value.
3. Keep PDF Oxide behind an explicit experimental engine flag until its Go install story and stability are better proven.
4. Use `pdfcpu` only for adjacent PDF diagnostics if needed, not as the primary text extractor.
5. Exclude MuPDF/go-fitz/mutool from the default path because of AGPL/commercial licensing risk.
6. Exclude UniPDF from OSS core because of commercial license-key friction.
7. Keep remote LLM parsing explicit and remote-marked.

PDF extraction output should include:

- extractor engine and version
- page count when known
- extracted page count
- truncation status
- confidence or quality warnings
- source refs like `paper.pdf#page=7`
- Markdown-ish text that the current extractor can parse

Plain text post-processing is required. Raw PDF text should be normalized enough to become useful evidence:

- infer headings from common paper patterns such as `Abstract`, `1 Introduction`, `2.3 Method`, `References`, and appendix markers
- dehyphenate line breaks
- remove repeated page headers and footers where deterministic
- preserve page boundaries
- preserve table-like blocks when possible
- avoid promoting OCR or weak extraction into high-confidence facts

## Architecture

Keep the current pipeline shape and add optional stages:

```text
sources
  -> internal/source, including PDF extraction in source/pdf.go
  -> visual-packet.json
  -> optional internal/pack stage
  -> content-pack.json and target briefs
  -> optional handoff files or generated images
  -> manifest.json and quality validation
```

Do not create a second architecture beside the existing pipeline.

Package boundaries:

- `internal/source/pdf.go`: PDF extraction, engine selection, page-aware evidence, extraction warnings.
- `internal/pack`: content pack schema, fixed target defaults, deterministic pack planning, target briefs, target output metadata.
- `internal/handoff`: Codex-specific pack handoff files and safe private writes.
- `internal/pipeline`: orchestrates optional pack stage, rollback, manifest entries, and quality validation.

Avoid for the first implementation:

- `internal/source/pdf` subpackage
- `internal/extract/normalize` package
- `internal/render/pack` package

Those boundaries can be introduced later only if concrete implementation pressure appears.

## Content Pack Schema

The pack schema should be separate from `VisualPacket`. `VisualPacket` remains one source-backed rendering packet. `content-pack.json` wraps that packet into multiple targets.

Example shape:

```json
{
  "schema_version": "content-pack/v1",
  "source_packet": "visual-packet.json",
  "title": "Technical Map: example/project",
  "status": "handoff-ready",
  "strategy": {
    "planner": "deterministic",
    "summary": "High-density technical source with architecture and docs evidence."
  },
  "targets": [
    {
      "id": "linkedin-dense",
      "platform": "linkedin",
      "intent": "technical deep-dive infographic",
      "density": "high",
      "aspect_ratio": "16:9",
      "brief_path": "pack/linkedin-dense/brief.md",
      "output_path": "pack/linkedin-dense/final.png",
      "state": "handoff_ready",
      "required_content": [
        "title",
        "top claims",
        "architecture",
        "risks"
      ],
      "avoid": [
        "inventing unsupported metrics",
        "using unreadable tiny text"
      ]
    }
  ]
}
```

Target output files should appear in `manifest.output_files` only when they actually exist. Planned outputs belong in `content-pack.json`, not in `manifest.output_files`.

## User Experience

Direct OpenAI generation should be the smoothest complete path:

```bash
visualize --pack auto --backend openai --out out https://github.com/example/project
```

Expected outputs when generation succeeds:

```text
out/
  visual-packet.json
  scaffold.html
  manifest.json
  final.png
  content-pack.json
  pack/
    linkedin-dense/
      brief.md
      final.png
    social-teaser/
      brief.md
      final.png
    blog-og/
      brief.md
      final.png
```

Codex handoff should be complete but manual:

```bash
visualize --pack auto --handoff codex --quick --out out https://github.com/example/project
```

Expected behavior:

- write `content-pack.json`
- write per-target briefs
- write a pack-level Codex prompt
- print a manual POSIX shell command
- keep `--backend codex` rejected
- mark targets as `handoff_ready`, not `generated`

Local/offline mode should be useful and honest:

```bash
visualize --pack auto --backend local --offline --out out ./source
```

Expected behavior:

- write `content-pack.json`
- write target briefs
- write local previews or a contact sheet only if clearly labeled as previews
- mark targets as `planned`
- include next steps for OpenAI generation or Codex handoff

## Error Handling

PDF failure behavior:

- If a PDF extractor succeeds, PDF-only runs should proceed.
- If no extractor is available, fail PDF-only runs with a precise message and installation guidance.
- If extraction yields too little text, fail PDF-only runs with a low-evidence message.
- If mixed sources include a failed PDF and other good evidence, continue with warnings.
- Record extractor, version, pages attempted, pages extracted, truncation, and warnings in the manifest.

Pack failure behavior:

- A partial pack is allowed.
- A failed target should not delete successfully generated sibling targets.
- Failed targets must include a reason.
- Re-running should clean up generated pack artifacts safely and preserve unrelated custom files.
- Manifest next steps should explain whether the pack is complete, handoff-ready, or planned.

## Testing Strategy

PDF tests:

- Unit tests with fake extractors.
- Integration tests with fixture PDFs:
  - born-digital text PDF
  - two-column technical paper
  - table-heavy PDF
  - scanned/no-text PDF
  - encrypted or unsupported PDF
  - oversized/truncated PDF
- Tests for missing Poppler, fallback use, extractor metadata, page refs, and PDF-only success/failure.

Pack tests:

- Unit tests for deterministic target selection.
- Golden JSON tests for `content-pack.json`.
- Tests that pack plans never introduce new factual claims beyond `visual-packet.json`.
- Fake planner tests before any real LLM planner.
- Fake image backend tests for per-target success, partial failure, rollback, manifest output entries, and quality validation.
- Codex handoff tests that confirm manual prompts exist and no direct Codex backend is implied.

Live OpenAI, PDF Oxide, and remote parsing tests should remain manual or optional smoke tests, not required CI gates.

## Phased Implementation

Release slice 1: PDF source quality.

- Add PDF extraction in `internal/source/pdf.go`.
- Prefer Poppler when installed.
- Add pure-Go fallback only after fixture validation.
- Emit page-aware evidence and extractor metadata.
- Normalize extracted text into Markdown-ish evidence.
- Update `doctor`, manifest warnings, docs, and tests.

Release slice 2: pack model and Codex handoff.

- Add `internal/pack`.
- Add `content-pack.json`.
- Add fixed targets: `linkedin-dense`, `social-teaser`, `blog-og`.
- Add per-target briefs.
- Add pack-level Codex handoff and quick command.
- Keep pack outputs honest: `planned` or `handoff_ready`, not generated.

Release slice 3: OpenAI pack generation.

- Generate each target through the existing OpenAI image path.
- Support partial success.
- Write target image hashes to `manifest.output_files`.
- Add image verification and per-target state.

Release slice 4: LLM pack planning.

- Add `--planner deterministic|openai`.
- Make planner output schema-first and reviewable.
- Keep planner from inventing facts.
- Preserve deterministic tests with fake planner responses.

Release slice 5: experimental smart PDF engines.

- Add explicit PDF Oxide adapter only after a fixture spike.
- Keep Poppler and deterministic normalization as the stable path.

## Research Summary

PDF dependency audit as of 2026-05-27:

- Poppler `pdftotext`: mature and current, good subprocess default when installed. Do not bundle or link it.
- PDF Oxide: technically promising and current, but too new and operationally surprising for default OSS CLI use because Go consumers need native release assets through CGo or purego shared libraries.
- Pure-Go readers such as `ledongthuc/pdf` or `dslipak/pdf`: install-friendly but weak for complex PDFs. Use only after fixture validation.
- `pdfcpu`: active and Apache-2.0, useful for diagnostics, not clean text extraction.
- MuPDF/go-fitz/mutool: strong extraction tools, but AGPL/commercial licensing makes them unsuitable for default OSS core.
- UniPDF: capable but commercial/license-key based, unsuitable for OSS default.
- Remote LLM parsing: useful only as explicit opt-in because of privacy, cost, and determinism.

## Sources

- Poppler `pdftotext`: https://poppler.freedesktop.org/
- Poppler Homebrew formula: https://formulae.brew.sh/formula/poppler
- PDF Oxide Go binding: https://github.com/yfedoseev/pdf_oxide/tree/main/go
- PDF Oxide Markdown docs: https://pdf.oxide.fyi/docs/extraction/markdown
- pdfcpu: https://github.com/pdfcpu/pdfcpu
- pdfcpu extract docs: https://pdfcpu.io/extract/extract/
- ledongthuc/pdf: https://github.com/ledongthuc/pdf
- MuPDF license: https://mupdf.readthedocs.io/en/latest/license.html
- UniPDF: https://github.com/unidoc/unipdf
