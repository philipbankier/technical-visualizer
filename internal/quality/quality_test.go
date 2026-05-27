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

func TestValidateBundleRequiresManifestNextSteps(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><title>Acme Map</title><body>Acme Map</body>")
	manifest := validManifest(dir)
	manifest.NextSteps = nil
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "next_steps") {
		t.Fatalf("ValidateBundle issues = %#v, want next_steps issue", issues)
	}
}

func TestValidateBundleReportsMissingDeclaredCodexHandoff(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.OutputFiles = append(manifest.OutputFiles, model.OutputFile{
		Kind:   "handoff_prompt",
		Path:   "handoff/codex-prompt.md",
		SHA256: "missing",
	})
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "handoff_brief") {
		t.Fatalf("ValidateBundle issues = %#v, want missing handoff_brief output", issues)
	}
	if !hasIssueContaining(issues, "manifest.json", "could not be checked") {
		t.Fatalf("ValidateBundle issues = %#v, want missing handoff prompt", issues)
	}
}

func TestValidateBundleRejectsBackslashOutputPathBeforeHashing(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.OutputFiles[0].Path = "..\\secret"
	manifest.OutputFiles[0].SHA256 = "missing"
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "unsafe path") {
		t.Fatalf("ValidateBundle issues = %#v, want unsafe path issue", issues)
	}
	if hasIssueContaining(issues, "manifest.json", "could not be checked") {
		t.Fatalf("ValidateBundle issues = %#v, should reject before hashing", issues)
	}
}

func TestValidateBundleAcceptsDeclaredCodexHandoff(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	if err := os.MkdirAll(filepath.Join(dir, "handoff"), 0o700); err != nil {
		t.Fatalf("MkdirAll(handoff) error = %v", err)
	}
	for _, rel := range []string{"handoff/codex-prompt.md", "handoff/image-brief.md", "handoff/qa-checklist.md", "handoff/style.md"} {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), rel)
	}
	manifest := validManifest(dir)
	manifest.NextSteps = []string{"Use the generated handoff package."}
	manifest.OutputFiles = append(manifest.OutputFiles,
		model.OutputFile{Kind: "handoff_prompt", Path: "handoff/codex-prompt.md", SHA256: sha256File(filepath.Join(dir, "handoff", "codex-prompt.md"))},
		model.OutputFile{Kind: "handoff_brief", Path: "handoff/image-brief.md", SHA256: sha256File(filepath.Join(dir, "handoff", "image-brief.md"))},
		model.OutputFile{Kind: "handoff_checklist", Path: "handoff/qa-checklist.md", SHA256: sha256File(filepath.Join(dir, "handoff", "qa-checklist.md"))},
		model.OutputFile{Kind: "handoff_style", Path: "handoff/style.md", SHA256: sha256File(filepath.Join(dir, "handoff", "style.md"))},
	)
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	if issues := ValidateBundle(dir); len(issues) != 0 {
		t.Fatalf("ValidateBundle issues = %#v, want none", issues)
	}
}

func TestValidateBundleAcceptsDeclaredContentPackAndBriefs(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	writeTestPackFiles(t, dir, "content-pack/v1", []map[string]any{
		{
			"id":          "linkedin-dense",
			"brief_path":  "pack/linkedin-dense/brief.md",
			"output_path": "pack/linkedin-dense/final.png",
			"state":       "planned",
		},
		{
			"id":          "social-teaser",
			"brief_path":  "pack/social-teaser/brief.md",
			"output_path": "pack/social-teaser/final.png",
			"state":       "planned",
		},
		{
			"id":          "blog-og",
			"brief_path":  "pack/blog-og/brief.md",
			"output_path": "pack/blog-og/final.png",
			"state":       "planned",
		},
	})
	manifest := validManifest(dir)
	manifest.NextSteps = []string{"Open content-pack.json and pack target briefs."}
	manifest.OutputFiles = append(manifest.OutputFiles, contentPackOutputFiles(dir)...)
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	if issues := ValidateBundle(dir); len(issues) != 0 {
		t.Fatalf("ValidateBundle issues = %#v, want none", issues)
	}
}

func TestValidateBundleRejectsInvalidContentPackSchema(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	writeTestPackFiles(t, dir, "content-pack/v0", []map[string]any{{
		"id":          "linkedin-dense",
		"brief_path":  "pack/linkedin-dense/brief.md",
		"output_path": "pack/linkedin-dense/final.png",
		"state":       "planned",
	}})
	manifest := validManifest(dir)
	manifest.OutputFiles = append(manifest.OutputFiles, contentPackOutputFiles(dir)...)
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "content-pack.json", "content-pack/v1") {
		t.Fatalf("ValidateBundle issues = %#v, want content-pack schema issue", issues)
	}
}

func TestValidateBundleRejectsUnsafePackTargetPaths(t *testing.T) {
	cases := []struct {
		name       string
		briefPath  string
		outputPath string
	}{
		{name: "brief traversal", briefPath: "../brief.md", outputPath: "pack/linkedin-dense/final.png"},
		{name: "brief nested traversal", briefPath: "pack/../brief.md", outputPath: "pack/linkedin-dense/final.png"},
		{name: "brief backslash", briefPath: `pack\linkedin-dense\brief.md`, outputPath: "pack/linkedin-dense/final.png"},
		{name: "output traversal", briefPath: "pack/linkedin-dense/brief.md", outputPath: "../final.png"},
		{name: "output nested traversal", briefPath: "pack/linkedin-dense/brief.md", outputPath: "pack/../final.png"},
		{name: "output backslash", briefPath: "pack/linkedin-dense/brief.md", outputPath: `pack\linkedin-dense\final.png`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
			writeTestPNG(t, filepath.Join(dir, "final.png"))
			writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
			writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
			writeTestPackFiles(t, dir, "content-pack/v1", []map[string]any{{
				"id":          "linkedin-dense",
				"brief_path":  tc.briefPath,
				"output_path": tc.outputPath,
				"state":       "planned",
			}})
			manifest := validManifest(dir)
			manifest.OutputFiles = append(manifest.OutputFiles, model.OutputFile{
				Kind:   "content_pack",
				Path:   "content-pack.json",
				SHA256: sha256File(filepath.Join(dir, "content-pack.json")),
			})
			writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

			issues := ValidateBundle(dir)
			if !hasIssueContaining(issues, "content-pack.json", "unsafe") {
				t.Fatalf("ValidateBundle issues = %#v, want unsafe target path issue", issues)
			}
		})
	}
}

func TestValidateBundleRejectsUnsafeDeclaredPackBriefPath(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	writeTestPackFiles(t, dir, "content-pack/v1", []map[string]any{{
		"id":          "linkedin-dense",
		"brief_path":  "pack/linkedin-dense/brief.md",
		"output_path": "pack/linkedin-dense/final.png",
		"state":       "planned",
	}})
	manifest := validManifest(dir)
	manifest.OutputFiles = append(manifest.OutputFiles,
		model.OutputFile{Kind: "content_pack", Path: "content-pack.json", SHA256: sha256File(filepath.Join(dir, "content-pack.json"))},
		model.OutputFile{Kind: "pack_brief", Path: `pack\linkedin-dense\brief.md`, SHA256: "missing"},
	)
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "unsafe path") {
		t.Fatalf("ValidateBundle issues = %#v, want unsafe declared pack brief path", issues)
	}
	if hasIssueContaining(issues, "manifest.json", "could not be checked") {
		t.Fatalf("ValidateBundle issues = %#v, should reject before hashing", issues)
	}
}

func TestValidateBundleRejectsPackImageForPlannedTargets(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	writeTestPackFiles(t, dir, "content-pack/v1", []map[string]any{{
		"id":          "linkedin-dense",
		"brief_path":  "pack/linkedin-dense/brief.md",
		"output_path": "pack/linkedin-dense/final.png",
		"state":       "planned",
	}})
	writeTestPNG(t, filepath.Join(dir, "pack", "linkedin-dense", "final.png"))
	manifest := validManifest(dir)
	manifest.OutputFiles = append(manifest.OutputFiles,
		model.OutputFile{Kind: "content_pack", Path: "content-pack.json", SHA256: sha256File(filepath.Join(dir, "content-pack.json"))},
		model.OutputFile{Kind: "pack_brief", Path: "pack/linkedin-dense/brief.md", SHA256: sha256File(filepath.Join(dir, "pack", "linkedin-dense", "brief.md"))},
		model.OutputFile{Kind: "pack_image", Path: "pack/linkedin-dense/final.png", SHA256: sha256File(filepath.Join(dir, "pack", "linkedin-dense", "final.png"))},
	)
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "pack_image") {
		t.Fatalf("ValidateBundle issues = %#v, want pack_image state issue", issues)
	}
}

func TestValidateBundleReportsMissingDeclaredPackHandoffPrompt(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.OutputFiles = append(manifest.OutputFiles, model.OutputFile{
		Kind:   "handoff_pack_prompt",
		Path:   "handoff/content-pack-codex-prompt.md",
		SHA256: "missing",
	})
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "handoff_pack_prompt") {
		t.Fatalf("ValidateBundle issues = %#v, want missing pack handoff prompt issue", issues)
	}
}

func TestValidateBundleAcceptsManifestAuditFields(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		Title:         "Acme Map",
		RequiredText:  []string{"Acme Map"},
	}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	writeValidManifest(t, dir)

	manifest := readManifestFixture(t, dir)
	manifest["audit"] = map[string]any{
		"tool_version":              "0.1.0",
		"requested_backend":         "auto",
		"selected_backend":          "local",
		"source_count":              float64(1),
		"evidence_item_count":       float64(2),
		"warning_count":             float64(0),
		"redaction_count":           float64(0),
		"remote_image_attempted":    true,
		"fallback_used":             true,
		"prompt_truncated":          true,
		"prompt_truncation_message": "prompt truncated to fit gpt-image-2 prompt limit",
	}
	writeManifestFixture(t, dir, manifest)

	issues := ValidateBundle(dir)
	if len(issues) != 0 {
		t.Fatalf("ValidateBundle() issues = %#v, want none", issues)
	}
}

func TestValidateBundleFailsWhenManifestAuditIsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{
			name: "missing-tool-version",
			mutate: func(audit map[string]any) {
				audit["tool_version"] = ""
			},
			want: "audit.tool_version",
		},
		{
			name: "backend-mismatch",
			mutate: func(audit map[string]any) {
				audit["selected_backend"] = "openai"
			},
			want: "audit.selected_backend",
		},
		{
			name: "source-count-mismatch",
			mutate: func(audit map[string]any) {
				audit["source_count"] = float64(2)
			},
			want: "audit.source_count",
		},
		{
			name: "negative-count",
			mutate: func(audit map[string]any) {
				audit["evidence_item_count"] = float64(-1)
			},
			want: "negative",
		},
		{
			name: "warning-count-mismatch",
			mutate: func(audit map[string]any) {
				audit["warning_count"] = float64(1)
			},
			want: "audit.warning_count",
		},
		{
			name: "missing-truncation-message",
			mutate: func(audit map[string]any) {
				audit["prompt_truncated"] = true
				delete(audit, "prompt_truncation_message")
			},
			want: "prompt_truncation_message",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
			writeTestPNG(t, filepath.Join(dir, "final.png"))
			writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
			writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
			writeValidManifest(t, dir)

			manifest := readManifestFixture(t, dir)
			audit, ok := manifest["audit"].(map[string]any)
			if !ok {
				t.Fatalf("audit fixture = %#v, want object", manifest["audit"])
			}
			tc.mutate(audit)
			writeManifestFixture(t, dir, manifest)

			issues := ValidateBundle(dir)
			if !hasIssueContaining(issues, "manifest.json", tc.want) {
				t.Fatalf("ValidateBundle() issues = %#v, want audit issue containing %q", issues, tc.want)
			}
		})
	}
}

func TestValidateBundleFailsWhenManifestAuditFieldsAreMissing(t *testing.T) {
	for _, field := range []string{
		"tool_version",
		"requested_backend",
		"selected_backend",
		"source_count",
		"evidence_item_count",
		"warning_count",
		"redaction_count",
		"remote_image_attempted",
		"fallback_used",
		"prompt_truncated",
	} {
		t.Run(field, func(t *testing.T) {
			dir := t.TempDir()
			packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
			writeTestPNG(t, filepath.Join(dir, "final.png"))
			writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
			writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
			writeValidManifest(t, dir)

			manifest := readManifestFixture(t, dir)
			audit, ok := manifest["audit"].(map[string]any)
			if !ok {
				t.Fatalf("audit fixture = %#v, want object", manifest["audit"])
			}
			delete(audit, field)
			writeManifestFixture(t, dir, manifest)

			issues := ValidateBundle(dir)
			if !hasIssueContaining(issues, "manifest.json", "audit."+field) {
				t.Fatalf("ValidateBundle() issues = %#v, want missing audit field issue for %q", issues, field)
			}
		})
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

func TestValidateBundleFailsWhenSourceDiagnosticsReferenceUnknownSource(t *testing.T) {
	dir := t.TempDir()
	packet := model.VisualPacket{SchemaVersion: "visual-packet/v1", Title: "Acme Map", RequiredText: []string{"Acme Map"}}
	writeTestPNG(t, filepath.Join(dir, "final.png"))
	writeJSON(t, filepath.Join(dir, "visual-packet.json"), packet)
	writeFile(t, filepath.Join(dir, "scaffold.html"), "<!doctype html><html><body>Acme Map</body></html>")
	manifest := validManifest(dir)
	manifest.SourceDiagnostics = []model.SourceDiagnostic{{
		SourceID:       "missing-source",
		Kind:           "pdf",
		Engine:         "pdftotext",
		Version:        "pdftotext 25.10.0",
		PagesAttempted: 1,
		PagesExtracted: 1,
	}}
	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)

	issues := ValidateBundle(dir)
	if !hasIssueContaining(issues, "manifest.json", "source_diagnostics") {
		t.Fatalf("ValidateBundle() issues = %#v, want source diagnostics issue", issues)
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

func writeTestPackFiles(t *testing.T, dir string, schemaVersion string, targets []map[string]any) {
	t.Helper()

	for _, target := range targets {
		briefPath, _ := target["brief_path"].(string)
		if strings.Contains(briefPath, "\\") || strings.HasPrefix(briefPath, "../") || filepath.IsAbs(briefPath) {
			continue
		}
		if briefPath == "" {
			continue
		}
		fullPath := filepath.Join(dir, filepath.FromSlash(briefPath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(fullPath), err)
		}
		writeFile(t, fullPath, "pack brief")
	}
	writeJSON(t, filepath.Join(dir, "content-pack.json"), map[string]any{
		"schema_version": schemaVersion,
		"source_packet":  "visual-packet.json",
		"title":          "Acme Map",
		"status":         "planned",
		"strategy": map[string]any{
			"planner": "deterministic",
			"summary": "test plan",
		},
		"targets": targets,
	})
}

func contentPackOutputFiles(dir string) []model.OutputFile {
	return []model.OutputFile{
		{Kind: "content_pack", Path: "content-pack.json", SHA256: sha256File(filepath.Join(dir, "content-pack.json"))},
		{Kind: "pack_brief", Path: "pack/linkedin-dense/brief.md", SHA256: sha256File(filepath.Join(dir, "pack", "linkedin-dense", "brief.md"))},
		{Kind: "pack_brief", Path: "pack/social-teaser/brief.md", SHA256: sha256File(filepath.Join(dir, "pack", "social-teaser", "brief.md"))},
		{Kind: "pack_brief", Path: "pack/blog-og/brief.md", SHA256: sha256File(filepath.Join(dir, "pack", "blog-og", "brief.md"))},
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
		NextSteps: []string{
			"Open scaffold.html first.",
		},
		Audit: model.ManifestAudit{
			ToolVersion:          "0.1.0",
			RequestedBackend:     "local",
			SelectedBackend:      "local",
			SourceCount:          1,
			EvidenceItemCount:    1,
			WarningCount:         0,
			RedactionCount:       0,
			RemoteImageAttempted: false,
			FallbackUsed:         false,
			PromptTruncated:      false,
		},
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

func readManifestFixture(t *testing.T, dir string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest.json) error = %v", err)
	}
	return manifest
}

func writeManifestFixture(t *testing.T, dir string, manifest map[string]any) {
	t.Helper()

	writeJSON(t, filepath.Join(dir, "manifest.json"), manifest)
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
