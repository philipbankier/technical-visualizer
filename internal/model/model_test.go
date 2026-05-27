package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVisualPacketRoundTrip(t *testing.T) {
	packet := VisualPacket{
		SchemaVersion: "visual-packet/v1",
		ArtifactGoal:  "architecture-map",
		Audience:      "technical decision maker",
		Title:         "Acme System Map",
		Thesis:        "Acme is a modular service with clear ingestion and rendering boundaries.",
		RequiredText:  []string{"Acme System Map", "Ingestion", "Renderer"},
		RankedClaims: []Claim{
			{ID: "claim-1", Text: "The renderer is isolated from source gathering.", Kind: "insight", Confidence: "high", SourceRefs: []string{"src:readme"}},
		},
		Style: StyleSpec{Name: "executive-dark", Renderer: "hybrid"},
	}

	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded VisualPacket
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.SchemaVersion != "visual-packet/v1" {
		t.Fatalf("SchemaVersion = %q", decoded.SchemaVersion)
	}
	if decoded.RankedClaims[0].SourceRefs[0] != "src:readme" {
		t.Fatalf("SourceRefs = %#v", decoded.RankedClaims[0].SourceRefs)
	}
}

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

func TestManifestRecordsOutputFiles(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "manifest/v1",
		Backend:       BackendInfo{Name: "local", Remote: false},
		Renderer:      "html",
		Style:         "analytic",
		OutputFiles:   []OutputFile{{Kind: "scaffold", Path: "scaffold.html", SHA256: "abc"}},
	}
	if manifest.OutputFiles[0].Kind != "scaffold" {
		t.Fatalf("Output kind = %q", manifest.OutputFiles[0].Kind)
	}
}

func TestManifestNextStepsRoundTrip(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "manifest/v1",
		Backend:       BackendInfo{Name: "local"},
		Renderer:      "html",
		Style:         "executive-dark",
		Audit:         ManifestAudit{ToolVersion: "0.1.0"},
		OutputFiles:   []OutputFile{{Kind: "manifest", Path: "manifest.json"}},
		NextSteps:     []string{"Open scaffold.html first.", "Use visual-packet.json for agent handoff."},
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

func TestManifestAuditFieldsMarshal(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "manifest/v1",
		Sources:       []SourceSpec{{ID: "src-abc", Kind: SourceMarkdown, Input: "notes.md"}},
		Backend:       BackendInfo{Name: "local", Remote: false},
		Renderer:      "html",
		Style:         "analytic",
		Audit: ManifestAudit{
			ToolVersion:             "0.1.0",
			RequestedBackend:        "auto",
			SelectedBackend:         "local",
			SourceCount:             1,
			EvidenceItemCount:       2,
			WarningCount:            1,
			RedactionCount:          0,
			RemoteImageAttempted:    true,
			FallbackUsed:            true,
			PromptTruncated:         true,
			PromptTruncationMessage: "prompt truncated to fit gpt-image-2 prompt limit",
		},
		OutputFiles: []OutputFile{{Kind: "manifest", Path: "manifest.json"}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, want := range []string{`"audit"`, `"tool_version":"0.1.0"`, `"requested_backend":"auto"`, `"selected_backend":"local"`, `"source_count":1`, `"evidence_item_count":2`, `"warning_count":1`, `"remote_image_attempted":true`, `"fallback_used":true`, `"prompt_truncated":true`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("manifest JSON missing %s: %s", want, data)
		}
	}
}
