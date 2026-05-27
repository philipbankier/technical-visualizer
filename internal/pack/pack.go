package pack

import (
	"fmt"

	"github.com/philipbankier/technical-visualizer/internal/model"
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

func Plan(packet model.VisualPacket, opts Options) (ContentPack, error) {
	if opts.Mode != "auto" {
		return ContentPack{}, fmt.Errorf("unsupported pack mode %q", opts.Mode)
	}

	targets := []TargetSpec{
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
