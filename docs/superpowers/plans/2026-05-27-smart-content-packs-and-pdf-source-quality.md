# Smart Content Packs And PDF Source Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add useful PDF-only source handling and first-class content packs so one source-backed packet can produce a dense infographic, a prettier lower-density social image, and a blog or README hero without losing auditability.

**Architecture:** Keep the existing Go pipeline. Source gathering emits evidence, packet building produces `visual-packet.json`, an optional `internal/pack` stage writes `content-pack.json` and target briefs, handoff files remain local agent instructions, and direct OpenAI generation remains the only remote image backend. PDF extraction stays package-local in `internal/source` and uses Poppler when available.

**Tech Stack:** Go 1.26, standard library tests, Poppler `pdftotext` as an optional system tool, existing OpenAI image backend, existing `script/*` release gates, deterministic local pack planning before any LLM planner.

---

## Guardrails

- Use strict TDD. Write the failing test, run it, then write the smallest passing code.
- Use one worker subagent per task during execution. Each worker owns only the files listed for that task.
- Workers are not alone in the codebase. They must not revert unrelated edits and must adapt to changes already present.
- After each worker task, run one spec-compliance review subagent and one code-quality review subagent before committing.
- Commit after each reviewed task with a semantic commit prefix.
- Never push to `main` or the default branch. Push only the feature branch.
- Keep generated bundles, caches, local PDFs, screenshots, API keys, and `.env` files out of commits.
- Update `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html` whenever a task makes a design decision, deviation, tradeoff, or discovers an open question.
- Keep `--backend codex` rejected. Codex remains a handoff workflow.
- Do not add PDF Oxide, MuPDF, UniPDF, or remote PDF parsing in this plan.
- Do not add a pure-Go PDF fallback unless the worker first proves value with the fixture tests in Task 3 and records the result in the implementation notes.

## File Structure

- Modify `docs/superpowers/specs/2026-05-27-smart-content-packs-and-pdf-source-quality-design.md`: keep approval status current.
- Modify `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`: running notes.
- Create `internal/source/pdf.go`: Poppler engine selection, subprocess runner seam, PDF text normalization, page-aware evidence, extractor metadata.
- Modify `internal/source/local.go`: route local PDFs through `pdf.go`.
- Modify `internal/source/web.go`: route remote PDFs through `pdf.go` after bounded download and temp-file handling.
- Modify `internal/source/gatherer.go`: add PDF gather options and normalize defaults if needed.
- Modify `internal/source/gatherer_test.go`: replace warning-only PDF expectations with useful evidence and precise failure expectations.
- Create `testdata/pdf/*.txt`: raw text fixtures for born-digital, two-column, table-heavy, scanned or empty, and low-evidence cases.
- Create optional `testdata/pdf/*.pdf`: tiny hand-built fixture PDFs only when a worker needs subprocess integration coverage.
- Modify `internal/extract/extract.go`: add plain-text section splitting support only if Markdown-ish PDF normalization does not provide enough structure.
- Modify `internal/extract/extract_test.go`: prove PDF-normalized text yields sections, metrics, timeline, tables, diagrams, and open questions.
- Modify `internal/model/model.go`: add source diagnostics and content pack JSON structs only if the package boundary proves simpler than keeping pack structs under `internal/pack`.
- Modify `internal/model/model_test.go`: JSON round-trip coverage for source diagnostics if added to `model`.
- Create `internal/pack/pack.go`: content-pack schema, deterministic targets, status derivation, brief data model.
- Create `internal/pack/pack_test.go`: target selection, status derivation, source-bound claims, golden JSON.
- Modify `internal/handoff/handoff.go`: add pack-level Codex prompt writing without changing the existing single-image handoff contract.
- Modify `internal/handoff/handoff_test.go`: pack handoff paths, file modes, prompt content, no direct backend wording.
- Modify `internal/pipeline/pipeline.go`: add `--pack` orchestration, generated file backup/rollback, manifest output entries, pack next steps, PDF diagnostics.
- Modify `internal/pipeline/pipeline_test.go`: pack bundle behavior, partial generation, rollback, local/offline honesty, PDF-only success/failure.
- Modify `internal/cli/cli.go`: add `--pack`, later `--planner`, doctor PDF tool status, quick pack command output.
- Modify `internal/cli/cli_test.go`: CLI flags, help text, quick output, Codex backend rejection.
- Modify `internal/backend/backend.go`: add request metadata only if pack image generation needs target-specific fields.
- Modify `internal/backend/openai.go`: reuse existing image generation path for each pack target.
- Modify `internal/backend/backend_test.go`: prompt build and fake backend coverage for target prompts.
- Modify `internal/quality/quality.go`: validate optional `content-pack.json`, target briefs, target image files when declared, and source diagnostics.
- Modify `internal/quality/quality_test.go`: valid and invalid pack bundles.
- Modify `README.md`, `docs/backends.md`, `docs/agent-workflows.md`, and `docs/release-readiness.md`: PDF and pack behavior.
- Modify `examples/sample-bundle/README.md` and sample outputs if CLI output shape changes.
- Create or modify `e2e_test.go`: broad local CLI smoke for `--pack auto` if no equivalent test exists.

## Pre-Execution Setup

- [ ] Confirm the active Codex Goal is scoped to this plan and not to the older content-rich handoff plan:

```bash
git status --short --branch
```

Expected: branch is `codex/content-rich-codex-handoff`, not `main`, and the worker understands which local files are already changed.

- [ ] Confirm the plan and approved spec are available to every worker:

```bash
test -s docs/superpowers/plans/2026-05-27-smart-content-packs-and-pdf-source-quality.md
test -s docs/superpowers/specs/2026-05-27-smart-content-packs-and-pdf-source-quality-design.md
test -s docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html
```

Expected: all commands exit 0.

- [ ] Use this prompt shape for each worker:

```text
You are a worker on technical-visualizer. Use superpowers:test-driven-development.
Own only the files listed in Task N of docs/superpowers/plans/2026-05-27-smart-content-packs-and-pdf-source-quality.md.
Do not revert unrelated edits. Start with the failing tests in the task, run the exact red command, make the smallest passing change, update the implementation notes if you make a design decision, then run the exact task gates. Return files changed, tests run, and any risks.
```

## Task 1: Planning Baseline And Notes

**Owner:** Parent agent.

**Files:**

- Modify: `docs/superpowers/specs/2026-05-27-smart-content-packs-and-pdf-source-quality-design.md`
- Create or modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`
- Create: `docs/superpowers/plans/2026-05-27-smart-content-packs-and-pdf-source-quality.md`

- [ ] Set the spec status to `approved for implementation planning`.
- [ ] Create the job-specific implementation notes file with sections for design decisions, deviations, tradeoffs, open questions, and validation evidence.
- [ ] Save this plan under `docs/superpowers/plans/`.
- [ ] Run:

```bash
CHECK_PATTERN='TB''D|TO''DO|FIX''ME|implement'' later|fill'' in|Similar'' to'
rg -n "$CHECK_PATTERN" docs/superpowers/plans/2026-05-27-smart-content-packs-and-pdf-source-quality.md
git diff --check
```

Expected: `rg` returns no matches and `git diff --check` exits 0.

- [ ] Commit after review:

```bash
git add docs/superpowers/specs/2026-05-27-smart-content-packs-and-pdf-source-quality-design.md docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html docs/superpowers/plans/2026-05-27-smart-content-packs-and-pdf-source-quality.md
SECRET_PATTERN='sk-(''proj|live|test|svcacct|admin)-[A-Za-z0-9_-]+|OPENAI_API''_KEY=[A-Za-z0-9_-]+|temporary API key'' supplied|temp api'' key'
git diff --cached | rg -n "$SECRET_PATTERN" || true
git commit -m "docs: plan smart content packs"
git push
```

Expected: secret scan prints no matches, commit succeeds, push updates the feature branch.

## Task 2: PDF Extractor Seam And PDF-Only Failure Contract

**Owner:** Worker A.

**Files:**

- Create: `internal/source/pdf.go`
- Modify: `internal/source/local.go`
- Modify: `internal/source/web.go`
- Modify: `internal/source/gatherer_test.go`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Replace `TestGatherPDFProducesEvidenceOrBoundedWarning` with red tests for explicit behavior:

```go
func TestGatherLocalPDFUsesExtractorEvidence(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "paper.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	restore := usePDFExtractorForTest(t, fakePDFExtractor{
		result: pdfExtraction{
			Engine:         "fake-pdftotext",
			Version:        "fake 1.0",
			PageCount:      2,
			ExtractedPages: 2,
			Text: "# Abstract\n\nSkillOpt reports +23.5 accuracy.\n\n# 1 Introduction\n\n- 2025-01: SkillOpt reports results.",
			PageRefs: []pdfPageRef{
				{Page: 1, Ref: "paper.pdf#page=1"},
				{Page: 2, Ref: "paper.pdf#page=2"},
			},
		},
	})
	defer restore()

	spec := model.SourceSpec{ID: "src-pdf", Kind: model.SourcePDF, Input: path, Resolved: path}
	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	item := findItemBySource(bundle, "src-pdf")
	if item == nil {
		t.Fatalf("expected PDF evidence item, warnings=%#v", bundle.Warnings)
	}
	if item.Kind != "pdf_text" {
		t.Fatalf("Kind = %q, want pdf_text", item.Kind)
	}
	if !strings.Contains(item.Text, "SkillOpt reports +23.5 accuracy") {
		t.Fatalf("Text = %q", item.Text)
	}
	if item.Metadata["pdf_engine"] != "fake-pdftotext" || item.Metadata["pdf_pages_extracted"] != "2" {
		t.Fatalf("Metadata = %#v", item.Metadata)
	}
	if !slices.Contains(item.SourceRefs, "paper.pdf#page=1") {
		t.Fatalf("SourceRefs = %#v, want page ref", item.SourceRefs)
	}
}
```

Add the missing `slices` import when the test uses it.

- [ ] Add a red test for missing extractor:

```go
func TestGatherLocalPDFWarnsWhenExtractorUnavailable(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "paper.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	restore := usePDFExtractorForTest(t, fakePDFExtractor{
		err: errPDFExtractorUnavailable,
	})
	defer restore()

	spec := model.SourceSpec{ID: "src-pdf", Kind: model.SourcePDF, Input: path, Resolved: path}
	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	if item := findItemBySource(bundle, "src-pdf"); item != nil {
		t.Fatalf("unexpected PDF evidence item: %#v", item)
	}
	if !containsWarning(bundle.Warnings, "install Poppler") {
		t.Fatalf("warnings = %#v, want install guidance", bundle.Warnings)
	}
}
```

- [ ] Run the red command:

```bash
go test ./internal/source -run 'TestGatherLocalPDF' -count=1
```

Expected: fails to compile because `usePDFExtractorForTest`, `fakePDFExtractor`, `pdfExtraction`, and `errPDFExtractorUnavailable` do not exist.

- [ ] Add the package-local seam in `internal/source/pdf.go`:

```go
type pdfExtractor interface {
	ExtractPDFText(context.Context, string, GatherOptions) (pdfExtraction, error)
}

type pdfExtraction struct {
	Engine         string
	Version        string
	PageCount      int
	ExtractedPages int
	Truncated      bool
	Text           string
	PageRefs       []pdfPageRef
	Warnings       []string
}

type pdfPageRef struct {
	Page int
	Ref  string
}
```

Add `errPDFExtractorUnavailable` as a package-local sentinel error.

- [ ] Add `usePDFExtractorForTest` and `fakePDFExtractor` in `gatherer_test.go` near other helpers. Keep the seam package-local, not exported.
- [ ] Update `gatherLocalPDF` and `gatherRemotePDF` to call a shared `gatherPDFText(ctx, spec, localPath, displayLocator, opts)` helper. At this stage remote PDF can still use a temp file, but it must share the same evidence contract.
- [ ] Ensure successful PDF evidence sets:

```go
model.EvidenceItem{
	Kind:       "pdf_text",
	Title:      filepath.Base(displayLocator),
	Text:       extraction.Text,
	Path:       localPathOrEmpty,
	URL:        remoteURLOrEmpty,
	SHA256:     sha256Hex(originalBytesOrFileBytes),
	Metadata:   pdfMetadata(extraction),
	SourceRefs: pdfSourceRefs(displayLocator, extraction),
}
```

- [ ] Run:

```bash
go test ./internal/source -run 'TestGatherLocalPDF|TestGatherRemotePDF' -count=1
```

Expected: passes. Existing non-PDF source tests also pass.

## Task 3: Poppler Adapter And PDF Text Normalization

**Owner:** Worker B.

**Files:**

- Modify: `internal/source/pdf.go`
- Modify: `internal/source/gatherer_test.go`
- Create: `testdata/pdf/born-digital.txt`
- Create: `testdata/pdf/two-column.txt`
- Create: `testdata/pdf/table-heavy.txt`
- Create: `testdata/pdf/scanned-empty.txt`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add red tests for normalization from raw Poppler text:

```go
func TestNormalizePDFTextPromotesPaperHeadings(t *testing.T) {
	raw := "AI Agent Skills\n\fAbstract\nSkillOpt reports +23.5 accuracy.\n\n1 Introduction\nAgent skill libraries reuse procedures.\n\n2.3 Method\nWe evaluate 52/52 tasks.\n\nReferences\n[1] Toolformer\n"
	got := normalizePDFTextToMarkdown(raw)
	for _, want := range []string{"# Abstract", "SkillOpt reports +23.5 accuracy", "# 1 Introduction", "# 2.3 Method", "# References"} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized text missing %q:\n%s", want, got)
		}
	}
}

func TestNormalizePDFTextDehyphenatesLineBreaks(t *testing.T) {
	got := normalizePDFTextToMarkdown("procedur-\nal knowledge improves repeatability")
	if strings.Contains(got, "procedur-") || !strings.Contains(got, "procedural knowledge") {
		t.Fatalf("normalized text = %q", got)
	}
}
```

- [ ] Add a red test for low evidence:

```go
func TestPDFExtractionTooLittleTextReturnsWarningOnly(t *testing.T) {
	result := pdfExtraction{Engine: "fake", Version: "1", Text: "1 2 3", ExtractedPages: 1}
	if hasUsefulPDFText(result.Text) {
		t.Fatalf("hasUsefulPDFText returned true for low-signal extraction")
	}
}
```

- [ ] Run:

```bash
go test ./internal/source -run 'NormalizePDFText|TooLittleText' -count=1
```

Expected: fails because normalization and low-evidence helpers do not exist.

- [ ] Add `normalizePDFTextToMarkdown` in `internal/source/pdf.go`. It must:
  - convert `\r\n` and `\r` to `\n`
  - preserve form-feed page boundaries as blank lines plus an HTML comment like `<!-- page 2 -->`
  - dehyphenate alphabetic line breaks such as `procedur-\nal`
  - promote common headings to Markdown headings: `Abstract`, `Introduction`, `References`, `Appendix`, numbered headings like `1 Introduction` and `2.3 Method`
  - keep table-like lines with repeated spacing or `|` separators intact
  - collapse more than two blank lines to two

- [ ] Add `hasUsefulPDFText` with this first-pass threshold:

```go
const minUsefulPDFLetters = 200
```

The helper should count Unicode letters after normalization. If the normalized text has fewer than 200 letters, `gatherPDFText` emits no item and returns a warning that says the extractor produced too little readable text.

- [ ] Add the Poppler runner seam:

```go
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error)
	LookPath(file string) (string, error)
}
```

Production runner uses `exec.CommandContext` and `exec.LookPath`. Tests use a fake runner.

- [ ] Add `popplerExtractor` that:
  - checks `pdftotext` with `LookPath`
  - obtains version from `pdftotext -v`
  - runs `pdftotext -layout -enc UTF-8 <path> -`
  - splits output on `\f` to infer extracted pages when Poppler emits page separators
  - normalizes raw text before returning it

- [ ] Run:

```bash
go test ./internal/source -run 'PDF|Normalize|Poppler' -count=1
```

Expected: passes without requiring real Poppler because unit tests use the fake runner.

- [ ] If a real `pdftotext` exists locally, run this optional smoke and record output in the notes:

```bash
command -v pdftotext
pdftotext -v 2>&1 | head -n 1
```

Expected when installed: command path and version line print. Expected when missing: worker records that local live PDF smoke was skipped and keeps unit coverage as the gate.

## Task 4: PDF Evidence Into Rich Packets

**Owner:** Worker C.

**Files:**

- Modify: `internal/source/pdf.go`
- Modify: `internal/extract/extract.go`
- Modify: `internal/extract/extract_test.go`
- Modify: `internal/packet/builder_test.go`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add a red packet-level test using PDF-normalized text as evidence:

```go
func TestBuildPacketFromPDFTextKeepsRichSignals(t *testing.T) {
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{ID: "src-paper", Kind: model.SourcePDF, Input: "paper.pdf"}},
		Items: []model.EvidenceItem{{
			ID:       "item-paper",
			SourceID: "src-paper",
			Kind:     "pdf_text",
			Title:    "paper.pdf",
			Text: "# Abstract\n\nSkillOpt reports +23.5 accuracy.\n\n# Core Papers\n\n### 1. Voyager\n\n- Date: 2023-05\n- Result: solved 52/52 discovered tasks.\n\n# Timeline\n\n- 2025-01: SkillOpt reports automated skill prompt optimization.\n\n# Open Questions\n\n1. How should agents choose overlapping skills?\n",
			SourceRefs: []string{"paper.pdf#page=1"},
		}},
	}
	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(packet.Metrics) == 0 || len(packet.Timeline) == 0 || len(packet.Entities) == 0 || len(packet.OpenQuestions) == 0 {
		t.Fatalf("packet missing rich PDF signals: metrics=%#v timeline=%#v entities=%#v questions=%#v", packet.Metrics, packet.Timeline, packet.Entities, packet.OpenQuestions)
	}
}
```

- [ ] Run:

```bash
go test ./internal/packet -run TestBuildPacketFromPDFTextKeepsRichSignals -count=1
```

Expected: fails only if existing extraction does not recognize the normalized PDF text shape.

- [ ] If the test fails, make the smallest change in `internal/extract/extract.go`. Prefer improving existing Markdown section handling over creating `internal/extract/normalize`.
- [ ] Add extraction tests for the actual gap, for example numbered open questions or heading promotion side effects.
- [ ] Run:

```bash
go test ./internal/extract ./internal/packet -run 'PDF|OpenQuestions|Timeline|Entity|Metric' -count=1
```

Expected: passes and proves PDF-derived Markdown-ish evidence feeds existing packet fields.

## Task 5: PDF Pipeline, Doctor, Manifest Diagnostics, And Docs

**Owner:** Worker D.

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/model/model_test.go`
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/quality/quality.go`
- Modify: `internal/quality/quality_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `docs/backends.md`
- Modify: `docs/release-readiness.md`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add red model and quality tests for source diagnostics:

```go
func TestManifestSourceDiagnosticsMarshal(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "manifest/v1",
		Sources:       []SourceSpec{{ID: "src-paper", Kind: SourcePDF, Input: "paper.pdf"}},
		Backend:       BackendInfo{Name: "local"},
		Renderer:      "html",
		Style:         "executive-dark",
		NextSteps:     []string{"Open scaffold.html."},
		Audit:         ManifestAudit{ToolVersion: "0.1.0", RequestedBackend: "local", SelectedBackend: "local", SourceCount: 1, EvidenceItemCount: 1},
		SourceDiagnostics: []SourceDiagnostic{{
			SourceID:       "src-paper",
			Kind:           "pdf",
			Engine:         "pdftotext",
			Version:        "pdftotext 25.10.0",
			PagesAttempted: 2,
			PagesExtracted: 2,
		}},
		OutputFiles: []OutputFile{{Kind: "manifest", Path: "manifest.json"}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `"source_diagnostics"`) || !strings.Contains(string(data), `"pdftotext"`) {
		t.Fatalf("manifest JSON missing source diagnostics: %s", data)
	}
}
```

- [ ] Run:

```bash
go test ./internal/model ./internal/quality -run 'SourceDiagnostics|Manifest' -count=1
```

Expected: fails because `SourceDiagnostic` and `Manifest.SourceDiagnostics` do not exist.

- [ ] Add:

```go
type SourceDiagnostic struct {
	SourceID       string   `json:"source_id"`
	Kind           string   `json:"kind"`
	Engine         string   `json:"engine,omitempty"`
	Version        string   `json:"version,omitempty"`
	PagesAttempted int      `json:"pages_attempted,omitempty"`
	PagesExtracted int      `json:"pages_extracted,omitempty"`
	Truncated      bool     `json:"truncated,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}
```

Add `SourceDiagnostics []SourceDiagnostic `json:"source_diagnostics,omitempty"` to `model.Manifest`.

- [ ] Derive PDF diagnostics in `pipeline.Run` from public evidence item metadata and public warnings. Do not include local absolute paths.
- [ ] Add `visualize doctor` output for Poppler:

```text
Poppler pdftotext: available (local, version...)
```

or:

```text
Poppler pdftotext: unavailable (install Poppler for PDF-only sources)
```

- [ ] Add pipeline tests:
  - PDF-only succeeds when fake extractor emits useful text.
  - PDF-only fails with a no-evidence error and install guidance when extractor is unavailable.
  - Mixed markdown plus failed PDF succeeds and keeps a warning.
  - Output files do not leak temp directories or absolute PDF paths.

- [ ] Run:

```bash
go test ./internal/source ./internal/pipeline ./internal/cli ./internal/quality -run 'PDF|Doctor|SourceDiagnostics|NoEvidence' -count=1
```

Expected: passes.

- [ ] Update docs to replace the v0.1 warning-only PDF limitation. The docs must say:
  - PDF extraction uses local Poppler when installed.
  - PDF-only runs can succeed when text extraction produces enough readable text.
  - scanned or image-only PDFs still fail with a low-evidence message.
  - no PDF content is sent remotely unless the user chooses a remote image backend or future explicit remote parsing.

## Task 6: Content Pack Schema And Deterministic Planner

**Owner:** Worker E.

**Files:**

- Create: `internal/pack/pack.go`
- Create: `internal/pack/pack_test.go`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add red tests for the content pack schema:

```go
func TestPlanAutoCreatesDefaultTargets(t *testing.T) {
	packet := sampleVisualPacket()
	contentPack, err := Plan(packet, Options{Mode: "auto"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if contentPack.SchemaVersion != "content-pack/v1" {
		t.Fatalf("SchemaVersion = %q", contentPack.SchemaVersion)
	}
	if contentPack.Status != StatusPlanned {
		t.Fatalf("Status = %q, want planned", contentPack.Status)
	}
	gotIDs := targetIDs(contentPack.Targets)
	wantIDs := []string{"linkedin-dense", "social-teaser", "blog-og"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("target IDs = %#v, want %#v", gotIDs, wantIDs)
	}
}

func TestPlanDoesNotInventClaims(t *testing.T) {
	packet := sampleVisualPacket()
	contentPack, err := Plan(packet, Options{Mode: "auto"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	allowed := strings.Join(append(packet.RequiredText, claimTexts(packet.RankedClaims)...), "\n")
	for _, target := range contentPack.Targets {
		for _, text := range target.RequiredContent {
			if !strings.Contains(allowed, text) && !allowedContentToken(text) {
				t.Fatalf("target %s required content invents %q", target.ID, text)
			}
		}
	}
}
```

- [ ] Run:

```bash
go test ./internal/pack -count=1
```

Expected: fails because `internal/pack` does not exist.

- [ ] Add package `internal/pack` with these exported types:

```go
type Status string
type TargetState string

const (
	StatusPlanned      Status = "planned"
	StatusHandoffReady Status = "handoff-ready"
	StatusPartial      Status = "partial"
	StatusComplete     Status = "complete"

	StatePlanned      TargetState = "planned"
	StateHandoffReady TargetState = "handoff_ready"
	StateGenerated    TargetState = "generated"
	StateVerified     TargetState = "verified"
	StateFailed       TargetState = "failed"
)

type ContentPack struct {
	SchemaVersion string       `json:"schema_version"`
	SourcePacket  string       `json:"source_packet"`
	Title         string       `json:"title"`
	Status        Status       `json:"status"`
	Strategy      Strategy     `json:"strategy"`
	Targets       []TargetSpec `json:"targets"`
}
```

Include fields from the approved spec for `Strategy` and `TargetSpec`: planner, summary, platform, intent, density, aspect ratio, brief path, output path, state, failure reason, required content, avoid.

- [ ] Add `Plan(packet model.VisualPacket, opts Options) (ContentPack, error)` with only `Mode: "auto"` and deterministic targets in this task.
- [ ] Add `DeriveStatus(targets []TargetSpec) Status` and tests for all planned, handoff-ready, partial, complete.
- [ ] Keep the approved wire-format split explicit in tests: target state `handoff_ready`, overall pack status `handoff-ready`.
- [ ] Add golden JSON assertion by marshalling a sample pack with `json.MarshalIndent` and comparing key snippets, not brittle whitespace.
- [ ] Run:

```bash
go test ./internal/pack -count=1
```

Expected: passes.

## Task 7: Pack Bundle Writes, Manifest Entries, Quality Validation

**Owner:** Worker F.

**Files:**

- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/quality/quality.go`
- Modify: `internal/quality/quality_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add red CLI and pipeline tests for `--pack auto`:

```go
func TestRunPackAutoWritesPackAndBriefs(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, rel := range []string{"content-pack.json", "pack/linkedin-dense/brief.md", "pack/social-teaser/brief.md", "pack/blog-og/brief.md"} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("Stat(%s) error = %v", rel, err)
		}
	}
	if !hasOutputKind(manifest.OutputFiles, "content_pack") {
		t.Fatalf("manifest missing content_pack output: %#v", manifest.OutputFiles)
	}
	if hasOutputKind(manifest.OutputFiles, "pack_image") {
		t.Fatalf("local planned pack should not declare generated images: %#v", manifest.OutputFiles)
	}
	assertNextStepContains(t, manifest.NextSteps, "content-pack.json")
}
```

Add a CLI parse and rejection test:

```go
func TestRunUnsupportedPackValueFailsBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--pack", "carousel", "--out", outputDir, sourcePath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("Run() code = 0, want unsupported pack failure")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "unsupported pack") {
		t.Fatalf("stderr missing pack explanation: %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outputDir, "content-pack.json")); !os.IsNotExist(err) {
		t.Fatalf("content-pack.json exists after rejected run, stat error = %v", err)
	}
}
```

- [ ] Run:

```bash
go test ./internal/pipeline ./internal/cli ./internal/quality -run 'Pack|ContentPack' -count=1
```

Expected: fails because pipeline options, CLI flag, pack writes, and quality validation do not exist.

- [ ] Add `Pack string` to `pipeline.Options` and `--pack` to CLI with accepted values `""`, `"off"`, and `"auto"`. Default is off to preserve existing behavior.
- [ ] Unsupported `--pack` values must fail during option validation before any bundle files are written.
- [ ] CLI success output for pack runs must name `content-pack.json` and the target brief directory so agents know what to inspect next.
- [ ] Add generated file tracking for:
  - `content-pack.json`
  - `pack/linkedin-dense/brief.md`
  - `pack/social-teaser/brief.md`
  - `pack/blog-og/brief.md`

The cleanup must remove only generated files and empty generated directories. It must preserve unrelated files under `pack/`.

Add rollback tests for these cases:

- stale generated `content-pack.json` is removed after a later no-evidence run
- generated target briefs are removed after pack mode is disabled
- custom files such as `pack/operator-note.md` are preserved
- a failed target image generation keeps successful sibling target images from the same run

- [ ] Write `content-pack.json` with mode `0600`, same as other generated JSON files.
- [ ] Write each brief with mode `0600`. Briefs must include title, target intent, density, aspect ratio, required content, avoid list, source references, and a note that planned local output is not a final polished image.
- [ ] Add manifest output kinds:
  - `content_pack`
  - `pack_brief`

Only add `pack_image` outputs in Task 9 when files exist.

- [ ] Extend quality validation:
  - if `content_pack` is declared, file exists, JSON parses, schema is `content-pack/v1`
  - each declared `pack_brief` path is safe and exists
  - no `pack_image` may be declared for a target still in `planned` or `handoff_ready`
  - target paths must not traverse parents or use backslashes

- [ ] Run:

```bash
go test ./internal/pack ./internal/pipeline ./internal/cli ./internal/quality -run 'Pack|ContentPack|Manifest|Rollback' -count=1
```

Expected: passes.

## Task 8: Codex Pack Handoff And Quick Mode

**Owner:** Worker G.

**Files:**

- Modify: `internal/handoff/handoff.go`
- Modify: `internal/handoff/handoff_test.go`
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add red tests for pack handoff:

```go
func TestRunPackCodexHandoffWritesPackPrompt(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
		Pack:      "auto",
		Quick:     true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	promptPath := filepath.Join(outputDir, "handoff", "content-pack-codex-prompt.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile(pack prompt) error = %v", err)
	}
	for _, want := range []string{"linkedin-dense", "social-teaser", "blog-og", "content-pack.json", "not `visualize --backend codex`"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("pack prompt missing %q:\n%s", want, data)
		}
	}
	if !hasOutputKind(manifest.OutputFiles, "handoff_pack_prompt") {
		t.Fatalf("manifest output files missing pack prompt: %#v", manifest.OutputFiles)
	}
}
```

- [ ] Run:

```bash
go test ./internal/handoff ./internal/pipeline ./internal/cli -run 'Pack.*Handoff|Quick|CodexBackend' -count=1
```

Expected: fails because pack handoff does not exist.

- [ ] Add `handoff.WriteCodexContentPackPackage(outputDir string, packet model.VisualPacket, contentPack pack.ContentPack)`.
- [ ] The pack prompt must instruct Codex to:
  - use `content-pack.json`, `visual-packet.json`, `scaffold.html`, and `pack/<target>/brief.md`
  - generate one image per target only when the user runs the prompt in interactive Codex
  - save images to the target `output_path`
  - preserve required text and avoid invented claims
  - treat private source-derived content as sensitive before using remote tools

- [ ] When `--pack auto --handoff codex` is used, target states become `handoff_ready` and pack status becomes `handoff-ready`.
- [ ] When `--pack auto --handoff codex --quick` is used, CLI prints:

```text
Content pack Codex handoff: <out>/handoff/content-pack-codex-prompt.md
POSIX shell: codex -C '<out>' "$(cat < '<out>/handoff/content-pack-codex-prompt.md')"
```

- [ ] Add tests that `--backend codex` still fails before writing a bundle.
- [ ] Run:

```bash
go test ./internal/handoff ./internal/pipeline ./internal/cli -run 'Pack|Codex|Quick' -count=1
```

Expected: passes.

## Task 9: OpenAI Pack Image Generation And Partial Success

**Owner:** Worker H.

**Files:**

- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/backend/backend.go`
- Modify: `internal/backend/openai.go`
- Modify: `internal/backend/backend_test.go`
- Modify: `internal/quality/quality.go`
- Modify: `internal/quality/quality_test.go`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add a fake backend test for successful pack generation:

```go
func TestRunPackOpenAIGeneratesEachTarget(t *testing.T) {
	fake := &recordingOpenAIBackend{}
	useOpenAIFake(t, fake)
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "openai",
		Renderer:  "image",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(fake.Requests) != 4 {
		t.Fatalf("OpenAI requests = %d, want 4 including primary final.png", len(fake.Requests))
	}
	for _, rel := range []string{"pack/linkedin-dense/final.png", "pack/social-teaser/final.png", "pack/blog-og/final.png"} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("Stat(%s) error = %v", rel, err)
		}
	}
	if !hasOutputKind(manifest.OutputFiles, "pack_image") {
		t.Fatalf("manifest missing pack_image output: %#v", manifest.OutputFiles)
	}
}
```

- [ ] Add a red partial-failure test:

```go
func TestRunPackOpenAIPartialFailureKeepsSuccessfulTargets(t *testing.T) {
	fake := &recordingOpenAIBackend{FailTargetID: "social-teaser"}
	useOpenAIFake(t, fake)
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "openai",
		Renderer:  "image",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want partial pack success", err)
	}
	assertNextStepContains(t, manifest.NextSteps, "partial")
	if _, err := os.Stat(filepath.Join(outputDir, "pack", "linkedin-dense", "final.png")); err != nil {
		t.Fatalf("successful target missing: %v", err)
	}
}
```

- [ ] Run:

```bash
go test ./internal/pipeline ./internal/backend ./internal/quality -run 'Pack.*OpenAI|Partial|BuildPrompt' -count=1
```

Expected: fails because pack image generation is not wired.

- [ ] Reuse the existing OpenAI image backend. Do not add a second remote image backend.
- [ ] Build a target prompt from `visual-packet.json`, `scaffold.html`, and each target brief. The prompt must say:

```text
Create the <target id> image from this source-backed content pack target.
Preserve required text exactly.
Do not invent facts, APIs, papers, numbers, dates, or recommendations.
Make it visually stunning while respecting the target density, intent, and aspect ratio.
```

- [ ] Use target aspect ratio as prompt guidance first. Do not change OpenAI size constants unless official API support is verified in a separate focused change.
- [ ] On explicit `--backend openai --pack auto`, primary image generation failure still fails the run. Individual pack target failures mark only that target as `failed` with a reason and keep successful sibling targets.
- [ ] Add `pack_image` output files only for images that exist. Include SHA256.
- [ ] Mark generated target states as `generated`; mark failed target states as `failed`; overall status becomes `partial` or `complete`.
- [ ] Run:

```bash
go test ./internal/pack ./internal/pipeline ./internal/backend ./internal/quality -run 'Pack|OpenAI|Partial|Manifest' -count=1
```

Expected: passes with fake backend only.

## Task 10: Schema-First LLM Pack Planner

**Owner:** Worker I.

**Files:**

- Modify: `internal/pack/pack.go`
- Modify: `internal/pack/pack_test.go`
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `docs/backends.md`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Add red tests for planner selection:

```go
func TestPackPlannerDefaultsToDeterministic(t *testing.T) {
	opts := normalizeOptions(Options{Pack: "auto"})
	if opts.Planner != "deterministic" {
		t.Fatalf("Planner = %q, want deterministic", opts.Planner)
	}
}

func TestOpenAIPlannerMustReturnSchemaValidPack(t *testing.T) {
	planner := fakePlanner{response: `{"schema_version":"content-pack/v1","source_packet":"visual-packet.json","title":"Sample","status":"planned","strategy":{"planner":"openai","summary":"selected targets"},"targets":[]}`}
	contentPack, err := planner.Plan(context.Background(), sampleVisualPacket(), PlannerOptions{Mode: "openai"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if contentPack.Strategy.Planner != "openai" {
		t.Fatalf("planner = %q, want openai", contentPack.Strategy.Planner)
	}
}
```

- [ ] Run:

```bash
go test ./internal/pack ./internal/pipeline ./internal/cli -run 'Planner|Pack' -count=1
```

Expected: fails because planner selection does not exist.

- [ ] Add `--planner deterministic|openai`, valid only when `--pack auto` is used.
- [ ] Keep deterministic as default and the release-safe path.
- [ ] Add a `Planner` interface in `internal/pack`:

```go
type Planner interface {
	Plan(context.Context, model.VisualPacket, PlannerOptions) (ContentPack, error)
}
```

- [ ] Add an OpenAI planner behind a fakeable seam. It must request JSON matching the same `ContentPack` schema and validate it before writing files.
- [ ] The OpenAI planner prompt must include the current `visual-packet.json`, allowed target IDs, allowed states, and the instruction that the planner may choose target emphasis but may not add factual claims not present in the packet.
- [ ] If the OpenAI planner fails in `--backend auto` or `--backend hybrid`, fall back to deterministic planning with a warning. If the user explicitly selects `--planner openai`, fail clearly.
- [ ] Run:

```bash
go test ./internal/pack ./internal/pipeline ./internal/cli -run 'Planner|Pack|Fallback' -count=1
```

Expected: passes without live OpenAI calls.

## Task 11: Public Docs, Samples, Smoke Tests, And Release Gates

**Owner:** Worker J.

**Files:**

- Modify: `README.md`
- Modify: `docs/backends.md`
- Modify: `docs/agent-workflows.md`
- Modify: `docs/release-readiness.md`
- Modify: `examples/sample-bundle/README.md`
- Modify: `examples/sample-bundle/output/scaffold.html`
- Modify: `examples/sample-bundle/output/visual-packet.json`
- Modify: `examples/sample-bundle/output/manifest.json`
- Create if useful: `examples/sample-bundle/output/content-pack.json`
- Modify: `docs/superpowers/notes/smart-content-packs-and-pdf-source-quality-implementation-notes.html`

- [ ] Update README examples to include:

```bash
visualize --pack auto --out out https://github.com/example/project
visualize --pack auto --handoff codex --quick --out out https://github.com/example/project
visualize --pack auto --backend openai --renderer image --out out ./source
```

- [ ] Document ChatGPT/Codex subscription usage as a handoff path, not a direct CLI backend:

```text
If your Codex environment has image tools through your ChatGPT or Codex subscription, run `visualize --pack auto --handoff codex --quick ...`, then paste or run the generated Codex prompt in interactive Codex. The Go CLI does not consume your ChatGPT subscription directly.
```

- [ ] Update docs to remove the old statement that PDF-only runs always fail.
- [ ] Regenerate sample bundle locally:

```bash
rm -rf /tmp/technical-visualizer-sample
go run ./cmd/visualize --pack auto --backend local --renderer html --out /tmp/technical-visualizer-sample examples/sample-bundle/input/notes.md
```

Expected: command exits 0 and writes `content-pack.json` plus target briefs.

- [ ] Copy text artifacts into `examples/sample-bundle/output/`. Do not commit `final.png` or generated pack PNGs.
- [ ] Run local smoke:

```bash
rm -rf /tmp/technical-visualizer-pack-smoke
go run ./cmd/visualize --pack auto --handoff codex --quick --backend local --renderer html --out /tmp/technical-visualizer-pack-smoke testdata/research-knowledge-base.md
test -s /tmp/technical-visualizer-pack-smoke/content-pack.json
test -s /tmp/technical-visualizer-pack-smoke/pack/linkedin-dense/brief.md
test -s /tmp/technical-visualizer-pack-smoke/pack/social-teaser/brief.md
test -s /tmp/technical-visualizer-pack-smoke/pack/blog-og/brief.md
test -s /tmp/technical-visualizer-pack-smoke/handoff/content-pack-codex-prompt.md
```

Expected: all commands exit 0.

- [ ] Add a local e2e test if the repo still has no equivalent broad smoke test:

```go
func TestVisualizePackAutoLocalE2E(t *testing.T) {
	outputDir := t.TempDir()
	cmd := exec.Command("go", "run", "./cmd/visualize", "--pack", "auto", "--backend", "local", "--renderer", "html", "--offline", "--out", outputDir, "testdata/research-knowledge-base.md")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("visualize pack e2e failed: %v\n%s", err, data)
	}
	for _, rel := range []string{"scaffold.html", "visual-packet.json", "manifest.json", "final.png", "content-pack.json", "pack/linkedin-dense/brief.md", "pack/social-teaser/brief.md", "pack/blog-og/brief.md"} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("Stat(%s) error = %v", rel, err)
		}
	}
}
```

Run:

```bash
go test ./... -run TestVisualizePackAutoLocalE2E -count=1
```

Expected: passes and proves the built CLI path writes the complete local pack bundle.

- [ ] If Poppler is installed, run PDF smoke with a checked-in tiny fixture or a temporary local PDF. If no fixture PDF exists, skip this live smoke and record why:

```bash
command -v pdftotext
go run ./cmd/visualize --backend local --renderer html --out /tmp/technical-visualizer-pdf-smoke testdata/pdf/born-digital.pdf
```

Expected when fixture and Poppler exist: command exits 0 and packet contains `pdf_text` evidence. Expected when absent: worker records skipped reason in implementation notes.

- [ ] Run full gates:

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
git diff --check
```

Expected: every command exits 0.

- [ ] Scan staged changes for secrets before the final commit:

```bash
SECRET_PATTERN='sk-(''proj|live|test|svcacct|admin)-[A-Za-z0-9_-]+|OPENAI_API''_KEY=[A-Za-z0-9_-]+|temporary API key'' supplied|temp api'' key'
git diff --cached | rg -n "$SECRET_PATTERN" || true
```

Expected: no matches.

## Final Acceptance

- [ ] `visualize --pack auto --backend local --renderer html` writes the normal bundle plus `content-pack.json` and target briefs, with no generated pack images claimed.
- [ ] `visualize --pack auto --handoff codex --quick` writes the normal bundle, pack briefs, content pack JSON, single-image Codex handoff, pack-level Codex handoff, and a safe POSIX quick command.
- [ ] `visualize --pack auto --backend openai --renderer image` uses the existing OpenAI image backend for the primary image and each pack target, records per-target state, and allows partial pack target failure without deleting successful sibling targets.
- [ ] PDF-only runs succeed when Poppler extraction produces useful text and fail clearly when no extractor or too little text is available.
- [ ] `manifest.json` includes next steps, source diagnostics for PDF extraction, pack outputs only when files exist, and no absolute local paths.
- [ ] Quality validation accepts valid pack bundles and rejects unsafe paths, missing declared files, invalid content pack schema, and generated image claims for non-generated targets.
- [ ] Docs accurately describe local, OpenAI, Codex handoff, PDF, pack, quick, and planner behavior.
- [ ] `script/lint`, `script/test`, `script/smoke`, `GOTOOLCHAIN=go1.26.3 script/security`, `go test -cover ./...`, and `git diff --check` pass before release.
