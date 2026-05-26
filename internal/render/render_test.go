package render

import (
	"bytes"
	"html"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestWriteScaffoldEscapesAndIncludesPacketContent(t *testing.T) {
	packet := scaffoldTestPacket()

	var buf bytes.Buffer
	if err := WriteScaffold(&buf, packet); err != nil {
		t.Fatalf("WriteScaffold() error = %v", err)
	}
	got := buf.String()

	if !strings.Contains(strings.ToLower(got), "<!doctype html>") {
		t.Fatalf("scaffold missing document doctype: %s", got)
	}
	if !strings.Contains(strings.ToLower(got), "<html") || !strings.Contains(strings.ToLower(got), "</html>") {
		t.Fatalf("scaffold missing standalone html wrapper: %s", got)
	}
	if strings.Contains(got, "<script>") || strings.Contains(got, "</script>") {
		t.Fatalf("scaffold contains executable script markup: %s", got)
	}

	assertEscaped(t, got, packet.Title)
	assertEscaped(t, got, packet.Thesis)
	assertEscaped(t, got, packet.RequiredText[0])
	assertEscaped(t, got, packet.RequiredText[1])
	assertEscaped(t, got, packet.RankedClaims[0].Text)
	assertEscaped(t, got, packet.RankedClaims[0].SourceRefs[0])
	assertEscaped(t, got, packet.Facts[0].Text)
	assertEscaped(t, got, packet.Risks[0].Text)
	assertEscaped(t, got, packet.Tradeoffs[0].Choice)
	assertEscaped(t, got, packet.Tradeoffs[0].Reason)
	assertEscaped(t, got, packet.Unknowns[0].Text)
	assertEscaped(t, got, packet.SourceRefs[0].Label)
	assertEscaped(t, got, packet.SourceRefs[0].Locator)
	assertEscaped(t, got, packet.Style.Name)
	assertEscaped(t, got, packet.Style.Renderer)
}

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
		"&#43;23.5",
		"Timeline",
		"2023-02",
		"Tables",
		"<table>",
		"<th>Layer</th>",
		"<th>Purpose</th>",
		"<td>Trigger</td>",
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

func TestWriteScaffoldDoesNotEmitTrailingWhitespace(t *testing.T) {
	packet := scaffoldTestPacket()
	packet.Metrics = nil
	packet.Timeline = nil
	packet.Tables = nil
	packet.Diagrams = nil
	packet.OpenQuestions = nil
	packet.ContentBlocks = nil

	var buf bytes.Buffer
	if err := WriteScaffold(&buf, packet); err != nil {
		t.Fatalf("WriteScaffold() error = %v", err)
	}
	for lineNumber, line := range strings.Split(buf.String(), "\n") {
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			t.Fatalf("line %d has trailing whitespace: %q", lineNumber+1, line)
		}
	}
}

func TestWriteScaffoldPreservesDiagramTrailingWhitespace(t *testing.T) {
	packet := scaffoldTestPacket()
	packet.Diagrams = []model.Diagram{{ID: "diagram-1", Title: "Aligned diagram", Kind: "ascii", Text: "left   \nright\t"}}

	var buf bytes.Buffer
	if err := WriteScaffold(&buf, packet); err != nil {
		t.Fatalf("WriteScaffold() error = %v", err)
	}
	got := buf.String()
	want := "<pre>left   \nright\t</pre>"
	if !strings.Contains(got, want) {
		t.Fatalf("scaffold did not preserve diagram whitespace %q in: %s", want, got)
	}
}

func TestWriteScaffoldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "scaffold.html")

	if err := WriteScaffoldFile(path, scaffoldTestPacket()); err != nil {
		t.Fatalf("WriteScaffoldFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), html.EscapeString(scaffoldTestPacket().Title)) {
		t.Fatalf("scaffold file missing escaped title: %s", data)
	}
}

func TestWriteFallbackPNGCreatesExpectedImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "final.png")

	if err := WriteFallbackPNG(path, scaffoldTestPacket()); err != nil {
		t.Fatalf("WriteFallbackPNG() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Fatalf("fallback png has zero dimensions: %v", bounds)
	}
	if bounds.Dx() != 1600 || bounds.Dy() != 900 {
		t.Fatalf("fallback png size = %dx%d, want 1600x900", bounds.Dx(), bounds.Dy())
	}
}

func assertEscaped(t *testing.T, doc string, value string) {
	t.Helper()

	escaped := html.EscapeString(value)
	if !strings.Contains(doc, escaped) {
		t.Fatalf("scaffold missing escaped value %q as %q in: %s", value, escaped, doc)
	}
}

func scaffoldTestPacket() model.VisualPacket {
	return model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		ArtifactGoal:  "architecture-map",
		Audience:      "technical decision maker",
		Title:         `<script>alert("x")</script> Acme System Map`,
		Thesis:        `Acme <Renderer> preserves source-backed evidence & uncertainty.`,
		RequiredText:  []string{`Renderer <must> appear`, `Source & claims`},
		RankedClaims: []model.Claim{{
			ID:         "claim-1",
			Text:       `Renderer isolates <source> gathering.`,
			Kind:       "insight",
			Confidence: "high",
			SourceRefs: []string{`src:readme<1>`},
		}},
		Facts: []model.Fact{{
			ID:         "fact-1",
			Text:       `Fact & evidence <safe>.`,
			SourceRefs: []string{`src:readme<1>`},
		}},
		Risks: []model.Risk{{
			ID:         "risk-1",
			Text:       `Secret <env> skipped.`,
			Severity:   "high",
			SourceRefs: []string{`src:readme<1>`},
		}},
		Tradeoffs: []model.Tradeoff{{
			ID:         "tradeoff-1",
			Choice:     `Local fallback <PNG>`,
			Reason:     `No remote calls & deterministic output.`,
			SourceRefs: []string{`src:readme<1>`},
		}},
		Unknowns: []model.Unknown{{
			ID:         "unknown-1",
			Text:       `PDF extraction <unknown>.`,
			SourceRefs: []string{`src:readme<1>`},
		}},
		Layout: model.LayoutSpec{
			Format:      "single-image-infographic",
			Orientation: "landscape",
			Regions:     []string{"title", "claims", "risks", "sources"},
		},
		Style: model.StyleSpec{Name: "executive-dark", Renderer: "html"},
		SourceRefs: []model.SourceRef{{
			ID:       `src:readme<1>`,
			SourceID: "src-acme",
			Label:    `README <docs>`,
			Locator:  `README.md#L1`,
		}},
	}
}
