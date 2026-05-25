package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestValidateBundleFailsWhenRequiredFilesAreMissing(t *testing.T) {
	issues := ValidateBundle(t.TempDir())

	for _, path := range []string{"final.png", "scaffold.html", "visual-packet.json", "manifest.json"} {
		if !hasIssueForPath(issues, path) {
			t.Fatalf("ValidateBundle() issues = %#v, want issue for %s", issues, path)
		}
	}
}

func TestValidateBundlePassesForValidLocalBundle(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         `Acme <Map>`,
		Thesis:        "Acme maps source evidence.",
		RequiredText:  []string{`Acme <Map>`, `Renderer & Source`},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>"+html.EscapeString(packet.Title)+" "+html.EscapeString(packet.RequiredText[1])+"</body></html>")
	writeValidManifest(t, dir)

	if issues := ValidateBundle(dir); len(issues) != 0 {
		t.Fatalf("ValidateBundle() issues = %#v, want none", issues)
	}
}

func TestValidateBundleFailsWhenManifestIsIncomplete(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeJSON(t, filepath.Join(dir, "manifest.json"), model.Manifest{SchemaVersion: "manifest/v1"})
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "sources") {
		t.Fatalf("ValidateBundle() issues = %#v, want manifest sources issue", issues)
	}
}

func TestValidateBundleFailsWhenManifestSHAIsWrong(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.OutputFiles[0].SHA256 = "bad"
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "sha") {
		t.Fatalf("ValidateBundle() issues = %#v, want sha issue", issues)
	}
}

func TestValidateBundleFailsWhenManifestSourceIsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*model.Manifest)
		want   string
	}{
		{
			name: "empty-id",
			mutate: func(manifest *model.Manifest) {
				manifest.Sources[0].ID = ""
			},
			want: "id",
		},
		{
			name: "unsupported-kind",
			mutate: func(manifest *model.Manifest) {
				manifest.Sources[0].Kind = "unsupported"
			},
			want: "kind",
		},
		{
			name: "empty-input",
			mutate: func(manifest *model.Manifest) {
				manifest.Sources[0].Input = ""
			},
			want: "input",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
			writeTestPNG(t, filepath.Join(dir, "final.png"))
			writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
			writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
			manifest := validManifest(dir)
			tc.mutate(&manifest)
			writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

			issues := ValidateBundle(dir)
			if !hasIssueContaining(issues, "manifest.json", "source") || !hasIssueContaining(issues, "manifest.json", tc.want) {
				t.Fatalf("ValidateBundle() issues = %#v, want source %s issue", issues, tc.want)
			}
		})
	}
}

func TestValidateBundleAllowsAparenteGistStyle(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.Style = "gist-aparente"
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	if issues := ValidateBundle(dir); len(issues) != 0 {
		t.Fatalf("ValidateBundle() issues = %#v, want none", issues)
	}
}

func TestValidateBundleFailsWhenManifestBackendRendererOrStyleIsInvalid(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.Backend.Name = "codex"
	manifest.Renderer = "video"
	manifest.Style = "unknown-style"
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "backend") {
		t.Fatalf("ValidateBundle() issues = %#v, want backend issue", issues)
	}
	if !hasIssueContaining(issues, "manifest.json", "renderer") {
		t.Fatalf("ValidateBundle() issues = %#v, want renderer issue", issues)
	}
	if !hasIssueContaining(issues, "manifest.json", "style") {
		t.Fatalf("ValidateBundle() issues = %#v, want style issue", issues)
	}
}

func TestValidateBundleFailsWhenScaffoldOmitsRequiredText(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Renderer"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	writeValidManifest(t, dir)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "scaffold.html", "Renderer") {
		t.Fatalf("ValidateBundle() issues = %#v, want missing required text issue", issues)
	}
}

func TestValidateBundleFailsWhenFinalPNGDoesNotDecode(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeFile(t, filepath.Join(dir, "final.png"), "not a png")
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	writeValidManifest(t, dir)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "final.png", "png") {
		t.Fatalf("ValidateBundle() issues = %#v, want png decode issue", issues)
	}
}

func writeValidManifest(t *testing.T, dir string) {
	t.Helper()

	writeJSON(t, filepath.Join(dir, "manifest.json"), validManifest(dir))
}

func validManifest(dir string) model.Manifest {
	return model.Manifest{
		SchemaVersion: "manifest/v1",
		Sources: []model.SourceSpec{{
			ID:    "src-test",
			Kind:  model.SourceMarkdown,
			Input: "notes.md",
		}},
		Backend:  model.BackendInfo{Name: "local", Remote: false},
		Renderer: "html",
		Style:    "executive-dark",
		OutputFiles: []model.OutputFile{
			{Kind: "scaffold", Path: "scaffold.html", SHA256: sha256File(filepath.Join(dir, "scaffold.html"))},
			{Kind: "visual_packet", Path: "visual-packet.json", SHA256: sha256File(filepath.Join(dir, "visual-packet.json"))},
			{Kind: "image", Path: "final.png", SHA256: sha256File(filepath.Join(dir, "final.png"))},
			{Kind: "manifest", Path: "manifest.json"},
		},
	}
}

func sha256File(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 80, B: 140, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	writeFile(t, path, string(data))
}

func writeFile(t *testing.T, path string, value string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func hasIssueForPath(issues []Issue, path string) bool {
	for _, issue := range issues {
		if issue.Path == path {
			return true
		}
	}
	return false
}

func hasIssueContaining(issues []Issue, path string, text string) bool {
	for _, issue := range issues {
		if issue.Path == path && strings.Contains(issue.Message, text) {
			return true
		}
	}
	return false
}
