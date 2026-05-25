package model

import (
	"encoding/json"
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
