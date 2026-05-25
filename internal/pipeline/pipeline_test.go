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
	if len(diskManifest.OutputFiles) < 4 {
		t.Fatalf("disk manifest output_files = %#v, want at least four files", diskManifest.OutputFiles)
	}
}

func hasOutputKind(files []model.OutputFile, kind string) bool {
	return slices.ContainsFunc(files, func(file model.OutputFile) bool {
		return file.Kind == kind
	})
}
