package visualizer_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/pipeline"
)

func TestEndToEndLocalVisualization(t *testing.T) {
	outputDir := t.TempDir()

	_, err := pipeline.Run(context.Background(), pipeline.Options{
		Sources: []string{
			filepath.Join("testdata", "sample-repo"),
			filepath.Join("testdata", "notes.md"),
		},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Style:     "analytic",
		Goal:      "architecture-map",
		Offline:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, name := range []string{"final.png", "scaffold.html", "visual-packet.json", "manifest.json"} {
		path := filepath.Join(outputDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}

	packet := readVisualPacket(t, filepath.Join(outputDir, "visual-packet.json"))
	if !packetHasSourceLocator(packet, filepath.ToSlash(filepath.Join("cmd", "server", "main.go"))) {
		t.Fatalf("visual packet missing cmd/server/main.go source locator: %#v", packet.SourceRefs)
	}
	combinedEvidence := packetEvidenceText(packet)
	for _, want := range []string{"/healthz", "/orders/{id}"} {
		if !strings.Contains(combinedEvidence, want) {
			t.Fatalf("visual packet evidence missing %q in %q", want, combinedEvidence)
		}
	}
}

type visualPacketSummary struct {
	RankedClaims []struct {
		Text string `json:"text"`
	} `json:"ranked_claims"`
	Facts []struct {
		Text string `json:"text"`
	} `json:"facts"`
	SourceRefs []struct {
		Label   string `json:"label"`
		Locator string `json:"locator"`
	} `json:"source_refs"`
}

func readVisualPacket(t *testing.T, path string) visualPacketSummary {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var packet visualPacketSummary
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", path, err)
	}
	return packet
}

func packetHasSourceLocator(packet visualPacketSummary, locator string) bool {
	for _, ref := range packet.SourceRefs {
		if filepath.ToSlash(ref.Locator) == locator {
			return true
		}
	}
	return false
}

func packetEvidenceText(packet visualPacketSummary) string {
	var parts []string
	for _, claim := range packet.RankedClaims {
		parts = append(parts, claim.Text)
	}
	for _, fact := range packet.Facts {
		parts = append(parts, fact.Text)
	}
	for _, ref := range packet.SourceRefs {
		parts = append(parts, ref.Label, ref.Locator)
	}
	return strings.Join(parts, "\n")
}
