package pack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

const (
	defaultOpenAIPlannerModel   = "gpt-4o-mini"
	defaultOpenAIPlannerBaseURL = "https://api.openai.com"
	defaultPlannerTimeout       = 90 * time.Second
)

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

type Strategy struct {
	Planner string `json:"planner"`
	Summary string `json:"summary"`
}

type TargetSpec struct {
	ID              string      `json:"id"`
	Platform        string      `json:"platform"`
	Intent          string      `json:"intent"`
	Density         string      `json:"density"`
	AspectRatio     string      `json:"aspect_ratio"`
	BriefPath       string      `json:"brief_path"`
	OutputPath      string      `json:"output_path"`
	State           TargetState `json:"state"`
	FailureReason   string      `json:"failure_reason,omitempty"`
	RequiredContent []string    `json:"required_content"`
	Avoid           []string    `json:"avoid"`
}

type Options struct {
	Mode string
}

type PlannerOptions struct {
	Mode string
}

type Planner interface {
	Plan(context.Context, model.VisualPacket, PlannerOptions) (ContentPack, error)
}

type DeterministicPlanner struct{}

func Plan(packet model.VisualPacket, opts Options) (ContentPack, error) {
	return DeterministicPlanner{}.Plan(context.Background(), packet, PlannerOptions{Mode: opts.Mode})
}

func (DeterministicPlanner) Plan(_ context.Context, packet model.VisualPacket, opts PlannerOptions) (ContentPack, error) {
	if opts.Mode != "auto" {
		return ContentPack{}, fmt.Errorf("unsupported pack mode %q", opts.Mode)
	}

	targets := defaultTargetSpecs(packet)

	return ContentPack{
		SchemaVersion: "content-pack/v1",
		SourcePacket:  "visual-packet.json",
		Title:         packet.Title,
		Status:        DeriveStatus(targets),
		Strategy: Strategy{
			Planner: "deterministic",
			Summary: fmt.Sprintf("Deterministic auto plan for three targets using %d required text items and %d ranked claims.", len(packet.RequiredText), len(packet.RankedClaims)),
		},
		Targets: targets,
	}, nil
}

type OpenAIPlannerConfig struct {
	APIKey           string
	BaseURL          string
	Model            string
	HTTPClient       *http.Client
	OperationTimeout time.Duration
}

type OpenAIPlanner struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	timeout    time.Duration
}

func NewOpenAIPlanner(config OpenAIPlannerConfig) *OpenAIPlanner {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenAIPlannerBaseURL
	}
	modelName := strings.TrimSpace(config.Model)
	if modelName == "" {
		modelName = defaultOpenAIPlannerModel
	}
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	timeout := config.OperationTimeout
	if timeout <= 0 {
		timeout = defaultPlannerTimeout
	}
	return &OpenAIPlanner{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      modelName,
		httpClient: client,
		timeout:    timeout,
	}
}

func (p *OpenAIPlanner) Plan(ctx context.Context, packet model.VisualPacket, opts PlannerOptions) (ContentPack, error) {
	if opts.Mode != "auto" {
		return ContentPack{}, fmt.Errorf("unsupported pack mode %q", opts.Mode)
	}
	if p.apiKey == "" {
		return ContentPack{}, errors.New("openai pack planner requires OPENAI_API_KEY or configured API key")
	}

	requestBody := openAIPlannerRequest{
		Model: p.model,
		Input: []openAIPlannerMessage{
			{Role: "system", Content: openAIPlannerSystemPrompt()},
			{Role: "user", Content: openAIPlannerUserPrompt(packet)},
		},
		Text: openAIPlannerText{
			Format: openAIPlannerTextFormat{
				Type:   "json_schema",
				Name:   "content_pack",
				Strict: true,
				Schema: contentPackJSONSchema(),
			},
		},
		Store: false,
	}
	data, err := json.Marshal(requestBody)
	if err != nil {
		return ContentPack{}, err
	}

	opCtx, cancel := plannerContext(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(opCtx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(data))
	if err != nil {
		return ContentPack{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ContentPack{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ContentPack{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ContentPack{}, fmt.Errorf("openai pack planner failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded openAIPlannerResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return ContentPack{}, err
	}
	text, err := decoded.text()
	if err != nil {
		return ContentPack{}, err
	}
	var contentPack ContentPack
	if err := json.Unmarshal([]byte(text), &contentPack); err != nil {
		return ContentPack{}, fmt.Errorf("openai pack planner returned invalid json: %w", err)
	}
	if err := validateContentPack(contentPack, packet, "openai"); err != nil {
		return ContentPack{}, err
	}
	return contentPack, nil
}

type openAIPlannerRequest struct {
	Model string                 `json:"model"`
	Input []openAIPlannerMessage `json:"input"`
	Text  openAIPlannerText      `json:"text"`
	Store bool                   `json:"store"`
}

type openAIPlannerMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIPlannerText struct {
	Format openAIPlannerTextFormat `json:"format"`
}

type openAIPlannerTextFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIPlannerResponse struct {
	Status            string `json:"status,omitempty"`
	IncompleteDetails *struct {
		Reason string `json:"reason,omitempty"`
	} `json:"incomplete_details,omitempty"`
	OutputText string `json:"output_text"`
	Output     []struct {
		Type    string `json:"type"`
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text,omitempty"`
			Refusal string `json:"refusal,omitempty"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (r openAIPlannerResponse) text() (string, error) {
	if r.Error != nil && strings.TrimSpace(r.Error.Message) != "" {
		return "", errors.New(r.Error.Message)
	}
	if r.Status == "incomplete" {
		reason := "unknown"
		if r.IncompleteDetails != nil && strings.TrimSpace(r.IncompleteDetails.Reason) != "" {
			reason = strings.TrimSpace(r.IncompleteDetails.Reason)
		}
		return "", fmt.Errorf("openai pack planner response incomplete: %s", reason)
	}
	if r.Status != "" && r.Status != "completed" {
		return "", fmt.Errorf("openai pack planner response status %q", r.Status)
	}
	if strings.TrimSpace(r.OutputText) != "" {
		return r.OutputText, nil
	}
	for _, output := range r.Output {
		for _, content := range output.Content {
			if content.Type == "refusal" && strings.TrimSpace(content.Refusal) != "" {
				return "", fmt.Errorf("openai pack planner refused: %s", content.Refusal)
			}
			if strings.TrimSpace(content.Text) != "" {
				return content.Text, nil
			}
		}
	}
	return "", errors.New("openai pack planner response did not include output text")
}

func openAIPlannerSystemPrompt() string {
	return strings.Join([]string{
		"You are planning a source-backed visual content pack.",
		"Return only JSON matching the provided content-pack schema.",
		"You may choose target emphasis, required content, and avoid lists.",
		"You may not add factual claims not present in the packet.",
		"Use only the allowed target IDs and planned output paths.",
	}, "\n")
}

func openAIPlannerUserPrompt(packet model.VisualPacket) string {
	packetJSON, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		packetJSON = []byte("{}")
	}
	return strings.Join([]string{
		"Allowed target IDs: linkedin-dense, social-teaser, blog-og.",
		"Allowed target states: planned.",
		"Source packet path: visual-packet.json.",
		"Allowed output paths: pack/linkedin-dense/final.png, pack/social-teaser/final.png, pack/blog-og/final.png.",
		"Allowed brief paths: pack/linkedin-dense/brief.md, pack/social-teaser/brief.md, pack/blog-og/brief.md.",
		"Use schema_version content-pack/v1 and source_packet visual-packet.json.",
		"Set strategy.planner to openai.",
		"",
		"Visual packet JSON:",
		string(packetJSON),
	}, "\n")
}

func contentPackJSONSchema() map[string]any {
	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"schema_version", "source_packet", "title", "status", "strategy", "targets"},
		"properties": map[string]any{
			"schema_version": map[string]any{"type": "string", "enum": []string{"content-pack/v1"}},
			"source_packet":  map[string]any{"type": "string", "enum": []string{"visual-packet.json"}},
			"title":          map[string]any{"type": "string"},
			"status":         map[string]any{"type": "string", "enum": []string{string(StatusPlanned)}},
			"strategy": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"planner", "summary"},
				"properties": map[string]any{
					"planner": map[string]any{"type": "string", "enum": []string{"openai"}},
					"summary": map[string]any{"type": "string"},
				},
			},
			"targets": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"id", "platform", "intent", "density", "aspect_ratio", "brief_path", "output_path", "state", "required_content", "avoid"},
					"properties": map[string]any{
						"id":               map[string]any{"type": "string", "enum": []string{"linkedin-dense", "social-teaser", "blog-og"}},
						"platform":         map[string]any{"type": "string"},
						"intent":           map[string]any{"type": "string"},
						"density":          map[string]any{"type": "string"},
						"aspect_ratio":     map[string]any{"type": "string"},
						"brief_path":       map[string]any{"type": "string"},
						"output_path":      map[string]any{"type": "string"},
						"state":            map[string]any{"type": "string", "enum": []string{string(StatePlanned)}},
						"required_content": stringArray,
						"avoid":            stringArray,
					},
				},
			},
		},
	}
}

func ValidateContentPack(contentPack ContentPack, packet model.VisualPacket) error {
	return validateContentPack(contentPack, packet, "")
}

func validateContentPack(contentPack ContentPack, packet model.VisualPacket, expectedPlanner string) error {
	if contentPack.SchemaVersion != "content-pack/v1" {
		return fmt.Errorf("schema_version = %q, want content-pack/v1", contentPack.SchemaVersion)
	}
	if contentPack.SourcePacket != "visual-packet.json" {
		return fmt.Errorf("source_packet = %q, want visual-packet.json", contentPack.SourcePacket)
	}
	if strings.TrimSpace(contentPack.Title) == "" {
		return errors.New("title must not be empty")
	}
	plannerName := strings.TrimSpace(contentPack.Strategy.Planner)
	if plannerName == "" {
		return errors.New("strategy.planner must not be empty")
	}
	if plannerName != "deterministic" && plannerName != "openai" {
		return fmt.Errorf("strategy.planner = %q, want deterministic or openai", contentPack.Strategy.Planner)
	}
	if expectedPlanner != "" && plannerName != expectedPlanner {
		return fmt.Errorf("strategy.planner = %q, want %q", contentPack.Strategy.Planner, expectedPlanner)
	}
	if strings.TrimSpace(contentPack.Strategy.Summary) == "" {
		return errors.New("strategy.summary must not be empty")
	}
	if contentPack.Status != DeriveStatus(contentPack.Targets) {
		return fmt.Errorf("status = %q, want %q", contentPack.Status, DeriveStatus(contentPack.Targets))
	}
	allowedClaims := allowedRequiredContent(packet)
	targets := defaultTargetSpecs(packet)
	byID := make(map[string]TargetSpec, len(targets))
	for _, target := range targets {
		byID[target.ID] = target
	}
	if len(contentPack.Targets) != len(byID) {
		return fmt.Errorf("content pack has %d targets, want %d", len(contentPack.Targets), len(byID))
	}
	seen := map[string]bool{}
	for _, target := range contentPack.Targets {
		defaultTarget, ok := byID[target.ID]
		if !ok {
			return fmt.Errorf("target id %q is not allowed", target.ID)
		}
		if seen[target.ID] {
			return fmt.Errorf("target id %q is duplicated", target.ID)
		}
		seen[target.ID] = true
		if target.BriefPath != defaultTarget.BriefPath || target.OutputPath != defaultTarget.OutputPath {
			return fmt.Errorf("target %q paths must be %q and %q", target.ID, defaultTarget.BriefPath, defaultTarget.OutputPath)
		}
		if target.AspectRatio != defaultTarget.AspectRatio {
			return fmt.Errorf("target %q aspect_ratio = %q, want %q", target.ID, target.AspectRatio, defaultTarget.AspectRatio)
		}
		if target.State != StatePlanned {
			return fmt.Errorf("target %q state = %q, want planned", target.ID, target.State)
		}
		if strings.TrimSpace(target.Platform) == "" || strings.TrimSpace(target.Intent) == "" || strings.TrimSpace(target.Density) == "" {
			return fmt.Errorf("target %q must include platform, intent, and density", target.ID)
		}
		if len(target.RequiredContent) == 0 {
			return fmt.Errorf("target %q required_content must not be empty", target.ID)
		}
		if len(target.Avoid) == 0 {
			return fmt.Errorf("target %q avoid must not be empty", target.ID)
		}
		for _, item := range target.RequiredContent {
			if !allowedClaims[item] {
				return fmt.Errorf("target %q required_content %q is not present in source packet", target.ID, item)
			}
		}
	}
	for id := range byID {
		if !seen[id] {
			return fmt.Errorf("target id %q is missing", id)
		}
	}
	return nil
}

func allowedRequiredContent(packet model.VisualPacket) map[string]bool {
	allowed := map[string]bool{
		"title":        true,
		"top claims":   true,
		"core message": true,
	}
	add := func(values ...string) {
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				allowed[value] = true
			}
		}
	}
	add(packet.RequiredText...)
	add(packet.Thesis)
	for _, section := range packet.Sections {
		add(section.Title, section.Summary)
		add(section.Items...)
	}
	for _, block := range packet.ContentBlocks {
		add(block.Title, block.Summary, block.Text)
		add(block.Items...)
	}
	for _, metric := range packet.Metrics {
		add(metric.Label, metric.Value, metric.Context)
	}
	for _, event := range packet.Timeline {
		add(event.Date, event.Label, event.Summary)
	}
	for _, entity := range packet.Entities {
		add(entity.Kind, entity.Name, entity.Detail)
	}
	for _, table := range packet.Tables {
		add(table.Title)
		add(table.Headers...)
		for _, row := range table.Rows {
			add(row...)
		}
	}
	for _, diagram := range packet.Diagrams {
		add(diagram.Title, diagram.Kind, diagram.Text)
	}
	for _, question := range packet.OpenQuestions {
		add(question.Text, question.Context)
	}
	for _, risk := range packet.Risks {
		add(risk.Text, risk.Severity)
	}
	for _, tradeoff := range packet.Tradeoffs {
		add(tradeoff.Choice, tradeoff.Reason)
	}
	for _, unknown := range packet.Unknowns {
		add(unknown.Text)
	}
	for _, claim := range packet.RankedClaims {
		add(claim.Text)
	}
	for _, fact := range packet.Facts {
		add(fact.Text)
	}
	for _, value := range packet.Layout.Regions {
		if strings.TrimSpace(value) != "" {
			allowed[value] = true
		}
	}
	return allowed
}

func plannerContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(timeout)
	if existing, ok := ctx.Deadline(); ok && existing.Before(deadline) {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func DeriveStatus(targets []TargetSpec) Status {
	if len(targets) == 0 {
		return StatusPlanned
	}

	allComplete := true
	allPlanned := true
	hasHandoffReady := false
	hasPartialState := false

	for _, target := range targets {
		switch target.State {
		case StatePlanned:
			allComplete = false
		case StateGenerated, StateVerified:
			allPlanned = false
		case StateHandoffReady:
			allComplete = false
			allPlanned = false
			hasHandoffReady = true
		case StateFailed:
			allComplete = false
			allPlanned = false
			hasPartialState = true
		default:
			allComplete = false
			allPlanned = false
			hasPartialState = true
		}
	}

	if allComplete {
		return StatusComplete
	}
	if hasPartialState || hasGeneratedOrVerified(targets) {
		return StatusPartial
	}
	if hasHandoffReady {
		return StatusHandoffReady
	}
	if allPlanned {
		return StatusPlanned
	}
	return StatusPartial
}

func hasGeneratedOrVerified(targets []TargetSpec) bool {
	for _, target := range targets {
		if target.State == StateGenerated || target.State == StateVerified {
			return true
		}
	}
	return false
}

func requiredContent(packet model.VisualPacket, structural []string, requiredLimit int, claimLimit int) []string {
	content := make([]string, 0, len(structural)+requiredLimit+claimLimit)
	content = appendUnique(content, structural...)
	content = appendUnique(content, limitedStrings(packet.RequiredText, requiredLimit)...)

	claimTexts := make([]string, 0, len(packet.RankedClaims))
	for _, claim := range packet.RankedClaims {
		claimTexts = append(claimTexts, claim.Text)
	}
	content = appendUnique(content, limitedStrings(claimTexts, claimLimit)...)
	return content
}

func defaultTargetSpecs(packet model.VisualPacket) []TargetSpec {
	return []TargetSpec{
		{
			ID:              "linkedin-dense",
			Platform:        "linkedin",
			Intent:          "technical deep-dive infographic",
			Density:         "high",
			AspectRatio:     "16:9",
			BriefPath:       "pack/linkedin-dense/brief.md",
			OutputPath:      "pack/linkedin-dense/final.png",
			State:           StatePlanned,
			RequiredContent: requiredContent(packet, []string{"title", "top claims"}, len(packet.RequiredText), 3),
			Avoid:           []string{"inventing unsupported metrics", "using unreadable tiny text"},
		},
		{
			ID:              "social-teaser",
			Platform:        "social",
			Intent:          "pretty lower-density social preview",
			Density:         "low",
			AspectRatio:     "1:1",
			BriefPath:       "pack/social-teaser/brief.md",
			OutputPath:      "pack/social-teaser/final.png",
			State:           StatePlanned,
			RequiredContent: requiredContent(packet, []string{"title", "core message"}, 1, 1),
			Avoid:           []string{"inventing unsupported metrics", "using unreadable tiny text"},
		},
		{
			ID:              "blog-og",
			Platform:        "blog",
			Intent:          "article and README open graph hero",
			Density:         "medium",
			AspectRatio:     "1.91:1",
			BriefPath:       "pack/blog-og/brief.md",
			OutputPath:      "pack/blog-og/final.png",
			State:           StatePlanned,
			RequiredContent: requiredContent(packet, []string{"title", "top claims"}, len(packet.RequiredText), 2),
			Avoid:           []string{"inventing unsupported metrics", "using unreadable tiny text"},
		},
	}
}

func limitedStrings(values []string, limit int) []string {
	if limit < 0 || limit > len(values) {
		limit = len(values)
	}
	return values[:limit]
}

func appendUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		if addition == "" || contains(values, addition) {
			continue
		}
		values = append(values, addition)
	}
	return values
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
