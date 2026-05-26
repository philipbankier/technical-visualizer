# Content-Rich Codex Handoff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a content-rich packet and Codex handoff workflow so dense technical sources produce renderer-ready artifacts and a fast manual Codex command.

**Architecture:** Add a deterministic extraction layer between evidence gathering and packet building, extend `VisualPacket` with rich renderer-ready fields, render those fields in `scaffold.html`, and write a local `handoff/` package for Codex. Keep Codex as an agent workflow, not a direct backend, and require strict red-green-refactor for every behavior change.

**Tech Stack:** Go 1.26, standard library markdown/text heuristics, existing `script/*` gates, GitHub Actions, no LLM dependency in extraction.

---

## Guardrails

- Use strict TDD. Do not write production code before a failing test exists and has been run.
- Use a fresh worker subagent per task during execution.
- After each worker task, run a spec-compliance review and a code-quality review before committing.
- Keep generated bundles, local caches, and API keys out of commits.
- Update `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html` in any task that interprets the spec, makes a tradeoff, or discovers a follow-up.
- Do not implement a direct `--backend codex` path.
- Do not include fallback PNG improvement in the first pass unless all higher-priority tasks are complete and reviewed.

## File Structure

- `testdata/research-knowledge-base.md`: Dense markdown fixture modeled on the agent failure case.
- `internal/model/model.go`: Adds rich packet and manifest fields.
- `internal/model/model_test.go`: JSON round-trip coverage for new schema fields.
- `internal/extract/extract.go`: New deterministic extraction layer from `EvidenceBundle` to rich content.
- `internal/extract/extract_test.go`: Fixture-backed extraction tests.
- `internal/packet/builder.go`: Maps extraction output into `VisualPacket`.
- `internal/packet/builder_test.go`: Dense markdown packet behavior tests.
- `internal/render/scaffold.go`: Renders rich packet sections in HTML.
- `internal/render/render_test.go`: Scaffold coverage for rich fields.
- `internal/pipeline/pipeline.go`: Adds handoff options, `next_steps`, and handoff artifact writes.
- `internal/pipeline/pipeline_test.go`: Pipeline behavior tests for manifest and handoff.
- `internal/handoff/handoff.go`: New handoff package writer.
- `internal/handoff/handoff_test.go`: Handoff file tests.
- `internal/cli/cli.go`: Adds `--handoff` and `--quick` flags and quick command output.
- `internal/cli/cli_test.go`: CLI flag and output tests.
- `internal/quality/quality.go`: Validates new optional bundle artifacts.
- `internal/quality/quality_test.go`: Quality checks for handoff files and next steps.
- `README.md`, `docs/agent-workflows.md`, `docs/backends.md`, `docs/release-readiness.md`: Public docs.
- `examples/sample-bundle/`: Regenerate sample excerpt if behavior changes sample output.
- `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`: Running implementation notes.

## Pre-Execution Setup

- [ ] Keep the active Codex Goal scoped to this plan: content-rich packet extraction, scaffold rendering, manifest next steps, Codex handoff artifacts, quick mode, docs, and release gates.
- [ ] Use implementation branch `codex/content-rich-codex-handoff` for the code work:

```bash
git status --short --branch
git switch -c codex/content-rich-codex-handoff
```

- [ ] Use one worker subagent per task. Each worker receives this plan, the design spec, the current implementation notes file, and the exact task section.
- [ ] After each task, run one spec-compliance reviewer and one code-quality reviewer before committing.

## Task 1: Rich Packet Schema And Dense Fixture

**Files:**
- Create: `testdata/research-knowledge-base.md`
- Modify: `internal/model/model.go`
- Modify: `internal/model/model_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Add the dense markdown fixture**

Create `testdata/research-knowledge-base.md` with this exact source:

```markdown
# AI Agent Skills - Research Knowledge Base

## Executive Summary

Agent skills are reusable, composable units of procedural knowledge for AI coding and research agents. The core pattern is to encode a task recipe, required context, tool constraints, and validation gates so agents can perform specialized work without relearning the workflow.

The strongest findings across the corpus are:

- Skill libraries improve repeatability when each skill has a clear trigger, minimal context, and a concrete validation loop.
- SkillOpt reports +23.5 accuracy on held-out tasks after optimizing skill instructions.
- The best operational pattern is a 5-layer skill stack: trigger, context, procedure, tools, and validation.
- Human review remains necessary for high-risk outputs because agents can overfit to examples and omit source uncertainty.

## Core Papers

### 1. Toolformer

- Authors: Schick et al.
- Date: 2023-02
- Identifier: arXiv:2302.04761
- Finding: language models can learn API-use examples from self-supervised traces.
- Relevance: tool-use examples can become reusable procedural memory.

### 2. Voyager

- Authors: Wang et al.
- Date: 2023-05
- Identifier: arXiv:2305.16291
- Finding: an agent can build a skill library from exploration and reuse skills in new tasks.
- Result: solved 52/52 discovered tasks in the Minecraft evaluation.

### 3. Reflexion

- Authors: Shinn et al.
- Date: 2023-03
- Identifier: arXiv:2303.11366
- Finding: verbal reinforcement improves future attempts through compact feedback memory.
- Result: improved pass@1 on coding tasks after 1-4 edits.

### 4. SkillOpt

- Authors: Example Research Lab
- Date: 2025-01
- Identifier: arXiv:2501.01234
- Finding: automatic skill prompt optimization improves agent task success.
- Result: +23.5 accuracy, 85-2K tokens per optimized skill.

## Key Concepts & Taxonomy

| Layer | Purpose | Failure Mode | Validation |
| --- | --- | --- | --- |
| Trigger | Decides when to activate | Over-triggering | Negative examples |
| Context | Loads relevant facts | Context bloat | Token budget check |
| Procedure | Lists ordered actions | Vague steps | Red-green task proof |
| Tools | Names allowed tools | Tool mismatch | Dry run |
| Validation | Defines done criteria | False confidence | Independent review |

## SkillOpt Deep Dive

SkillOpt treats a skill as an optimizable prompt program:

1. collect task traces
2. identify recurring failures
3. mutate skill instructions
4. evaluate on held-out tasks
5. keep revisions with better success and lower token cost

Deep learning analogy:

```text
task traces -> loss signal -> prompt mutation -> validation split -> skill checkpoint
```

ASCII pipeline:

```text
[Tasks] -> [Failures] -> [Skill Mutator] -> [Evaluator] -> [Skill Library]
             ^                                             |
             +---------------- feedback -------------------+
```

## Timeline

- 2023-02: Toolformer shows self-supervised tool-use examples.
- 2023-03: Reflexion shows verbal feedback memory improves coding attempts.
- 2023-05: Voyager demonstrates autonomous skill libraries.
- 2024-09: Production agent teams standardize skill registries.
- 2025-01: SkillOpt reports automated skill prompt optimization.

## Scaling Laws

| Variable | Small | Medium | Large |
| --- | --- | --- | --- |
| Skill length | 85 tokens | 600 tokens | 2K tokens |
| Review burden | 1 reviewer | 2 reviewers | 3 reviewers |
| Failure recovery | manual | semi-automatic | policy-driven |

Key statistic: skill quality improves fastest when review examples stay below 12 per skill and each edit changes one behavior.

## Transfer Results

- Documentation skills transfer well across repos when tool names are abstracted.
- Debugging skills transfer poorly unless environment assumptions are explicit.
- Security skills need stricter validation because false negatives are costly.

## Open Questions

1. How should agents choose between overlapping skills?
2. Can skill quality be measured without task-specific gold labels?
3. What is the best way to decay stale skills?
4. How much source text should a skill cite?
5. Can skill libraries be safely shared across organizations?
6. What review protocol catches hallucinated tool affordances?
7. How should skills expose privacy boundaries?
8. Can optimized skills overfit to benchmark phrasing?
9. What is the minimum metadata for discoverability?
10. How should agents explain why a skill was triggered?
```

- [ ] **Step 2: Write the failing model round-trip test**

Append this test to `internal/model/model_test.go`:

```go
func TestVisualPacketRichContentRoundTrip(t *testing.T) {
	packet := VisualPacket{
		SchemaVersion: "visual-packet/v1",
		ArtifactGoal:  "research-infographic",
		Audience:      "technical decision maker",
		Title:         "AI Agent Skills",
		Thesis:        "Agent skills combine triggers, context, procedures, tools, and validation.",
		RequiredText:  []string{"AI Agent Skills"},
		RankedClaims:  []Claim{{ID: "claim-1", Text: "SkillOpt reports +23.5 accuracy.", Kind: "metric", Confidence: "source-backed", SourceRefs: []string{"src-skillopt"}}},
		ContentBlocks: []ContentBlock{{ID: "block-1", Kind: "section", Title: "Executive Summary", Text: "Agent skills are reusable procedural knowledge.", SourceRefs: []string{"src-summary"}}},
		Metrics:       []Metric{{ID: "metric-1", Label: "SkillOpt accuracy lift", Value: "+23.5", Context: "held-out tasks", SourceRefs: []string{"src-skillopt"}}},
		Timeline:      []TimelineEvent{{ID: "time-1", Date: "2023-02", Label: "Toolformer", Summary: "Self-supervised tool use.", SourceRefs: []string{"src-toolformer"}}},
		Entities:      []Entity{{ID: "entity-1", Kind: "paper", Name: "Voyager", Detail: "Wang et al., arXiv:2305.16291", SourceRefs: []string{"src-voyager"}}},
		Tables:        []PacketTable{{ID: "table-1", Title: "5-layer stack", Headers: []string{"Layer", "Purpose"}, Rows: [][]string{{"Trigger", "Decides when to activate"}}, SourceRefs: []string{"src-taxonomy"}}},
		Diagrams:      []Diagram{{ID: "diagram-1", Title: "SkillOpt pipeline", Kind: "ascii", Text: "[Tasks] -> [Failures]", SourceRefs: []string{"src-diagram"}}},
		OpenQuestions: []OpenQuestion{{ID: "question-1", Text: "How should agents choose between overlapping skills?", SourceRefs: []string{"src-questions"}}},
		Layout:        LayoutSpec{Format: "single-image-infographic", Orientation: "landscape"},
		Style:         StyleSpec{Name: "executive-dark", Renderer: "html"},
	}

	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded VisualPacket
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := decoded.Metrics[0].Value; got != "+23.5" {
		t.Fatalf("metric value = %q, want +23.5", got)
	}
	if got := decoded.Tables[0].Rows[0][0]; got != "Trigger" {
		t.Fatalf("table first cell = %q, want Trigger", got)
	}
	if got := decoded.OpenQuestions[0].Text; got == "" {
		t.Fatalf("open question text empty")
	}
}
```

- [ ] **Step 3: Run the model test to verify RED**

Run:

```bash
go test ./internal/model -run TestVisualPacketRichContentRoundTrip -count=1
```

Expected: FAIL to compile with errors like `undefined: ContentBlock`, `undefined: Metric`, and `decoded.Metrics undefined`.

- [ ] **Step 4: Add minimal rich packet types**

Modify `internal/model/model.go`.

Add fields to `VisualPacket` after `Sections`:

```go
	ContentBlocks []ContentBlock   `json:"content_blocks,omitempty"`
	Metrics       []Metric         `json:"metrics,omitempty"`
	Timeline      []TimelineEvent  `json:"timeline,omitempty"`
	Entities      []Entity         `json:"entities,omitempty"`
	Tables        []PacketTable    `json:"tables,omitempty"`
	Diagrams      []Diagram        `json:"diagrams,omitempty"`
	OpenQuestions []OpenQuestion   `json:"open_questions,omitempty"`
```

Add these types after `Section`:

```go
type ContentBlock struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Text       string   `json:"text,omitempty"`
	Items      []string `json:"items,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Metric struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Value      string   `json:"value"`
	Unit       string   `json:"unit,omitempty"`
	Context    string   `json:"context,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type TimelineEvent struct {
	ID         string   `json:"id"`
	Date       string   `json:"date,omitempty"`
	Label      string   `json:"label"`
	Summary    string   `json:"summary,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Entity struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Detail     string   `json:"detail,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type PacketTable struct {
	ID         string     `json:"id"`
	Title      string     `json:"title,omitempty"`
	Headers    []string   `json:"headers,omitempty"`
	Rows       [][]string `json:"rows,omitempty"`
	SourceRefs []string   `json:"source_refs,omitempty"`
}

type Diagram struct {
	ID         string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	Kind       string   `json:"kind"`
	Text       string   `json:"text"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type OpenQuestion struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Context    string   `json:"context,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}
```

- [ ] **Step 5: Run the model test to verify GREEN**

Run:

```bash
go test ./internal/model -run TestVisualPacketRichContentRoundTrip -count=1
go test ./internal/model -count=1
```

Expected: PASS.

- [ ] **Step 6: Update implementation notes**

Append an entry under `Design Decisions` in `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`:

```html
      <div class="entry">
        <div class="meta">Task 1</div>
        <p>
          Rich packet fields were added as optional JSON fields so existing v0.1 packet consumers
          continue to work while richer renderers can opt into structured content.
        </p>
      </div>
```

- [ ] **Step 7: Commit Task 1**

Run:

```bash
gofmt -w internal/model/model.go internal/model/model_test.go
git add testdata/research-knowledge-base.md internal/model/model.go internal/model/model_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: add rich packet schema"
```

## Task 2: Deterministic Markdown Extraction

**Files:**
- Create: `internal/extract/extract.go`
- Create: `internal/extract/extract_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write the failing extraction test**

Create `internal/extract/extract_test.go`:

```go
package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestFromEvidenceExtractsDenseResearchMarkdown(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "research-knowledge-base.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:    "src-research",
			Kind:  model.SourceMarkdown,
			Input: "research-knowledge-base.md",
		}},
		Items: []model.EvidenceItem{{
			ID:       "src-research-doc",
			SourceID: "src-research",
			Kind:     "markdown",
			Title:    "research-knowledge-base.md",
			Text:     string(data),
			Path:     "research-knowledge-base.md",
		}},
	}

	result := FromEvidence(bundle)

	assertMetric(t, result.Metrics, "+23.5")
	assertMetric(t, result.Metrics, "52/52")
	assertEntity(t, result.Entities, "Toolformer")
	assertEntity(t, result.Entities, "SkillOpt")
	assertTimeline(t, result.Timeline, "2023-02")
	assertTable(t, result.Tables, "Layer", "Trigger")
	assertDiagram(t, result.Diagrams, "[Tasks] -> [Failures]")
	assertQuestion(t, result.OpenQuestions, "overlapping skills")
	if len(result.ContentBlocks) < 8 {
		t.Fatalf("ContentBlocks length = %d, want at least 8", len(result.ContentBlocks))
	}
}

func assertMetric(t *testing.T, metrics []model.Metric, value string) {
	t.Helper()
	for _, metric := range metrics {
		if metric.Value == value {
			return
		}
	}
	t.Fatalf("missing metric value %q in %#v", value, metrics)
}

func assertEntity(t *testing.T, entities []model.Entity, name string) {
	t.Helper()
	for _, entity := range entities {
		if strings.Contains(entity.Name, name) {
			return
		}
	}
	t.Fatalf("missing entity %q in %#v", name, entities)
}

func assertTimeline(t *testing.T, timeline []model.TimelineEvent, date string) {
	t.Helper()
	for _, event := range timeline {
		if event.Date == date {
			return
		}
	}
	t.Fatalf("missing timeline date %q in %#v", date, timeline)
}

func assertTable(t *testing.T, tables []model.PacketTable, header string, cell string) {
	t.Helper()
	for _, table := range tables {
		if containsString(table.Headers, header) {
			for _, row := range table.Rows {
				if containsString(row, cell) {
					return
				}
			}
		}
	}
	t.Fatalf("missing table header %q cell %q in %#v", header, cell, tables)
}

func assertDiagram(t *testing.T, diagrams []model.Diagram, text string) {
	t.Helper()
	for _, diagram := range diagrams {
		if strings.Contains(diagram.Text, text) {
			return
		}
	}
	t.Fatalf("missing diagram text %q in %#v", text, diagrams)
}

func assertQuestion(t *testing.T, questions []model.OpenQuestion, text string) {
	t.Helper()
	for _, question := range questions {
		if strings.Contains(strings.ToLower(question.Text), strings.ToLower(text)) {
			return
		}
	}
	t.Fatalf("missing open question %q in %#v", text, questions)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the extraction test to verify RED**

Run:

```bash
go test ./internal/extract -run TestFromEvidenceExtractsDenseResearchMarkdown -count=1
```

Expected: FAIL because package `internal/extract` has no non-test Go files or `FromEvidence` is undefined.

- [ ] **Step 3: Add minimal extraction implementation**

Create `internal/extract/extract.go`:

```go
package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type Result struct {
	ContentBlocks []model.ContentBlock
	Metrics       []model.Metric
	Timeline      []model.TimelineEvent
	Entities      []model.Entity
	Tables        []model.PacketTable
	Diagrams      []model.Diagram
	OpenQuestions []model.OpenQuestion
}

var (
	metricPattern   = regexp.MustCompile(`(?:\+\d+(?:\.\d+)?|\d+/\d+|\d+\s*-\s*\d+\s+edits|\d+\s*-\s*\d+K?\s+tokens|[0-9]+K?\s+tokens)`)
	dateLinePattern = regexp.MustCompile(`^[-*]\s+(\d{4}(?:-\d{2})?):\s*(.+)$`)
)

func FromEvidence(bundle model.EvidenceBundle) Result {
	var result Result
	for _, item := range bundle.Items {
		refs := sourceRefsForItem(item)
		sections := splitMarkdownSections(item.Text)
		for _, section := range sections {
			result.ContentBlocks = append(result.ContentBlocks, contentBlock(section, refs))
			result.Metrics = append(result.Metrics, metricsFromSection(section, refs)...)
			result.Timeline = append(result.Timeline, timelineFromSection(section, refs)...)
			result.Entities = append(result.Entities, entitiesFromSection(section, refs)...)
			result.Tables = append(result.Tables, tablesFromSection(section, refs)...)
			result.Diagrams = append(result.Diagrams, diagramsFromSection(section, refs)...)
			result.OpenQuestions = append(result.OpenQuestions, questionsFromSection(section, refs)...)
		}
	}
	return dedupe(result)
}

type section struct {
	Title string
	Body  []string
}

func splitMarkdownSections(text string) []section {
	var sections []section
	current := section{Title: "Document"}
	for _, line := range strings.Split(text, "\n") {
		if title, ok := markdownHeading(line); ok {
			if current.Title != "" || len(current.Body) > 0 {
				sections = append(sections, current)
			}
			current = section{Title: title}
			continue
		}
		current.Body = append(current.Body, line)
	}
	if current.Title != "" || len(current.Body) > 0 {
		sections = append(sections, current)
	}
	return sections
}

func markdownHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimLeft(trimmed, "#")), true
}

func contentBlock(section section, refs []string) model.ContentBlock {
	return model.ContentBlock{
		ID:         stableLocalID("block", section.Title),
		Kind:       "section",
		Title:      section.Title,
		Summary:    firstSentence(section.Body),
		Text:       trimText(strings.Join(section.Body, "\n"), 1200),
		Items:      bullets(section.Body),
		SourceRefs: refs,
	}
}

func metricsFromSection(section section, refs []string) []model.Metric {
	text := strings.Join(section.Body, " ")
	matches := metricPattern.FindAllString(text, -1)
	metrics := make([]model.Metric, 0, len(matches))
	for _, match := range matches {
		value := strings.Join(strings.Fields(match), " ")
		metrics = append(metrics, model.Metric{
			ID:         stableLocalID("metric", section.Title, value),
			Label:      section.Title,
			Value:      value,
			Context:    sentenceContaining(text, value),
			SourceRefs: refs,
		})
	}
	return metrics
}

func timelineFromSection(section section, refs []string) []model.TimelineEvent {
	var events []model.TimelineEvent
	for _, line := range section.Body {
		match := dateLinePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 3 {
			continue
		}
		events = append(events, model.TimelineEvent{
			ID:         stableLocalID("time", match[1], match[2]),
			Date:       match[1],
			Label:      firstColonPart(match[2]),
			Summary:    strings.TrimSpace(match[2]),
			SourceRefs: refs,
		})
	}
	return events
}

func entitiesFromSection(section section, refs []string) []model.Entity {
	if !strings.Contains(strings.ToLower(section.Title), "paper") && !looksNumberedPaper(section.Title) {
		return nil
	}
	name := strings.TrimSpace(strings.TrimLeft(section.Title, "0123456789. "))
	if name == "" || strings.EqualFold(name, "Core Papers") {
		return nil
	}
	return []model.Entity{{
		ID:         stableLocalID("entity", name),
		Kind:       "paper",
		Name:       name,
		Detail:     trimText(strings.Join(nonEmptyLines(section.Body), "; "), 260),
		SourceRefs: refs,
	}}
}

func tablesFromSection(section section, refs []string) []model.PacketTable {
	lines := nonEmptyLines(section.Body)
	var tables []model.PacketTable
	for i := 0; i+1 < len(lines); i++ {
		if !isTableRow(lines[i]) || !isTableSeparator(lines[i+1]) {
			continue
		}
		headers := splitTableRow(lines[i])
		var rows [][]string
		for j := i + 2; j < len(lines) && isTableRow(lines[j]); j++ {
			rows = append(rows, splitTableRow(lines[j]))
			i = j
		}
		tables = append(tables, model.PacketTable{
			ID:         stableLocalID("table", section.Title, strings.Join(headers, "|")),
			Title:      section.Title,
			Headers:    headers,
			Rows:       rows,
			SourceRefs: refs,
		})
	}
	return tables
}

func diagramsFromSection(section section, refs []string) []model.Diagram {
	var diagrams []model.Diagram
	inFence := false
	var fenced []string
	for _, line := range section.Body {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence && looksDiagram(strings.Join(fenced, "\n")) {
				text := strings.Join(fenced, "\n")
				diagrams = append(diagrams, model.Diagram{ID: stableLocalID("diagram", section.Title, text), Title: section.Title, Kind: "ascii", Text: text, SourceRefs: refs})
			}
			inFence = !inFence
			fenced = nil
			continue
		}
		if inFence {
			fenced = append(fenced, line)
		}
	}
	return diagrams
}

func questionsFromSection(section section, refs []string) []model.OpenQuestion {
	if !strings.Contains(strings.ToLower(section.Title), "question") {
		return nil
	}
	var questions []model.OpenQuestion
	for _, line := range section.Body {
		text := strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if strings.Contains(text, "?") {
			questions = append(questions, model.OpenQuestion{ID: stableLocalID("question", text), Text: text, Context: section.Title, SourceRefs: refs})
		}
	}
	return questions
}

func sourceRefsForItem(item model.EvidenceItem) []string {
	if len(item.SourceRefs) > 0 {
		return append([]string(nil), item.SourceRefs...)
	}
	if item.ID != "" {
		return []string{item.ID}
	}
	return nil
}

func bullets(lines []string) []string {
	var items []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			items = append(items, strings.TrimSpace(trimmed[2:]))
		}
		if len(items) == 8 {
			break
		}
	}
	return items
}

func nonEmptyLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func firstSentence(lines []string) string {
	text := strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
	if text == "" {
		return ""
	}
	for i, r := range text {
		if r == '.' || r == '?' || r == '!' {
			return strings.TrimSpace(text[:i+len(string(r))])
		}
	}
	return trimText(text, 180)
}

func sentenceContaining(text string, value string) string {
	for _, part := range strings.Split(text, ".") {
		if strings.Contains(part, value) {
			return strings.TrimSpace(part) + "."
		}
	}
	return ""
}

func firstColonPart(text string) string {
	parts := strings.SplitN(text, ":", 2)
	return strings.TrimSpace(parts[0])
}

func looksNumberedPaper(title string) bool {
	return regexp.MustCompile(`^\d+\.\s+\S+`).MatchString(strings.TrimSpace(title))
}

func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|") && strings.HasSuffix(strings.TrimSpace(line), "|")
}

func isTableSeparator(line string) bool {
	trimmed := strings.Trim(line, "| ")
	return trimmed != "" && strings.Trim(trimmed, "-: |") == ""
}

func splitTableRow(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func looksDiagram(text string) bool {
	return strings.Contains(text, "->") || strings.Contains(text, "+---") || strings.Contains(text, "[") && strings.Contains(text, "]")
}

func trimText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:limit]))
}

func stableLocalID(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "-" + hex.EncodeToString(sum[:])[:12]
}

func dedupe(result Result) Result {
	result.Metrics = dedupeMetrics(result.Metrics)
	result.Entities = dedupeEntities(result.Entities)
	return result
}

func dedupeMetrics(metrics []model.Metric) []model.Metric {
	seen := map[string]bool{}
	var out []model.Metric
	for _, metric := range metrics {
		key := metric.Value + "\x00" + metric.Context
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, metric)
	}
	return out
}

func dedupeEntities(entities []model.Entity) []model.Entity {
	seen := map[string]bool{}
	var out []model.Entity
	for _, entity := range entities {
		key := strings.ToLower(entity.Kind + "\x00" + entity.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entity)
	}
	return out
}
```

- [ ] **Step 4: Run extraction tests to verify GREEN**

Run:

```bash
gofmt -w internal/extract/extract.go internal/extract/extract_test.go
go test ./internal/extract -run TestFromEvidenceExtractsDenseResearchMarkdown -count=1
go test ./internal/extract -count=1
```

Expected: PASS.

- [ ] **Step 5: Update implementation notes**

Append this entry under `Tradeoffs`:

```html
      <div class="entry">
        <div class="meta">Task 2</div>
        <p>
          Markdown extraction is heuristic and deterministic. It preserves excerpts and simple
          structures instead of trying to perfectly understand every source document.
        </p>
      </div>
```

- [ ] **Step 6: Commit Task 2**

Run:

```bash
git add internal/extract/extract.go internal/extract/extract_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: extract rich markdown content"
```

## Task 3: Packet Builder Uses Rich Extraction

**Files:**
- Modify: `internal/packet/builder.go`
- Modify: `internal/packet/builder_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write the failing packet behavior test**

Append to `internal/packet/builder_test.go`:

```go
func TestBuildPreservesDenseResearchContent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "research-knowledge-base.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:    "src-research",
			Kind:  model.SourceMarkdown,
			Input: "research-knowledge-base.md",
		}},
		Items: []model.EvidenceItem{{
			ID:       "src-research-doc",
			SourceID: "src-research",
			Kind:     "markdown",
			Title:    "research-knowledge-base.md",
			Text:     string(data),
			Path:     "research-knowledge-base.md",
		}},
	}

	packet, err := Build(bundle, BuildOptions{Goal: "research-infographic", Style: "executive-dark", Renderer: "html"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(packet.RankedClaims) < 3 {
		t.Fatalf("RankedClaims length = %d, want at least 3", len(packet.RankedClaims))
	}
	if !packetHasMetric(packet, "+23.5") || !packetHasMetric(packet, "52/52") {
		t.Fatalf("packet metrics missing expected values: %#v", packet.Metrics)
	}
	if !packetHasEntity(packet, "SkillOpt") || !packetHasEntity(packet, "Voyager") {
		t.Fatalf("packet entities missing expected papers: %#v", packet.Entities)
	}
	if len(packet.Tables) == 0 || len(packet.Timeline) == 0 || len(packet.OpenQuestions) == 0 || len(packet.Diagrams) == 0 {
		t.Fatalf("packet missing rich fields: tables=%d timeline=%d questions=%d diagrams=%d", len(packet.Tables), len(packet.Timeline), len(packet.OpenQuestions), len(packet.Diagrams))
	}
	for _, claim := range packet.RankedClaims {
		if strings.Contains(claim.Text, "Table of Contents: 1.") {
			t.Fatalf("claim relies on table of contents instead of source content: %#v", claim)
		}
	}
}

func packetHasMetric(packet model.VisualPacket, value string) bool {
	for _, metric := range packet.Metrics {
		if metric.Value == value {
			return true
		}
	}
	return false
}

func packetHasEntity(packet model.VisualPacket, name string) bool {
	for _, entity := range packet.Entities {
		if strings.Contains(entity.Name, name) {
			return true
		}
	}
	return false
}
```

Also add imports if missing:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)
```

- [ ] **Step 2: Run packet test to verify RED**

Run:

```bash
go test ./internal/packet -run TestBuildPreservesDenseResearchContent -count=1
```

Expected: FAIL because `packet.Metrics`, `packet.Tables`, `packet.Timeline`, `packet.OpenQuestions`, and `packet.Diagrams` are empty.

- [ ] **Step 3: Integrate extraction into packet builder**

Modify `internal/packet/builder.go`.

Add import:

```go
	"github.com/philipbankier/technical-visualizer/internal/extract"
```

Inside `Build`, after `labels := sourceLabels(primaryName, bundle)`, add:

```go
	extracted := extract.FromEvidence(bundle)
```

Set the new fields in `model.VisualPacket`:

```go
		ContentBlocks: extracted.ContentBlocks,
		Metrics:       extracted.Metrics,
		Timeline:      extracted.Timeline,
		Entities:      extracted.Entities,
		Tables:        extracted.Tables,
		Diagrams:      extracted.Diagrams,
		OpenQuestions: extracted.OpenQuestions,
```

Replace `RankedClaims: rankedClaims(primaryName, bundle),` with:

```go
		RankedClaims: rankedClaims(primaryName, bundle, extracted),
```

Change the function signature and start of `rankedClaims`:

```go
func rankedClaims(primaryName string, bundle model.EvidenceBundle, extracted extract.Result) []model.Claim {
	claims := make([]model.Claim, 0, 6)
	for _, metric := range extracted.Metrics {
		claims = append(claims, model.Claim{
			ID:         "claim-" + strconv.Itoa(len(claims)+1),
			Text:       ensureSentence(metric.Label + " reports " + metric.Value + contextSuffix(metric.Context)),
			Kind:       "metric",
			Confidence: "source-backed",
			SourceRefs: metric.SourceRefs,
		})
		if len(claims) == 3 {
			break
		}
	}
```

Then keep the existing item-based claim loop below it until `len(claims) == 6`.

Add helper:

```go
func contextSuffix(context string) string {
	context = strings.TrimSpace(context)
	if context == "" {
		return ""
	}
	return " in " + strings.TrimSuffix(context, ".")
}
```

- [ ] **Step 4: Run packet tests to verify GREEN**

Run:

```bash
gofmt -w internal/packet/builder.go internal/packet/builder_test.go
go test ./internal/packet -run TestBuildPreservesDenseResearchContent -count=1
go test ./internal/packet -count=1
```

Expected: PASS.

- [ ] **Step 5: Update implementation notes**

Append under `Design Decisions`:

```html
      <div class="entry">
        <div class="meta">Task 3</div>
        <p>
          The packet builder prioritizes metric-backed claims before generic evidence claims.
          This prevents dense documents from collapsing into table-of-contents snippets.
        </p>
      </div>
```

- [ ] **Step 6: Commit Task 3**

Run:

```bash
git add internal/packet/builder.go internal/packet/builder_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: build packets from rich extraction"
```

## Task 4: Scaffold Renders Rich Packet Content

**Files:**
- Modify: `internal/render/scaffold.go`
- Modify: `internal/render/render_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write failing scaffold test**

Append to `internal/render/render_test.go`:

```go
func TestWriteScaffoldRendersRichPacketFields(t *testing.T) {
	packet := scaffoldTestPacket()
	packet.Metrics = []model.Metric{{ID: "metric-1", Label: "SkillOpt accuracy lift", Value: "+23.5", Context: "held-out tasks"}}
	packet.Timeline = []model.TimelineEvent{{ID: "time-1", Date: "2023-02", Label: "Toolformer", Summary: "Self-supervised tool use"}}
	packet.Tables = []model.PacketTable{{ID: "table-1", Title: "5-layer stack", Headers: []string{"Layer", "Purpose"}, Rows: [][]string{{"Trigger", "Decides when to activate"}}}}
	packet.Diagrams = []model.Diagram{{ID: "diagram-1", Title: "SkillOpt pipeline", Kind: "ascii", Text: "[Tasks] -> [Failures]"}}
	packet.OpenQuestions = []model.OpenQuestion{{ID: "question-1", Text: "How should agents choose between overlapping skills?"}}
	packet.ContentBlocks = []model.ContentBlock{{ID: "block-1", Kind: "section", Title: "Executive Summary", Summary: "Agent skills are reusable procedural knowledge."}}

	var buf bytes.Buffer
	if err := WriteScaffold(&buf, packet); err != nil {
		t.Fatalf("WriteScaffold() error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"Key Metrics",
		"SkillOpt accuracy lift",
		"+23.5",
		"Timeline",
		"2023-02",
		"Tables",
		"Trigger",
		"Diagrams",
		"[Tasks] -&gt; [Failures]",
		"Open Questions",
		"overlapping skills",
		"Executive Summary",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("scaffold missing %q in: %s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run scaffold test to verify RED**

Run:

```bash
go test ./internal/render -run TestWriteScaffoldRendersRichPacketFields -count=1
```

Expected: FAIL because the scaffold does not render rich sections.

- [ ] **Step 3: Add rich sections to scaffold template**

Modify `internal/render/scaffold.go`.

Insert after the Required Text section:

```html
    {{if .Metrics}}
    <section>
      <h2>Key Metrics</h2>
      <ul>
{{range .Metrics}}        <li><span class="text">{{.Label}}: {{.Value}}</span><span class="label">{{.Context}}</span></li>
{{end}}
      </ul>
    </section>
    {{end}}

    {{if .Timeline}}
    <section>
      <h2>Timeline</h2>
      <ul>
{{range .Timeline}}        <li><span class="text">{{.Date}} - {{.Label}}</span><span class="label">{{.Summary}}</span></li>
{{end}}
      </ul>
    </section>
    {{end}}

    {{if .Tables}}
    <section>
      <h2>Tables</h2>
{{range .Tables}}      <h3>{{.Title}}</h3>
      <ul>
{{range .Rows}}        <li>{{range .}}{{.}} {{end}}</li>
{{end}}      </ul>
{{end}}
    </section>
    {{end}}

    {{if .Diagrams}}
    <section>
      <h2>Diagrams</h2>
{{range .Diagrams}}      <h3>{{.Title}}</h3>
      <pre>{{.Text}}</pre>
{{end}}
    </section>
    {{end}}

    {{if .OpenQuestions}}
    <section>
      <h2>Open Questions</h2>
      <ul>
{{range .OpenQuestions}}        <li>{{.Text}}</li>
{{end}}
      </ul>
    </section>
    {{end}}

    {{if .ContentBlocks}}
    <section>
      <h2>Content Blocks</h2>
      <ul>
{{range .ContentBlocks}}        <li><span class="text">{{.Title}}</span><span class="label">{{.Summary}}</span></li>
{{end}}
      </ul>
    </section>
    {{end}}
```

Add CSS for `pre` and `h3` inside the existing style block:

```css
    h3 {
      margin: 16px 0 8px;
      font-size: 16px;
      letter-spacing: 0;
      color: #d6e1e8;
    }
    pre {
      overflow-x: auto;
      padding: 14px;
      border: 1px solid #2c3942;
      border-radius: 6px;
      background: #101418;
      color: #d6e1e8;
      white-space: pre-wrap;
    }
```

- [ ] **Step 4: Run render tests to verify GREEN**

Run:

```bash
gofmt -w internal/render/scaffold.go internal/render/render_test.go
go test ./internal/render -run TestWriteScaffoldRendersRichPacketFields -count=1
go test ./internal/render -count=1
```

Expected: PASS.

- [ ] **Step 5: Update implementation notes**

Append under `Design Decisions`:

```html
      <div class="entry">
        <div class="meta">Task 4</div>
        <p>
          Scaffold HTML remains static and script-free, but now acts as an infographic outline
          for rich packet fields rather than only an audit list.
        </p>
      </div>
```

- [ ] **Step 6: Commit Task 4**

Run:

```bash
git add internal/render/scaffold.go internal/render/render_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: render rich scaffold sections"
```

## Task 5: Manifest Next Steps

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/model/model_test.go`
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/quality/quality.go`
- Modify: `internal/quality/quality_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write failing manifest model test**

Append to `internal/model/model_test.go`:

```go
func TestManifestNextStepsRoundTrip(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "manifest/v1",
		Backend:       BackendInfo{Name: "local"},
		Renderer:      "html",
		Style:         "executive-dark",
		Audit:         ManifestAudit{ToolVersion: "0.1.0"},
		OutputFiles:   []OutputFile{{Kind: "manifest", Path: "manifest.json"}},
		NextSteps:     []string{"Open scaffold.html first.", "Rerun with --handoff codex for Codex handoff."},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(decoded.NextSteps) != 2 {
		t.Fatalf("NextSteps length = %d, want 2", len(decoded.NextSteps))
	}
}
```

- [ ] **Step 2: Run manifest model test to verify RED**

Run:

```bash
go test ./internal/model -run TestManifestNextStepsRoundTrip -count=1
```

Expected: FAIL to compile because `Manifest.NextSteps` is undefined.

- [ ] **Step 3: Add manifest schema field**

Add to `model.Manifest` after `Warnings`:

```go
	NextSteps    []string      `json:"next_steps,omitempty"`
```

- [ ] **Step 4: Run manifest model test to verify GREEN**

Run:

```bash
gofmt -w internal/model/model.go internal/model/model_test.go
go test ./internal/model -run TestManifestNextStepsRoundTrip -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing pipeline next-steps tests**

Append to `internal/pipeline/pipeline_test.go`:

```go
func TestRunAddsLocalNextSteps(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "notes.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertNextStepContains(t, manifest.NextSteps, "scaffold.html")
	assertNextStepContains(t, manifest.NextSteps, "--handoff codex")
	assertNextStepContains(t, manifest.NextSteps, "--backend openai --renderer image")
}

func TestRunAddsOfflineNextStep(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "notes.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Offline:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertNextStepContains(t, manifest.NextSteps, "offline")
}

func assertNextStepContains(t *testing.T, steps []string, want string) {
	t.Helper()
	for _, step := range steps {
		if strings.Contains(step, want) {
			return
		}
	}
	t.Fatalf("next_steps missing %q in %#v", want, steps)
}
```

- [ ] **Step 6: Run pipeline next-steps tests to verify RED**

Run:

```bash
go test ./internal/pipeline -run 'TestRunAdds(LocalNextSteps|OfflineNextStep)' -count=1
```

Expected: FAIL because `manifest.NextSteps` is empty.

- [ ] **Step 7: Implement contextual next steps**

Add a field to `imageGenerationResult`:

```go
	HandoffWritten bool
```

Set `NextSteps` in the manifest:

```go
		NextSteps:    nextSteps(opts, imageResult),
```

Add helper:

```go
func nextSteps(opts Options, imageResult imageGenerationResult) []string {
	var steps []string
	if imageResult.BackendInfo.Name == "local" {
		steps = append(steps, "Open scaffold.html first; it is the primary local artifact for this run.")
		steps = append(steps, "For a polished OpenAI image, set OPENAI_API_KEY and rerun with --backend openai --renderer image.")
		if imageResult.HandoffWritten {
			steps = append(steps, "For Codex image generation, open handoff/codex-prompt.md and run it in interactive Codex.")
		} else {
			steps = append(steps, "For a guided Codex package, rerun with --handoff codex.")
		}
	}
	if imageResult.BackendInfo.Name == "openai" {
		steps = append(steps, "Inspect final.png for the generated image and manifest.json for source and backend audit details.")
	}
	if imageResult.FallbackUsed {
		steps = append(steps, "The run used a local fallback; use explicit --backend openai or --handoff codex for a polished image path.")
	}
	if opts.Offline {
		steps = append(steps, "Offline mode was enabled; no remote source fetching or remote image generation was performed.")
	}
	return steps
}
```

- [ ] **Step 8: Write failing quality test for manifest next steps**

In `internal/quality/quality_test.go`, update `validManifest` to include a valid next-step fixture:

```go
		NextSteps: []string{"Open scaffold.html first."},
```

Append this test:

```go
func TestValidateBundleRequiresManifestNextSteps(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><title>Acme Map</title><body>Acme Map</body>")
	manifest := validManifest(dir)
	manifest.NextSteps = nil
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "next_steps") {
		t.Fatalf("ValidateBundle issues = %#v, want next_steps issue", issues)
	}
}
```

- [ ] **Step 9: Run quality test to verify RED**

Run:

```bash
go test ./internal/quality -run TestValidateBundleRequiresManifestNextSteps -count=1
```

Expected: FAIL because quality validation does not check `next_steps`.

- [ ] **Step 10: Implement quality validation for next steps**

In `internal/quality/quality.go`, extend the manifest summary type with:

```go
	NextSteps []string `json:"next_steps"`
```

Add validation after warnings/backend checks:

```go
	if len(manifest.NextSteps) == 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "next_steps must not be empty"})
	}
```

- [ ] **Step 11: Run tests to verify GREEN**

Run:

```bash
gofmt -w internal/model/model.go internal/model/model_test.go internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go internal/quality/quality.go internal/quality/quality_test.go
go test ./internal/quality -run TestValidateBundleRequiresManifestNextSteps -count=1
go test ./internal/model ./internal/pipeline ./internal/quality -count=1
```

Expected: PASS.

- [ ] **Step 12: Update implementation notes**

Append under `Design Decisions`:

```html
      <div class="entry">
        <div class="meta">Task 5</div>
        <p>
          Manifest next steps are treated as a required generated-bundle quality signal so agents
          are not left to infer what to do from backend metadata alone.
        </p>
      </div>
```

- [ ] **Step 13: Commit Task 5**

Run:

```bash
git add internal/model/model.go internal/model/model_test.go internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go internal/quality/quality.go internal/quality/quality_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: add manifest next steps"
```

## Task 6: Codex Handoff Artifact Writer

**Files:**
- Create: `internal/handoff/handoff.go`
- Create: `internal/handoff/handoff_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write failing handoff writer test**

Create `internal/handoff/handoff_test.go`:

```go
package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestWriteCodexPackageWritesPromptBriefChecklistAndStyle(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		Title:        "AI Agent Skills",
		Thesis:       "Skills improve repeatability.",
		RequiredText: []string{"AI Agent Skills"},
		RankedClaims: []model.Claim{{ID: "claim-1", Text: "SkillOpt reports +23.5 accuracy.", SourceRefs: []string{"src-skillopt"}}},
		Metrics:      []model.Metric{{ID: "metric-1", Label: "SkillOpt accuracy lift", Value: "+23.5"}},
		OpenQuestions: []model.OpenQuestion{{
			ID:   "question-1",
			Text: "How should agents choose between overlapping skills?",
		}},
		Style: model.StyleSpec{Name: "executive-dark", Renderer: "html"},
	}

	result, err := WriteCodexPackage(dir, packet, Options{OutputImagePath: "final.png"})
	if err != nil {
		t.Fatalf("WriteCodexPackage() error = %v", err)
	}
	for _, path := range []string{result.PromptPath, result.BriefPath, result.ChecklistPath, result.StylePath} {
		info, err := os.Stat(filepath.Join(dir, path))
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %o, want 0600", path, info.Mode().Perm())
		}
	}
	prompt := readFile(t, filepath.Join(dir, result.PromptPath))
	for _, want := range []string{"visual-packet.json", "scaffold.html", "final.png", "SkillOpt reports +23.5 accuracy", "Do not claim `visualize --backend codex` exists"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q in: %s", want, prompt)
		}
	}
	brief := readFile(t, filepath.Join(dir, result.BriefPath))
	if !strings.Contains(brief, "+23.5") || !strings.Contains(brief, "overlapping skills") {
		t.Fatalf("brief missing packet content: %s", brief)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
```

- [ ] **Step 2: Run handoff test to verify RED**

Run:

```bash
go test ./internal/handoff -run TestWriteCodexPackageWritesPromptBriefChecklistAndStyle -count=1
```

Expected: FAIL because package has no implementation or `WriteCodexPackage` is undefined.

- [ ] **Step 3: Add handoff writer implementation**

Create `internal/handoff/handoff.go`:

```go
package handoff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type Options struct {
	OutputImagePath string
}

type Result struct {
	PromptPath    string
	BriefPath     string
	ChecklistPath string
	StylePath     string
}

func WriteCodexPackage(outputDir string, packet model.VisualPacket, opts Options) (Result, error) {
	if opts.OutputImagePath == "" {
		opts.OutputImagePath = "final.png"
	}
	result := Result{
		PromptPath:    filepath.ToSlash(filepath.Join("handoff", "codex-prompt.md")),
		BriefPath:     filepath.ToSlash(filepath.Join("handoff", "image-brief.md")),
		ChecklistPath: filepath.ToSlash(filepath.Join("handoff", "qa-checklist.md")),
		StylePath:     filepath.ToSlash(filepath.Join("handoff", "style.md")),
	}
	files := map[string]string{
		result.PromptPath:    promptMarkdown(packet, opts),
		result.BriefPath:     briefMarkdown(packet),
		result.ChecklistPath: checklistMarkdown(opts),
		result.StylePath:     styleMarkdown(packet),
	}
	for rel, content := range files {
		path := filepath.Join(outputDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func promptMarkdown(packet model.VisualPacket, opts Options) string {
	return strings.Join([]string{
		"# Codex Image Generation Prompt",
		"",
		"Use `visual-packet.json`, `scaffold.html`, `handoff/image-brief.md`, and `handoff/qa-checklist.md` as source material.",
		"Generate one polished single-image technical infographic and save it as `" + opts.OutputImagePath + "`.",
		"",
		"Do not claim `visualize --backend codex` exists. This is an agent handoff workflow.",
		"Preserve required text exactly. Do not invent facts. Show uncertainty visibly.",
		"",
		"Title: " + packet.Title,
		"Thesis: " + packet.Thesis,
		"",
		"Priority claims:",
		bullets(claimTexts(packet.RankedClaims)),
		"",
		"Key metrics:",
		bullets(metricTexts(packet.Metrics)),
	}, "\n")
}

func briefMarkdown(packet model.VisualPacket) string {
	return strings.Join([]string{
		"# Image Brief",
		"",
		"Title: " + packet.Title,
		"",
		packet.Thesis,
		"",
		"Required text:",
		bullets(packet.RequiredText),
		"",
		"Open questions:",
		bullets(questionTexts(packet.OpenQuestions)),
	}, "\n")
}

func checklistMarkdown(opts Options) string {
	return strings.Join([]string{
		"# QA Checklist",
		"",
		"- Required text is readable.",
		"- Factual claims are source-backed by the packet.",
		"- No invented APIs, numbers, papers, or dates were added.",
		"- Uncertainty and open questions are visible.",
		"- Final image is saved at `" + opts.OutputImagePath + "`.",
	}, "\n")
}

func styleMarkdown(packet model.VisualPacket) string {
	if packet.Style.Name == "executive-dark" || packet.Style.Name == "" {
		return "# Style\n\nUse a premium dark technical infographic style with strong hierarchy, crisp readable text, restrained cards, and vibrant but harmonious accents.\n"
	}
	return fmt.Sprintf("# Style\n\nUse the `%s` style profile. Keep the result source-backed, readable, and polished.\n", packet.Style.Name)
}

func claimTexts(claims []model.Claim) []string {
	out := make([]string, 0, len(claims))
	for _, claim := range claims {
		out = append(out, claim.Text)
	}
	return out
}

func metricTexts(metrics []model.Metric) []string {
	out := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, metric.Label+": "+metric.Value+" "+metric.Context)
	}
	return out
}

func questionTexts(questions []model.OpenQuestion) []string {
	out := make([]string, 0, len(questions))
	for _, question := range questions {
		out = append(out, question.Text)
	}
	return out
}

func bullets(items []string) string {
	if len(items) == 0 {
		return "- None provided."
	}
	var lines []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			lines = append(lines, "- "+item)
		}
	}
	if len(lines) == 0 {
		return "- None provided."
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Run handoff tests to verify GREEN**

Run:

```bash
gofmt -w internal/handoff/handoff.go internal/handoff/handoff_test.go
go test ./internal/handoff -count=1
```

Expected: PASS.

- [ ] **Step 5: Update implementation notes**

Append under `Design Decisions`:

```html
      <div class="entry">
        <div class="meta">Task 6</div>
        <p>
          Codex handoff files are deterministic local markdown files with private permissions.
          They make the agent workflow first-class without adding a direct Codex backend.
        </p>
      </div>
```

- [ ] **Step 6: Commit Task 6**

Run:

```bash
git add internal/handoff/handoff.go internal/handoff/handoff_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: write codex handoff package"
```

## Task 7: Pipeline And CLI Handoff Integration

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/quality/quality.go`
- Modify: `internal/quality/quality_test.go`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write failing pipeline handoff test**

Append to `internal/pipeline/pipeline_test.go`:

```go
func TestRunWritesCodexHandoffPackage(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, rel := range []string{"handoff/codex-prompt.md", "handoff/image-brief.md", "handoff/qa-checklist.md", "handoff/style.md"} {
		if _, err := os.Stat(filepath.Join(outputDir, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
	assertNextStepContains(t, manifest.NextSteps, "handoff/codex-prompt.md")
}

func TestRunRejectsQuickWithoutCodexHandoff(t *testing.T) {
	outputDir := t.TempDir()
	_, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "notes.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Quick:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "--quick requires --handoff codex") {
		t.Fatalf("Run() error = %v, want quick handoff error", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(statErr) {
		t.Fatalf("scaffold.html exists after rejected quick run, stat error = %v", statErr)
	}
}
```

- [ ] **Step 2: Run pipeline handoff tests to verify RED**

Run:

```bash
go test ./internal/pipeline -run 'TestRun(WritesCodexHandoffPackage|RejectsQuickWithoutCodexHandoff)' -count=1
```

Expected: FAIL to compile because `Options.Handoff` and `Options.Quick` are undefined.

- [ ] **Step 3: Add pipeline options and validation**

Add to `pipeline.Options`:

```go
	Handoff   string
	Quick     bool
```

In `normalizeOptions`, normalize handoff:

```go
	opts.Handoff = strings.ToLower(strings.TrimSpace(opts.Handoff))
```

In `validateOptions`, before backend validation returns:

```go
	switch opts.Handoff {
	case "", "codex":
	default:
		return fmt.Errorf("unsupported handoff %q", opts.Handoff)
	}
	if opts.Quick && opts.Handoff != "codex" {
		return errors.New("--quick requires --handoff codex")
	}
```

- [ ] **Step 4: Write handoff files from pipeline**

Import:

```go
	"github.com/philipbankier/technical-visualizer/internal/handoff"
```

After `generateImage`, add:

```go
	if opts.Handoff == "codex" {
		if _, err := handoff.WriteCodexPackage(opts.OutputDir, visualPacket, handoff.Options{OutputImagePath: "final.png"}); err != nil {
			return model.Manifest{}, err
		}
		imageResult.HandoffWritten = true
	}
```

Ensure `nextSteps` includes `handoff/codex-prompt.md` when `HandoffWritten` is true:

```go
steps = append(steps, "For Codex image generation, open handoff/codex-prompt.md and run it in interactive Codex.")
```

- [ ] **Step 5: Run pipeline handoff tests to verify GREEN**

Run:

```bash
gofmt -w internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go
go test ./internal/pipeline -run 'TestRun(WritesCodexHandoffPackage|RejectsQuickWithoutCodexHandoff)' -count=1
go test ./internal/pipeline -count=1
```

Expected: PASS.

- [ ] **Step 6: Write failing CLI quick output test**

Append to `internal/cli/cli_test.go`:

```go
func TestRunQuickCodexPrintsManualCommand(t *testing.T) {
	outputDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"--backend", "local",
		"--renderer", "html",
		"--handoff", "codex",
		"--quick",
		"--out", outputDir,
		filepath.Join("..", "..", "testdata", "research-knowledge-base.md"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"Codex handoff:", "codex -C", "handoff/codex-prompt.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q in: %s", want, got)
		}
	}
}
```

- [ ] **Step 7: Run CLI quick test to verify RED**

Run:

```bash
go test ./internal/cli -run TestRunQuickCodexPrintsManualCommand -count=1
```

Expected: FAIL because CLI does not parse `--handoff` or `--quick`.

- [ ] **Step 8: Add CLI flags and quick command output**

In `parsePipelineArgs`, add:

```go
	fs.StringVar(&opts.Handoff, "handoff", "", "handoff package: codex")
	fs.BoolVar(&opts.Quick, "quick", false, "print a ready manual command for the selected handoff")
```

In `printUsage`, add:

```text
  --handoff       Optional handoff package, currently codex.
  --quick         Print a ready manual command for the selected handoff.
```

In `runPipeline`, after backend output:

```go
	if opts.Handoff == "codex" {
		fmt.Fprintf(stdout, "Codex handoff: %s\n", filepath.Join(opts.OutputDir, "handoff", "codex-prompt.md"))
	}
	if opts.Handoff == "codex" && opts.Quick {
		fmt.Fprintf(stdout, "Run: codex -C %s \"$(cat handoff/codex-prompt.md)\"\n", opts.OutputDir)
	}
```

Add `path/filepath` to CLI imports.

- [ ] **Step 9: Run CLI tests to verify GREEN**

Run:

```bash
gofmt -w internal/cli/cli.go internal/cli/cli_test.go
go test ./internal/cli -run TestRunQuickCodexPrintsManualCommand -count=1
go test ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 10: Extend quality validation for handoff files**

Write a failing quality test first in `internal/quality/quality_test.go`:

```go
func TestValidateBundleReportsMissingDeclaredCodexHandoff(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.NextSteps = []string{"Open handoff/codex-prompt.md."}
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "handoff/codex-prompt.md", "no such file") {
		t.Fatalf("ValidateBundle issues = %#v, want missing handoff prompt", issues)
	}
}
```

Run:

```bash
go test ./internal/quality -run TestValidateBundleReportsMissingDeclaredCodexHandoff -count=1
```

Expected: FAIL because quality validation does not check handoff files.

Then implement in `internal/quality/quality.go`: if any next step contains `handoff/codex-prompt.md`, require these files to exist:

```go
for _, rel := range []string{"handoff/codex-prompt.md", "handoff/image-brief.md", "handoff/qa-checklist.md", "handoff/style.md"} {
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		issues = append(issues, Issue{Path: rel, Message: err.Error()})
	}
}
```

Run:

```bash
gofmt -w internal/quality/quality.go internal/quality/quality_test.go
go test ./internal/quality -run TestValidateBundleReportsMissingDeclaredCodexHandoff -count=1
go test ./internal/quality -count=1
```

Expected: PASS.

- [ ] **Step 11: Update implementation notes**

Append under `Design Decisions`:

```html
      <div class="entry">
        <div class="meta">Task 7</div>
        <p>
          Quick mode prints a manual interactive Codex command and never runs Codex itself. This
          preserves the boundary between the Go CLI and Codex's agent-side image tools.
        </p>
      </div>
```

- [ ] **Step 12: Commit Task 7**

Run:

```bash
git add internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go internal/cli/cli.go internal/cli/cli_test.go internal/quality/quality.go internal/quality/quality_test.go docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "feat: add codex handoff mode"
```

## Task 8: Docs, Examples, And Sample Bundle

**Files:**
- Modify: `README.md`
- Modify: `docs/agent-workflows.md`
- Modify: `docs/backends.md`
- Modify: `docs/release-readiness.md`
- Modify: `examples/sample-bundle/README.md`
- Modify: `examples/sample-bundle/output/scaffold.html`
- Modify: `examples/sample-bundle/output/visual-packet.json`
- Modify: `examples/sample-bundle/output/manifest.json`
- Modify: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Write docs scan command before edits**

Run this scan before doc edits:

```bash
rg -n "visualize --backend codex|--backend codex works|subscription.*pay|subscription.*OPENAI_API_KEY" README.md docs examples || true
```

Expected: existing matches, if any, must be negative or boundary wording only.

- [ ] **Step 2: Update README usage**

Add a section after `Codex And ChatGPT Subscriptions`:

```markdown
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

Quick mode prints a command you can run in interactive Codex. The CLI does not run Codex for you and does not support `--backend codex`.
```

- [ ] **Step 3: Update agent workflow docs**

In `docs/agent-workflows.md`, replace the old handoff command with:

```markdown
visualize --backend local --renderer html --handoff codex --quick --out visualize-output https://github.com/example/service
```

Add:

```markdown
Use the printed command in an interactive Codex session. If quick mode is not used, open `visualize-output/handoff/codex-prompt.md` and paste it into Codex manually.
```

- [ ] **Step 4: Update backend docs**

In `docs/backends.md`, keep Codex out of backend list and add:

```markdown
`--handoff codex` is not a backend. It writes local prompt and brief files that an interactive Codex session can use.
```

- [ ] **Step 5: Update release readiness docs**

In `docs/release-readiness.md`, add acceptance statements:

```markdown
- Dense markdown sources produce rich packet fields for metrics, tables, timelines, diagrams, entities, and open questions.
- Codex handoff files are generated locally with `--handoff codex`.
- Quick mode prints a manual command and does not run Codex automatically.
```

- [ ] **Step 6: Regenerate sample bundle excerpt**

Run:

```bash
rm -rf examples/sample-bundle/output
go run ./cmd/visualize --backend local --renderer html --handoff codex --out examples/sample-bundle/output examples/sample-bundle/input/notes.md
rm -f examples/sample-bundle/output/final.png
tmp=$(mktemp)
jq '(.output_files) |= map(select(.kind != "image"))' examples/sample-bundle/output/manifest.json > "$tmp"
mv "$tmp" examples/sample-bundle/output/manifest.json
rm -rf examples/sample-bundle/output/handoff
```

Expected: sample excerpt includes scaffold, visual packet, and manifest only. Handoff files are documented but not checked in unless explicitly chosen.

- [ ] **Step 7: Update sample README**

Add:

```markdown
The sample is a pruned excerpt. A full `--handoff codex` run also writes `output/handoff/`, but those generated prompt files are omitted from this checked-in sample to keep the example compact.
```

- [ ] **Step 8: Run docs and sample validation**

Run:

```bash
jq empty examples/sample-bundle/output/manifest.json examples/sample-bundle/output/visual-packet.json
test ! -e examples/sample-bundle/output/final.png
test ! -d examples/sample-bundle/output/handoff
rg -n "visualize --backend codex|--backend codex works|subscription.*pay|subscription.*OPENAI_API_KEY" README.md docs examples || true
git diff --check
```

Expected: JSON parse succeeds, generated binary/handoff folders are absent from sample excerpt, scan matches are only negative/boundary wording, and diff check passes.

- [ ] **Step 9: Update implementation notes**

Append under `Design Decisions`:

```html
      <div class="entry">
        <div class="meta">Task 8</div>
        <p>
          Public docs present Codex handoff as a generated local package plus a manual quick
          command, not as a direct backend or automatic Codex execution.
        </p>
      </div>
```

- [ ] **Step 10: Commit Task 8**

Run:

```bash
git add README.md docs/agent-workflows.md docs/backends.md docs/release-readiness.md examples/sample-bundle/README.md examples/sample-bundle/output/scaffold.html examples/sample-bundle/output/visual-packet.json examples/sample-bundle/output/manifest.json docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "docs: document codex handoff workflow"
```

## Task 9: Agent Usability Review And Final Validation

**Files:**
- Modify only if review findings require fixes.
- Update: `docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html`

- [ ] **Step 1: Generate a dense fixture bundle**

Run:

```bash
rm -rf /tmp/technical-visualizer-rich-smoke
go run ./cmd/visualize --backend local --renderer html --handoff codex --quick --out /tmp/technical-visualizer-rich-smoke testdata/research-knowledge-base.md
```

Expected:

- Command exits 0.
- Output includes `Codex handoff:`.
- Output includes `codex -C /tmp/technical-visualizer-rich-smoke`.

- [ ] **Step 2: Inspect generated artifact presence**

Run:

```bash
test -s /tmp/technical-visualizer-rich-smoke/visual-packet.json
test -s /tmp/technical-visualizer-rich-smoke/scaffold.html
test -s /tmp/technical-visualizer-rich-smoke/manifest.json
test -s /tmp/technical-visualizer-rich-smoke/handoff/codex-prompt.md
test -s /tmp/technical-visualizer-rich-smoke/handoff/image-brief.md
test -s /tmp/technical-visualizer-rich-smoke/handoff/qa-checklist.md
test -s /tmp/technical-visualizer-rich-smoke/handoff/style.md
jq '{metrics, timeline, entities, tables, diagrams, open_questions}' /tmp/technical-visualizer-rich-smoke/visual-packet.json
jq '{next_steps}' /tmp/technical-visualizer-rich-smoke/manifest.json
```

Expected: fields contain dense research content.

- [ ] **Step 3: Run full local gates**

Run:

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
```

Expected: all pass.

- [ ] **Step 4: Run agent usability review**

Spawn a reviewer subagent with this prompt:

```text
You are reviewing the technical-visualizer Codex handoff output in /tmp/technical-visualizer-rich-smoke. Do not open testdata/research-knowledge-base.md or any original source file. Use only visual-packet.json, scaffold.html, manifest.json, and handoff/*.md. Report whether you can identify the core papers, key statistics, taxonomy, timeline, diagrams, and open questions well enough to plan a polished infographic. Return APPROVED if sufficient, otherwise CHANGES_REQUIRED with exact missing content.
```

Expected: APPROVED.

- [ ] **Step 5: Handle review findings through TDD**

If the reviewer returns `CHANGES_REQUIRED`, write a failing test for each missing content class before changing production code. Use the narrow package command first, then rerun the full gates.

- [ ] **Step 6: Final branch audit**

Run:

```bash
git status --short --branch
rg -n "sk-""(proj|live|test|svcacct|admin)-[A-Za-z0-9_-]+|temporary API key"" supplied|temp api"" key|--backend codex"" works|subscription.*""pays" .
rg -n "TO""DO|TB""D|FIX""ME" README.md docs internal testdata examples -g '!docs/superpowers/plans/**' -g '!docs/superpowers/notes/**'
```

Expected: clean worktree except intentional changes before commit; scans have no actionable matches.

- [ ] **Step 7: Update implementation notes**

Append under `Validation Evidence`:

```html
      <div class="entry">
        <div class="meta">Task 9</div>
        <p>
          Final validation generated a dense markdown Codex handoff bundle, passed local gates,
          and passed agent usability review using only generated artifacts.
        </p>
      </div>
```

- [ ] **Step 8: Commit final validation notes if changed**

Run:

```bash
git add docs/superpowers/notes/content-rich-codex-handoff-implementation-notes.html
git commit -m "docs: record codex handoff validation"
```

Only run this commit if Task 9 changed the notes file after the previous commit.

- [ ] **Step 9: Push and PR**

Run:

```bash
git push -u origin codex/content-rich-codex-handoff
gh pr create \
  --repo philipbankier/technical-visualizer \
  --base main \
  --head codex/content-rich-codex-handoff \
  --title "feat: add content-rich codex handoff" \
  --body "This PR adds source-rich visual packets and a Codex handoff package for local infographic generation workflows."
```

Expected: PR opens against `main`. After creation, open the PR URL in the default browser.

## Final Review Checklist

- [ ] Every behavior change had a failing test first.
- [ ] Dense markdown fixture proves the original failure mode is fixed.
- [ ] `visual-packet.json` carries metrics, entities, tables, timeline, diagrams, and open questions.
- [ ] `scaffold.html` renders rich fields.
- [ ] `manifest.json` includes contextual `next_steps`.
- [ ] `--handoff codex` writes all handoff files.
- [ ] `--quick` prints a manual command and does not run Codex.
- [ ] Docs do not claim `--backend codex` works.
- [ ] Local and offline privacy behavior remains unchanged.
- [ ] Full local gates and GitHub CI pass.
