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
