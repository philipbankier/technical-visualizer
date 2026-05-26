# Content-Rich Codex Handoff Design

Date: 2026-05-26

Status: design approved in chat, written spec pending user review before implementation planning.

Branch: `codex/content-rich-codex-handoff-spec`

## Context

`technical-visualizer` currently works as an evidence-first Go CLI. It can gather a URL or local source, write `visual-packet.json`, build `scaffold.html`, record `manifest.json`, and write `final.png` through either a local fallback preview or the OpenAI Images API.

The first public release made the backend boundaries honest:

- `--backend openai` is the direct API path and requires `OPENAI_API_KEY`.
- Codex is documented as an agent handoff workflow, not a direct backend.
- Local output is the default.
- `scaffold.html` is the primary local artifact in v0.1.

A real agent trial exposed the next product problem. A dense 20 KB markdown research document with papers, statistics, tables, taxonomy, timelines, open questions, and ASCII diagrams produced a `visual-packet.json` with one weak claim and one weak fact:

```text
research-knowledge-base.md describes Table of Contents: 1.
research-knowledge-base.md evidence states Table of Contents: 1.
```

The agent had to reread the original source and handcraft the infographic and prompt. That means the generated packet was not useful enough for Codex, OpenAI, a human renderer, or any future renderer.

The core issue is upstream of Codex. The packet must carry enough source-backed signal to make a strong visual artifact without forcing the downstream renderer to reopen the source.

## Goal

Make ad hoc Codex infographic generation feel first-class while keeping the standalone Go CLI honest.

The tool should:

- Extract rich content from dense markdown and docs.
- Write a renderer-ready `visual-packet.json`.
- Write a better `scaffold.html` that reads like an infographic outline.
- Write a Codex handoff package that tells Codex exactly what to generate, what to preserve, what to avoid inventing, and where to save the image.
- Provide a quick mode that prints a ready manual interactive Codex command.
- Add `manifest.next_steps` so agents and humans know what to do after a local run.

The tool should not:

- Claim that `visualize --backend codex` works.
- Claim that ChatGPT or Codex subscriptions pay for OpenAI API calls.
- Depend on an unproven Codex CLI image-output command.
- Require remote processing for local-only runs.

## Recommended Approach

Use the content-rich handoff package approach.

This combines two necessary improvements:

1. Rich packet extraction, so the generated bundle contains enough signal.
2. First-class Codex handoff files, so the user does not have to invent the prompt.

This is better than a handoff-only patch because a handoff wrapper around a shallow packet still produces weak results. It is better than a packet-only improvement because users and agents still need explicit next steps and a ready Codex prompt.

## User Experience

## Full Handoff Mode

The full handoff command should generate the normal bundle plus a handoff folder.

Candidate CLI shape:

```bash
visualize --backend local --renderer html --handoff codex --out visualize-output ./research-knowledge-base.md
```

Expected outputs:

```text
visualize-output/
  final.png
  manifest.json
  scaffold.html
  visual-packet.json
  handoff/
    codex-prompt.md
    image-brief.md
    qa-checklist.md
    style.md
```

The CLI should print a concise post-run message that names the primary files and tells the user to open `handoff/codex-prompt.md`.

## Quick Mode

Quick mode should use the same generated bundle and handoff files, then print a ready manual Codex command.

Candidate CLI shape:

```bash
visualize --backend local --renderer html --handoff codex --quick --out visualize-output ./research-knowledge-base.md
```

Expected terminal output should include a command like:

```bash
codex -C visualize-output "$(cat handoff/codex-prompt.md)"
```

This command is intentionally manual. The CLI should not auto-run Codex in this version. Manual execution keeps the workflow transparent, avoids assuming interactive image tool availability, and avoids hiding subscription/API differences.

## Existing OpenAI Path

The OpenAI backend should remain the direct API path:

```bash
OPENAI_API_KEY=... visualize --backend openai --renderer image --out visualize-output ./research-knowledge-base.md
```

The richer packet should improve OpenAI results too, but this design does not change the OpenAI billing or credential model.

## Architecture

The pipeline should become:

1. Resolve sources.
2. Gather bounded evidence.
3. Extract rich source-backed content units.
4. Build `visual-packet.json`.
5. Render `scaffold.html`.
6. Generate `final.png` through selected backend or local fallback.
7. Write `manifest.json` with `next_steps`.
8. Optionally write `handoff/` artifacts.
9. Run quality validation.

The new extraction stage should be deterministic and local. It should not call an LLM. It should prioritize useful structure from source text and preserve source refs.

## Components

## Rich Content Extraction

Add a focused extraction layer between evidence gathering and packet building.

The extraction layer should accept `model.EvidenceBundle` and return structured content candidates for the packet builder.

It should support at least:

- Markdown section hierarchy.
- Paper or citation-like entries.
- Dates and timeline entries.
- Numeric metrics and statistics.
- Markdown tables.
- ASCII diagrams and code-fenced diagrams.
- Taxonomy-like tables or bullet groups.
- Open questions.
- Key concepts and definitions.
- Dense source excerpts with source refs.

The extractor should be conservative. It can preserve source text as excerpts when it cannot classify content perfectly. The goal is not perfect semantic parsing. The goal is to avoid dropping important source signal.

## Visual Packet Schema

Extend `model.VisualPacket` with renderer-ready fields:

- `content_blocks`: typed source-backed blocks for sections, tables, metrics, diagrams, timelines, questions, concepts, and excerpts.
- `metrics`: normalized important numbers with labels, values, context, and source refs.
- `timeline`: dated or ordered events.
- `entities`: named papers, systems, methods, projects, or concepts.
- `tables`: markdown table structures or compact row summaries.
- `diagrams`: ASCII or fenced diagram snippets.
- `open_questions`: explicit research questions or unknowns.

The existing `ranked_claims`, `facts`, `sections`, `risks`, `unknowns`, `source_refs`, and constraints should remain. The new fields should complement them rather than break existing consumers.

## Packet Builder

The packet builder should use the extracted content to produce:

- More than one claim/fact for dense single-file inputs.
- Source-backed claims from meaningful sections, not the table of contents.
- Required text with important labels, not only top-level headings.
- Sections that reflect the source document's actual conceptual structure.
- Content blocks that downstream renderers can use directly.

For a dense research markdown fixture, the packet should include the core papers, key stats, taxonomy, timeline, open questions, and diagrams without reopening the source.

## Codex Handoff Artifacts

When `--handoff codex` is set, write:

`handoff/codex-prompt.md`

- Direct instructions to Codex.
- References to `visual-packet.json` and `scaffold.html`.
- The expected output path, normally `final.png`.
- Design-quality instruction for a polished single-image infographic.
- Source discipline: preserve required text, do not invent facts, cite uncertainty visibly.
- A request to inspect `image-brief.md` and `qa-checklist.md`.

`handoff/image-brief.md`

- Human-readable synthesis of the packet.
- The top source-backed story.
- Suggested visual hierarchy.
- Important content to include.
- Risks and open questions.
- Style notes.

`handoff/qa-checklist.md`

- Checks Codex or a human should perform before accepting the image.
- Includes source fidelity, text readability, required text, no invented claims, visible uncertainty, and output path.

`handoff/style.md`

- The selected style profile in plain language.
- For `executive-dark`, use premium modern technical infographic language.
- Include guidance for dense but readable visual hierarchy.

These files should be deterministic and local.

## Manifest Next Steps

Add `next_steps` to `model.Manifest`.

The field should be an ordered list of strings. It should be contextual:

- Local HTML run: explain that `scaffold.html` is the local primary artifact, OpenAI can be used with `--backend openai --renderer image`, and Codex handoff can use `handoff/codex-prompt.md` when present.
- Local run without handoff: suggest rerunning with `--handoff codex` for a guided Codex package.
- Handoff run: point to `handoff/codex-prompt.md` and the printed quick command if quick mode was used.
- OpenAI success: point to `final.png` and `manifest.json`.
- Auto/hybrid fallback: explain fallback and suggest explicit OpenAI or Codex handoff.
- Offline run: make clear that no remote source fetching or remote image generation happened.

## Scaffold Rendering

Upgrade `scaffold.html` from an audit list to a richer infographic outline.

It should render:

- Title and thesis.
- Key metrics.
- Timeline.
- Tables or compact table summaries.
- Concepts and taxonomy.
- Open questions.
- Source-backed claims and source refs.
- Diagrams as preformatted blocks when present.

It should remain static HTML with no scripts. It should continue to be safe to open locally.

## Fallback PNG

Fallback PNG improvement is deferred from the first implementation unless the user expands scope after packet, scaffold, manifest, and handoff behavior pass review.

When it is implemented later, it must be test-driven and should not add brittle dependencies. Acceptable directions:

- A simple text-rendered PNG using Go image/font packages.
- Optional headless browser screenshot only if the dependency and runtime behavior are proven and documented.

Until then, the docs and manifest should keep labeling local `final.png` as a preview.

## Data Flow

Dense markdown example:

1. Source gatherer reads `research-knowledge-base.md` as one evidence item.
2. Extractor splits it into sections and content units.
3. Tables, metrics, timelines, open questions, and diagrams are captured as structured blocks.
4. Packet builder turns those blocks into `visual-packet.json`.
5. Scaffold renderer presents those blocks visibly.
6. Handoff writer creates Codex-ready prompt and brief files.
7. Manifest records outputs, audit fields, and next steps.
8. Quick mode prints a command for manual interactive Codex.

The user or agent should be able to generate a strong infographic using only:

- `visual-packet.json`
- `scaffold.html`
- `handoff/codex-prompt.md`
- `handoff/image-brief.md`
- `handoff/qa-checklist.md`

The original source should not be required for normal downstream rendering.

## Error Handling

Invalid `--handoff` values should fail before bundle writes.

Quick mode without `--handoff codex` should fail with a clear message. The first version should require `--handoff codex --quick`.

Handoff file write failures should fail the run and clean up partial handoff files where practical.

Extraction should not fail the run for imperfect classification. It should preserve excerpts and add warnings only for real data-loss conditions such as truncation.

If source content is too large and excerpts are truncated, the packet and manifest should make truncation visible.

## Privacy And Safety

The extraction and handoff package are local-only steps. They should not call remote services.

The handoff prompt should warn users not to send private source to remote agents or image tools unless acceptable.

Generated handoff files can contain source-derived content and should inherit the existing private file permissions.

Secret redaction and secret-like path rejection remain required.

## TDD Requirements

Implementation must follow strict test-driven development.

No production code should be written for a behavior until a failing test exists and has been run.

Each task should follow:

1. Write a focused failing test.
2. Run the narrow test and record that it fails for the expected reason.
3. Add minimal code to pass.
4. Run the narrow test and relevant package tests.
5. Refactor only after green.
6. Run the broader gates required for the task.

Tests should use real code. Mocks are allowed only for unavoidable external boundaries.

## Test Plan

## Dense Markdown Fixture

Add a fixture modeled on the failure case. It should be dense enough to prove the behavior:

- Executive summary.
- Core papers with titles, authors, dates, and identifiers.
- Statistics such as `+23.5`, `52/52`, `1-4 edits`, and token ranges.
- Markdown tables.
- A taxonomy section.
- A timeline section.
- A SkillOpt-like pipeline section.
- Open research questions.
- ASCII diagrams or fenced diagrams.

## Packet Tests

Red tests should prove current behavior fails by producing too little useful packet content from the dense markdown fixture.

Green behavior should assert:

- More than one claim and fact.
- Metrics survive with labels and source refs.
- Tables are represented.
- Timeline entries are represented.
- Open questions are represented.
- Diagrams are represented.
- Important papers or named concepts are represented.
- The packet does not rely only on table-of-contents text.

## Handoff Tests

Tests should assert:

- `--handoff codex` writes the handoff folder.
- `codex-prompt.md`, `image-brief.md`, `qa-checklist.md`, and `style.md` exist.
- The prompt references `visual-packet.json`, `scaffold.html`, and the desired output path.
- The prompt does not claim `--backend codex` exists.
- Handoff files contain enough packet-derived content to be useful.

## Quick Mode Tests

Tests should assert:

- Quick mode prints a manual interactive Codex command.
- The command points at the output directory.
- The command uses the generated prompt file.
- Quick mode does not auto-run Codex.

## Manifest Tests

Tests should assert contextual `next_steps` for:

- Local run without handoff.
- Local run with Codex handoff.
- OpenAI run.
- Auto/hybrid fallback.
- Offline run.

## Scaffold Tests

Tests should assert that rich packet fields render in `scaffold.html`:

- Metrics.
- Timeline.
- Tables or row summaries.
- Open questions.
- Diagrams.
- Source refs.

## Agent Usability Review

A review subagent should attempt to plan an infographic using only generated packet and handoff files. It must not reopen the original source.

The review passes only if the subagent can identify the core papers, key statistics, taxonomy, timeline, and open questions from the generated artifacts.

## Subagent Execution Plan

This spec is designed for subagent-driven implementation.

Recommended task split:

1. Packet schema and dense markdown fixture.
2. Markdown content extraction.
3. Packet builder integration.
4. Scaffold rendering upgrade.
5. Manifest `next_steps`.
6. Codex handoff artifact writer.
7. CLI flags and quick mode output.
8. Docs and examples.
9. Final validation and PR.

Each task should have:

- A worker subagent for implementation.
- A spec-compliance reviewer.
- A code-quality reviewer.

Implementation tasks should be sequential where data contracts are shared. The fixture and packet schema should land before handoff and scaffold work. Docs should land after behavior is stable.

## Acceptance Criteria

- A dense single markdown file produces a rich `visual-packet.json`.
- The generated packet contains important papers, metrics, tables, timeline entries, taxonomy or concept blocks, open questions, and diagrams when present in the source.
- `scaffold.html` visibly renders the richer packet content.
- `--handoff codex` writes the handoff package.
- Quick mode prints a ready manual Codex command and does not auto-run Codex.
- `manifest.json` includes contextual `next_steps`.
- Docs clearly separate OpenAI API use from Codex subscription and agent workflows.
- No docs or prompts claim `visualize --backend codex` works.
- Local and offline privacy behavior remains unchanged.
- Existing release gates pass: `script/lint`, `script/test`, `script/smoke`, `GOTOOLCHAIN=go1.26.3 script/security`, and `go test -cover ./...`.
- Agent usability review passes using only generated artifacts.

## Implementation Planning Decisions

These decisions should be carried into the implementation plan:

- Require `--handoff codex --quick` for the first version.
- Start with `style.md` because Codex consumes prose well.
- Defer fallback PNG unless the implementation has time after the packet, scaffold, manifest, and handoff pass review.
