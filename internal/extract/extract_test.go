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
	if metric := findMetric(t, result.Metrics, "+23.5"); !strings.Contains(metric.Context, "+23.5 accuracy") {
		t.Fatalf("metric context = %q, want decimal value preserved", metric.Context)
	}
	assertEntity(t, result.Entities, "Toolformer")
	assertEntity(t, result.Entities, "SkillOpt")
	assertTimeline(t, result.Timeline, "2023-02")
	assertTable(t, result.Tables, "Layer", "Trigger")
	assertDiagram(t, result.Diagrams, "[Tasks] -> [Failures]")
	assertQuestion(t, result.OpenQuestions, "overlapping skills")
	if len(result.ContentBlocks) < 8 {
		t.Fatalf("ContentBlocks length = %d, want at least 8", len(result.ContentBlocks))
	}
	assertContentDerivedIDs(t)
}

func TestFromEvidenceIgnoresHeadingsInsideFencedBlocks(t *testing.T) {
	result := FromEvidence(model.EvidenceBundle{
		Items: []model.EvidenceItem{{
			ID: "src-fenced",
			Text: strings.Join([]string{
				"# Diagram Source",
				"",
				"```text",
				"# Not A Heading",
				"[A] -> [B]",
				"```",
				"",
				"## Real Section",
				"Actual content.",
			}, "\n"),
		}},
	})

	for _, block := range result.ContentBlocks {
		if block.Title == "Not A Heading" {
			t.Fatalf("fenced heading became content block: %#v", result.ContentBlocks)
		}
	}
	assertDiagram(t, result.Diagrams, "[A] -> [B]")
}

func TestFromEvidenceKeepsDuplicateSourceProvenance(t *testing.T) {
	source := strings.Join([]string{
		"# Summary",
		"SkillOpt reports +23.5 accuracy.",
		"",
		"### 1. Toolformer",
		"- Finding: reusable tool traces.",
	}, "\n")
	result := FromEvidence(model.EvidenceBundle{
		Items: []model.EvidenceItem{
			{ID: "src-one", Text: source},
			{ID: "src-two", Text: source},
		},
	})

	assertUniqueContentBlockIDs(t, result.ContentBlocks)
	metric := findMetric(t, result.Metrics, "+23.5")
	assertSourceRefs(t, metric.SourceRefs, "src-one", "src-two")
	entity := findEntity(t, result.Entities, "Toolformer")
	assertSourceRefs(t, entity.SourceRefs, "src-one", "src-two")
}

func TestFromEvidencePreservesNumericLeadingEntityName(t *testing.T) {
	result := FromEvidence(model.EvidenceBundle{
		Items: []model.EvidenceItem{{
			ID: "src-3d",
			Text: strings.Join([]string{
				"# Papers",
				"",
				"### 1. 3D Skill",
				"- Finding: spatial skill representation.",
			}, "\n"),
		}},
	})

	assertEntity(t, result.Entities, "3D Skill")
}

func TestFromEvidenceFallsBackToSourceIDRefs(t *testing.T) {
	result := FromEvidence(model.EvidenceBundle{
		Items: []model.EvidenceItem{{
			SourceID: "src-only",
			Text:     "# Summary\nSkillOpt reports +23.5 accuracy.",
		}},
	})

	metric := findMetric(t, result.Metrics, "+23.5")
	assertSourceRefs(t, metric.SourceRefs, "src-only")
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

func findMetric(t *testing.T, metrics []model.Metric, value string) model.Metric {
	t.Helper()
	for _, metric := range metrics {
		if metric.Value == value {
			return metric
		}
	}
	t.Fatalf("missing metric value %q in %#v", value, metrics)
	return model.Metric{}
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

func findEntity(t *testing.T, entities []model.Entity, name string) model.Entity {
	t.Helper()
	for _, entity := range entities {
		if strings.Contains(entity.Name, name) {
			return entity
		}
	}
	t.Fatalf("missing entity %q in %#v", name, entities)
	return model.Entity{}
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

func assertContentDerivedIDs(t *testing.T) {
	t.Helper()
	result := FromEvidence(model.EvidenceBundle{
		Items: []model.EvidenceItem{{
			ID:   "id-probe",
			Text: "# Alpha\nsame\n\n# Bravo\nsame",
		}},
	})
	if len(result.ContentBlocks) != 2 {
		t.Fatalf("ContentBlocks length = %d, want 2", len(result.ContentBlocks))
	}
	if result.ContentBlocks[0].ID == result.ContentBlocks[1].ID {
		t.Fatalf("same-length sections got matching IDs %q", result.ContentBlocks[0].ID)
	}
}

func assertUniqueContentBlockIDs(t *testing.T, blocks []model.ContentBlock) {
	t.Helper()
	seen := map[string]bool{}
	for _, block := range blocks {
		if seen[block.ID] {
			t.Fatalf("duplicate content block ID %q in %#v", block.ID, blocks)
		}
		seen[block.ID] = true
	}
}

func assertSourceRefs(t *testing.T, refs []string, want ...string) {
	t.Helper()
	for _, ref := range want {
		if !containsString(refs, ref) {
			t.Fatalf("source refs = %#v, want %q", refs, ref)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
