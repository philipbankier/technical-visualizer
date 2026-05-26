# Technical Visualizer v0.1 Evidence-First Hardening Design

Date: 2026-05-26

Status: design direction selected in chat, pending written-spec review before implementation planning.

Branch: `codex/v0.1-evidence-hardening`

## Context

`technical-visualizer` is a Go CLI that turns source-backed technical context into a visualization bundle. The current public repo is healthy enough to build and test, but it still reads like a release-prep branch rather than a product-quality open source repo.

Current verified state:

- Local branch before this design was `codex/oss-release-prep`.
- GitHub default branch is currently `codex/oss-release-prep`.
- CI is green on the pushed prep branch.
- `script/lint`, `script/test`, and `script/smoke` pass locally.
- `script/security` fails under local Go 1.26.0 because govulncheck reports standard-library issues fixed in Go 1.26.3.
- `GOTOOLCHAIN=go1.26.3 script/security` passes locally.
- The OpenAI backend uses `gpt-image-2` through the OpenAI Images API and requires `OPENAI_API_KEY`.
- The Codex backend is doctor-only in v1 and intentionally does not generate images.

## Release Promise

Version 0.1 should be positioned as an evidence-first technical visualization bundle CLI:

- Give it a URL or local source.
- It gathers bounded, source-backed evidence.
- It writes an auditable bundle: `scaffold.html`, `visual-packet.json`, `manifest.json`, and `final.png`.
- Local mode is private and deterministic.
- OpenAI image generation is explicit, API-key based, and uses `gpt-image-2`.
- Codex and ChatGPT subscriptions are useful for running agents around the tool, but they do not silently power `--backend openai`.

The repo should not position v0.1 as a finished image-generation product or a Codex subscription image backend.

## In Scope

- Rewrite public docs around the real first-run journey.
- Add clear docs for OpenAI API keys versus ChatGPT/Codex subscription use.
- Add agent workflow guidance for Codex and similar CLI agents.
- Improve `visualize doctor` wording so Codex is shown as an agent-workflow tool, not an image backend.
- Make `auto` and `hybrid` fallback behavior match the README promise.
- Fail clearly when all sources produce zero evidence.
- Add manifest fields that make runs easier to audit.
- Make prompt truncation visible and preserve the highest-value content first.
- Add examples and GitHub community files that make the repo feel launch-ready.
- Align CI and release workflows with the documented release gates.
- Publish all work on a feature branch and avoid pushing to the default branch.

## Out Of Scope

- Do not wire `--backend codex` for image generation in v0.1.
- Do not claim ChatGPT, Codex, Plus, Pro, Team, Business, or Enterprise subscriptions pay for OpenAI API image calls.
- Do not tag `v0.1.0` or create a GitHub release without explicit approval after implementation is verified.
- Do not build a hosted-service security model in this pass.
- Do not turn local fallback PNG rendering into a full design renderer unless it is small and clearly contained.

## Architecture

The hardening pass has four workstreams.

## Product Surface

This workstream owns:

- `README.md`
- `docs/`
- examples and sample outputs
- GitHub issue templates
- PR template
- release-readiness docs
- repo metadata recommendations

The docs should lead with a URL-first quickstart, then show local/offline operation. The README should explain each output file and tell users which artifact to inspect first.

The remote-call matrix should be explicit:

- `local`: no remote image call
- `offline`: no remote source fetch and no remote image call
- `openai`: sends source-derived prompt and scaffold content to OpenAI
- `auto` and `hybrid`: may call OpenAI when credentials are available, then fall back locally when OpenAI is unavailable or generation fails

The docs should include a section named `OpenAI API keys vs ChatGPT/Codex subscriptions` or equivalent. It must state that `OPENAI_API_KEY` is required for `--backend openai`, while `codex login` is a separate agent workflow path.

The docs should describe local `final.png` honestly. In v0.1, `scaffold.html` is the auditable local visualization artifact. `final.png` is either OpenAI output or a local fallback preview.

## CLI And Pipeline Behavior

This workstream owns:

- `internal/cli`
- `internal/pipeline`
- backend selection behavior
- user-facing command output
- behavior tests

Behavior changes:

- Explicit `--backend openai` fails when OpenAI generation fails.
- `--backend auto` and `--backend hybrid` use OpenAI when available.
- If OpenAI generation fails under `auto` or `hybrid`, write the local fallback PNG, return success, and record a manifest warning.
- If every source produces zero evidence, return a clear non-zero error before writing a success-looking bundle.
- `visualize doctor` should report three lanes:
  - local renderer availability
  - OpenAI Images API credential presence
  - Codex CLI availability for agent workflows only

The command should avoid implying that Codex is a generation backend in v0.1.

## Audit And Safety

This workstream owns:

- manifest schema additions
- source run accounting
- warning and redaction counts
- fallback accounting
- prompt truncation visibility
- related tests

The manifest should make these facts inspectable:

- tool version
- requested backend
- selected backend
- renderer
- style
- source count
- evidence item count
- warning count
- redaction count
- whether remote image generation was attempted
- whether fallback was used
- whether prompt content was truncated

Prompt construction should preserve priority when content is large:

- title
- required text
- top claims
- risks and unknowns
- source references
- lower-priority detail

If truncation happens, it should be visible outside the prompt, not only inside the prompt text. A manifest warning is enough for v0.1.

Private-network URL blocking is not required for this pass unless it can be added cleanly. The docs must still state that the CLI is not ready to expose as a network service without URL guards, authentication, and allowlists.

## Validation And Release

This workstream owns:

- scripts
- CI workflow
- release workflow
- local gate execution
- branch and PR flow
- final readiness evidence

Required gates:

- `script/lint`
- `script/test`
- `script/smoke`
- `GOTOOLCHAIN=go1.26.3 script/security`
- `go test -cover ./...`
- URL input smoke
- missing or empty evidence smoke
- `auto` fallback smoke with simulated OpenAI failure
- manifest inspection for audit fields
- docs scan for stale temporary-key or unresolved release-decision language
- staged secret scan before commit
- GitHub Actions check after pushing the feature branch

CI and release workflows should run the same release gates the docs tell maintainers to run, including `script/security`.

## Subagent Execution Plan

After this design spec is reviewed and accepted, implementation should be split across subagents with disjoint ownership.

Subagent A, product surface:

- README rewrite
- docs for agent workflows and backend behavior
- release-readiness cleanup
- examples and GitHub templates

Subagent B, CLI and pipeline:

- `auto` and `hybrid` fallback behavior
- empty evidence failure
- focused behavior tests

Subagent C, audit and doctor:

- manifest audit fields
- prompt truncation warning
- `visualize doctor` wording
- focused schema and output tests

Main agent:

- create and maintain the integration branch
- keep subagent changes from overlapping
- resolve conflicts
- run final validation
- scan for secrets
- push the feature branch
- open the PR if the repo state allows it
- avoid default-branch pushes and release tags

## Acceptance Criteria

The hardening pass is complete only when current evidence proves all of the following:

- Public docs explain the first-run path, output files, remote-call behavior, and v0.1 limits.
- Docs clearly separate OpenAI API-key image generation from ChatGPT/Codex subscription agent workflows.
- No docs claim `--backend codex` works for image generation.
- `visualize doctor` presents Codex as agent-workflow readiness, not image generation readiness.
- `auto` and `hybrid` fall back locally after OpenAI generation failure and record a warning.
- Explicit `openai` backend still fails when OpenAI generation fails.
- A run with zero evidence exits non-zero with a clear error.
- Manifest output includes the agreed audit fields.
- Prompt truncation is externally visible.
- Examples and GitHub templates exist.
- CI and release workflows run documented gates.
- Local validation gates pass.
- The feature branch is pushed.
- No default-branch push or release tag is made without explicit approval.

## Tradeoffs

This design keeps v0.1 credible by aligning claims with behavior. It does not chase the bigger offline renderer yet because that would turn the release-prep pass into a rendering engine project.

The ChatGPT/Codex subscription section is intentionally documentation-first. A subscription can help a user run agents around the tool, but there is no verified CLI image-generation bridge to use as a supported backend.

Private-network URL protection remains a documented hosted-service boundary unless it can be implemented cleanly in this pass. This avoids mixing local CLI release prep with a broader network-service threat model.

## Spec Review Notes

This spec has no empty sections. The design intentionally prepares for v0.1 but does not authorize tagging or publishing a release. Implementation should proceed only after this written spec is reviewed and accepted.
