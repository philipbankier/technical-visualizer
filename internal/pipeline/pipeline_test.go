package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestRunOfflineAutoUsesLocalBackendEvenWithAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nOffline mode must not upload source-derived content."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "auto",
		Renderer:  "hybrid",
		Offline:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local non-remote", manifest.Backend)
	}
	if !manifest.Audit.FallbackUsed {
		t.Fatalf("manifest audit fallback = false, want true for offline auto")
	}
	if manifest.Audit.WarningCount != len(manifest.Warnings) {
		t.Fatalf("manifest audit warning count = %d, want %d", manifest.Audit.WarningCount, len(manifest.Warnings))
	}
	if _, err := os.Stat(filepath.Join(outputDir, "final.png")); err != nil {
		t.Fatalf("Stat(final.png) error = %v", err)
	}
}

func TestRunDefaultBackendStaysLocalWithAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nDefault mode must not upload source-derived content."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local non-remote", manifest.Backend)
	}
}

func TestBaselineAuditCopiesImageGenerationMetadata(t *testing.T) {
	imageResult := imageGenerationResult{
		BackendInfo:             model.BackendInfo{Name: "openai", Remote: true, Model: "gpt-image-2"},
		RemoteImageAttempted:    true,
		FallbackUsed:            true,
		PromptTruncated:         true,
		PromptTruncationMessage: "prompt truncated to fit gpt-image-2 prompt limit",
	}
	publicEvidence := model.EvidenceBundle{
		Sources:    []model.SourceSpec{{ID: "src-test", Kind: model.SourceMarkdown, Input: "notes.md"}},
		Items:      []model.EvidenceItem{{ID: "item-test", SourceID: "src-test", Kind: "markdown", Title: "Notes"}},
		Redactions: []model.Redaction{{SourceID: "src-test", Reason: "secret"}},
	}
	warnings := []string{"openai unavailable", "used fallback"}

	audit := baselineAudit(Options{Backend: "hybrid"}, imageResult, publicEvidence, warnings)

	if audit.SelectedBackend != "openai" || audit.RequestedBackend != "hybrid" {
		t.Fatalf("audit backend = %#v, want requested hybrid and selected openai", audit)
	}
	if audit.SourceCount != 1 || audit.EvidenceItemCount != 1 || audit.WarningCount != 2 || audit.RedactionCount != 1 {
		t.Fatalf("audit counts = %#v, want source=1 evidence=1 warning=2 redaction=1", audit)
	}
	if !audit.RemoteImageAttempted || !audit.FallbackUsed || !audit.PromptTruncated {
		t.Fatalf("audit flags = %#v, want remote, fallback, and truncation true", audit)
	}
	if audit.PromptTruncationMessage != imageResult.PromptTruncationMessage {
		t.Fatalf("audit truncation message = %q, want %q", audit.PromptTruncationMessage, imageResult.PromptTruncationMessage)
	}
}

func TestRunRejectsExplicitRemoteBackendWhenOfflineBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "openai",
		Offline:   true,
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want offline/openai rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(statErr) {
		t.Fatalf("scaffold.html exists after rejected run, stat error = %v", statErr)
	}
}

func TestRunRejectsCodexBackendBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "codex",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want codex rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(statErr) {
		t.Fatalf("scaffold.html exists after rejected run, stat error = %v", statErr)
	}
}

func TestRunSanitizesAbsoluteLocalPathsInOutputs(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "private-repo")
	if err := os.MkdirAll(filepath.Join(repo, "cmd", "server"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Private Repo\n\nSource-backed docs."), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "cmd", "server", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{repo},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, name := range []string{"visual-packet.json", "manifest.json", "scaffold.html"} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		if strings.Contains(string(data), root) {
			t.Fatalf("%s leaked absolute root %q:\n%s", name, root, data)
		}
	}
}

func TestRunLocalHTMLWritesBundleAndManifest(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThe renderer creates a local fallback artifact."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
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

	for _, name := range []string{"scaffold.html", "visual-packet.json", "manifest.json", "final.png"} {
		info, err := os.Stat(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}

	if len(manifest.Sources) != 1 {
		t.Fatalf("manifest sources = %#v, want one source", manifest.Sources)
	}
	if manifest.Renderer != "html" {
		t.Fatalf("manifest renderer = %q, want html", manifest.Renderer)
	}
	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local non-remote", manifest.Backend)
	}
	if manifest.Style != "analytic" {
		t.Fatalf("manifest style = %q, want analytic", manifest.Style)
	}
	if len(manifest.Warnings) != 0 {
		t.Fatalf("manifest warnings = %#v, want empty warnings", manifest.Warnings)
	}
	if manifest.Audit.ToolVersion != "0.1.0" {
		t.Fatalf("manifest audit tool version = %q, want 0.1.0", manifest.Audit.ToolVersion)
	}
	if manifest.Audit.RequestedBackend != "local" || manifest.Audit.SelectedBackend != manifest.Backend.Name {
		t.Fatalf("manifest audit backend = %#v, want requested local and selected %q", manifest.Audit, manifest.Backend.Name)
	}
	if manifest.Audit.SourceCount != len(manifest.Sources) || manifest.Audit.EvidenceItemCount == 0 || manifest.Audit.WarningCount != len(manifest.Warnings) {
		t.Fatalf("manifest audit counts = %#v, sources=%d warnings=%d", manifest.Audit, len(manifest.Sources), len(manifest.Warnings))
	}
	if manifest.Audit.RemoteImageAttempted || manifest.Audit.FallbackUsed || manifest.Audit.PromptTruncated {
		t.Fatalf("manifest audit flags = %#v, want false remote/fallback/truncation", manifest.Audit)
	}
	for _, kind := range []string{"scaffold", "visual_packet", "image", "manifest"} {
		if !hasOutputKind(manifest.OutputFiles, kind) {
			t.Fatalf("manifest output files missing kind %q: %#v", kind, manifest.OutputFiles)
		}
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}
	var diskManifest struct {
		Sources     []any                 `json:"sources"`
		Renderer    string                `json:"renderer"`
		Backend     struct{ Name string } `json:"backend"`
		Style       string                `json:"style"`
		Warnings    json.RawMessage       `json:"warnings"`
		Audit       model.ManifestAudit   `json:"audit"`
		OutputFiles []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"output_files"`
	}
	if err := json.Unmarshal(data, &diskManifest); err != nil {
		t.Fatalf("Unmarshal(manifest.json) error = %v", err)
	}
	if len(diskManifest.Sources) != 1 || diskManifest.Renderer != "html" || diskManifest.Backend.Name != "local" || diskManifest.Style != "analytic" {
		t.Fatalf("disk manifest missing required fields: %s", data)
	}
	if len(diskManifest.Warnings) == 0 || string(diskManifest.Warnings) != "[]" {
		t.Fatalf("disk manifest warnings = %s, want [] in %s", diskManifest.Warnings, data)
	}
	if diskManifest.Audit.ToolVersion != "0.1.0" || diskManifest.Audit.RequestedBackend != "local" || diskManifest.Audit.SelectedBackend != "local" {
		t.Fatalf("disk manifest audit missing baseline fields: %s", data)
	}
	if len(diskManifest.OutputFiles) < 4 {
		t.Fatalf("disk manifest output_files = %#v, want at least four files", diskManifest.OutputFiles)
	}
}

func hasOutputKind(files []model.OutputFile, kind string) bool {
	return slices.ContainsFunc(files, func(file model.OutputFile) bool {
		return file.Kind == kind
	})
}
