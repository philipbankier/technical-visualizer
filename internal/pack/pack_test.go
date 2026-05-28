package pack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestOpenAIPlannerBuildsStructuredOutputRequest(t *testing.T) {
	var gotRequest struct {
		Model string `json:"model"`
		Input []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"input"`
		Text struct {
			Format struct {
				Type   string         `json:"type"`
				Name   string         `json:"name"`
				Strict bool           `json:"strict"`
				Schema map[string]any `json:"schema"`
			} `json:"format"`
		} `json:"text"`
	}
	responsePack := ContentPack{
		SchemaVersion: "content-pack/v1",
		SourcePacket:  "visual-packet.json",
		Title:         "Technical Map: SkillOpt",
		Status:        StatusPlanned,
		Strategy:      Strategy{Planner: "openai", Summary: "selected targets"},
		Targets:       openAIPlannerTargets(sampleVisualPacket()),
	}
	responseJSON, err := json.Marshal(responsePack)
	if err != nil {
		t.Fatalf("Marshal(response pack) error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("request path = %q, want /v1/responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("Decode(request) error = %v", err)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{{
				"type": "message",
				"content": []map[string]string{{
					"type": "output_text",
					"text": string(responseJSON),
				}},
			}},
		}); err != nil {
			t.Fatalf("Encode(response) error = %v", err)
		}
	}))
	defer server.Close()

	planner := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Model:      "gpt-4o-mini",
	})
	contentPack, err := planner.Plan(context.Background(), sampleVisualPacket(), PlannerOptions{Mode: "auto"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if contentPack.Strategy.Planner != "openai" {
		t.Fatalf("planner = %q, want openai", contentPack.Strategy.Planner)
	}
	if gotRequest.Model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want gpt-4o-mini", gotRequest.Model)
	}
	if gotRequest.Text.Format.Type != "json_schema" || gotRequest.Text.Format.Name != "content_pack" || !gotRequest.Text.Format.Strict {
		t.Fatalf("text.format = %#v, want strict content_pack json_schema", gotRequest.Text.Format)
	}
	if gotRequest.Text.Format.Schema["type"] != "object" {
		t.Fatalf("schema = %#v, want object schema", gotRequest.Text.Format.Schema)
	}
	requestText := marshalForSearch(t, gotRequest.Input)
	for _, want := range []string{"linkedin-dense", "social-teaser", "blog-og", "planned", "visual-packet.json", "may not add factual claims"} {
		if !strings.Contains(requestText, want) {
			t.Fatalf("planner request missing %q:\n%s", want, requestText)
		}
	}
}

func TestValidateContentPackRejectsMissingTargets(t *testing.T) {
	contentPack := validOpenAIContentPack(sampleVisualPacket())
	contentPack.Targets = contentPack.Targets[:1]

	err := ValidateContentPack(contentPack, sampleVisualPacket())
	if err == nil {
		t.Fatalf("ValidateContentPack() error = nil, want missing target error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "target") {
		t.Fatalf("ValidateContentPack() error = %q, want target explanation", err)
	}
}

func TestOpenAIPlannerRejectsIncompleteResponse(t *testing.T) {
	responsePack := validOpenAIContentPack(sampleVisualPacket())
	responseJSON, err := json.Marshal(responsePack)
	if err != nil {
		t.Fatalf("Marshal(response pack) error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":             "incomplete",
			"incomplete_details": map[string]string{"reason": "max_output_tokens"},
			"output_text":        string(responseJSON),
		}); err != nil {
			t.Fatalf("Encode(response) error = %v", err)
		}
	}))
	defer server.Close()

	planner := NewOpenAIPlanner(OpenAIPlannerConfig{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	_, err = planner.Plan(context.Background(), sampleVisualPacket(), PlannerOptions{Mode: "auto"})
	if err == nil {
		t.Fatalf("Plan() error = nil, want incomplete response error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "incomplete") {
		t.Fatalf("Plan() error = %q, want incomplete response explanation", err)
	}
}

func TestOpenAIPlannerRejectsInvalidContentPack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"output_text": `{"schema_version":"content-pack/v0","source_packet":"visual-packet.json","title":"Bad","status":"planned","strategy":{"planner":"openai","summary":"bad"},"targets":[]}`,
		}); err != nil {
			t.Fatalf("Encode(response) error = %v", err)
		}
	}))
	defer server.Close()

	planner := NewOpenAIPlanner(OpenAIPlannerConfig{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	_, err := planner.Plan(context.Background(), sampleVisualPacket(), PlannerOptions{Mode: "auto"})
	if err == nil {
		t.Fatalf("Plan() error = nil, want invalid schema error")
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

func validOpenAIContentPack(packet model.VisualPacket) ContentPack {
	return ContentPack{
		SchemaVersion: "content-pack/v1",
		SourcePacket:  "visual-packet.json",
		Title:         packet.Title,
		Status:        StatusPlanned,
		Strategy:      Strategy{Planner: "openai", Summary: "selected targets"},
		Targets:       openAIPlannerTargets(packet),
	}
}

func openAIPlannerTargets(packet model.VisualPacket) []TargetSpec {
	targets := defaultTargetSpecs(packet)
	for i := range targets {
		targets[i].RequiredContent = []string{"title", packet.RequiredText[0]}
		targets[i].Avoid = []string{"inventing unsupported metrics"}
	}
	return targets
}

func marshalForSearch(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return string(data)
}
