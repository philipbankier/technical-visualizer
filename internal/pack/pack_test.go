package pack

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

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
	if contentPack.Strategy.Planner != "deterministic" {
		t.Fatalf("Strategy.Planner = %q, want deterministic", contentPack.Strategy.Planner)
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

func TestPlanAutoTargetFields(t *testing.T) {
	contentPack, err := Plan(sampleVisualPacket(), Options{Mode: "auto"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := map[string]TargetSpec{
		"linkedin-dense": {
			Platform:    "linkedin",
			Intent:      "technical deep-dive infographic",
			Density:     "high",
			AspectRatio: "16:9",
			BriefPath:   "pack/linkedin-dense/brief.md",
			OutputPath:  "pack/linkedin-dense/final.png",
		},
		"social-teaser": {
			Platform:    "social",
			Intent:      "pretty lower-density social preview",
			Density:     "low",
			AspectRatio: "1:1",
			BriefPath:   "pack/social-teaser/brief.md",
			OutputPath:  "pack/social-teaser/final.png",
		},
		"blog-og": {
			Platform:    "blog",
			Intent:      "article and README open graph hero",
			Density:     "medium",
			AspectRatio: "1.91:1",
			BriefPath:   "pack/blog-og/brief.md",
			OutputPath:  "pack/blog-og/final.png",
		},
	}
	for _, target := range contentPack.Targets {
		expected := want[target.ID]
		if target.Platform != expected.Platform || target.Intent != expected.Intent || target.Density != expected.Density || target.AspectRatio != expected.AspectRatio {
			t.Fatalf("target %s metadata = %#v, want %#v", target.ID, target, expected)
		}
		if target.BriefPath != expected.BriefPath || target.OutputPath != expected.OutputPath {
			t.Fatalf("target %s paths = %q %q, want %q %q", target.ID, target.BriefPath, target.OutputPath, expected.BriefPath, expected.OutputPath)
		}
		if target.State != StatePlanned {
			t.Fatalf("target %s State = %q, want planned", target.ID, target.State)
		}
	}
}

func TestPlanRejectsUnsupportedMode(t *testing.T) {
	_, err := Plan(sampleVisualPacket(), Options{Mode: "custom"})
	if err == nil {
		t.Fatalf("Plan() error = nil, want unsupported mode error")
	}
}

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name    string
		targets []TargetSpec
		want    Status
	}{
		{
			name:    "all planned",
			targets: []TargetSpec{{State: StatePlanned}, {State: StatePlanned}},
			want:    StatusPlanned,
		},
		{
			name:    "handoff ready",
			targets: []TargetSpec{{State: StatePlanned}, {State: StateHandoffReady}},
			want:    StatusHandoffReady,
		},
		{
			name:    "partial",
			targets: []TargetSpec{{State: StateGenerated}, {State: StateHandoffReady}},
			want:    StatusPartial,
		},
		{
			name:    "failed",
			targets: []TargetSpec{{State: StateFailed}, {State: StatePlanned}},
			want:    StatusPartial,
		},
		{
			name:    "empty target state",
			targets: []TargetSpec{{State: ""}},
			want:    StatusPartial,
		},
		{
			name:    "unknown target state",
			targets: []TargetSpec{{State: TargetState("archived")}},
			want:    StatusPartial,
		},
		{
			name:    "complete",
			targets: []TargetSpec{{State: StateGenerated}, {State: StateVerified}},
			want:    StatusComplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveStatus(tt.targets); got != tt.want {
				t.Fatalf("DeriveStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContentPackJSONSnippets(t *testing.T) {
	contentPack := ContentPack{
		SchemaVersion: "content-pack/v1",
		SourcePacket:  "visual-packet.json",
		Title:         "Technical Map: SkillOpt",
		Status:        StatusHandoffReady,
		Strategy: Strategy{
			Planner: "deterministic",
			Summary: "Deterministic pack plan.",
		},
		Targets: []TargetSpec{{
			ID:              "linkedin-dense",
			Platform:        "linkedin",
			Intent:          "technical deep-dive infographic",
			Density:         "high",
			AspectRatio:     "16:9",
			BriefPath:       "pack/linkedin-dense/brief.md",
			OutputPath:      "pack/linkedin-dense/final.png",
			State:           StateHandoffReady,
			FailureReason:   "manual handoff pending",
			RequiredContent: []string{"title", "SkillOpt reports +23.5 accuracy."},
			Avoid:           []string{"inventing unsupported metrics"},
		}},
	}
	data, err := json.MarshalIndent(contentPack, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	jsonText := string(data)
	for _, want := range []string{
		`"schema_version": "content-pack/v1"`,
		`"status": "handoff-ready"`,
		`"planner": "deterministic"`,
		`"aspect_ratio": "16:9"`,
		`"brief_path": "pack/linkedin-dense/brief.md"`,
		`"output_path": "pack/linkedin-dense/final.png"`,
		`"state": "handoff_ready"`,
		`"failure_reason": "manual handoff pending"`,
		`"required_content": [`,
		`"avoid": [`,
	} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("content pack JSON missing %s: %s", want, jsonText)
		}
	}
}

func sampleVisualPacket() model.VisualPacket {
	return model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Technical Map: SkillOpt",
		Thesis:        "SkillOpt combines source-backed agent skill evidence into reusable workflows.",
		RequiredText: []string{
			"Technical Map: SkillOpt",
			"Source-backed agent skills",
			"Evaluation workflow",
		},
		RankedClaims: []model.Claim{
			{ID: "claim-1", Text: "SkillOpt reports +23.5 accuracy on held-out tasks.", Kind: "metric", Confidence: "source-backed", SourceRefs: []string{"paper.pdf#page=2"}},
			{ID: "claim-2", Text: "The planner keeps source gathering separate from rendering.", Kind: "architecture", Confidence: "source-backed", SourceRefs: []string{"readme.md#architecture"}},
			{ID: "claim-3", Text: "Open questions remain around overlapping skill selection.", Kind: "risk", Confidence: "source-backed", SourceRefs: []string{"notes.md#questions"}},
		},
	}
}

func targetIDs(targets []TargetSpec) []string {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	return ids
}

func claimTexts(claims []model.Claim) []string {
	texts := make([]string, 0, len(claims))
	for _, claim := range claims {
		texts = append(texts, claim.Text)
	}
	return texts
}

func allowedContentToken(text string) bool {
	switch text {
	case "title", "top claims", "core message":
		return true
	default:
		return false
	}
}
