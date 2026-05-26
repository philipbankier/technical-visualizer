# Technical Visualizer v0.1 Evidence-First Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepare `technical-visualizer` for a product-quality v0.1 open-source release as an evidence-first visualization bundle CLI with honest OpenAI and Codex agent workflows.

**Architecture:** Keep the standalone Go CLI conservative: source gathering builds an auditable bundle, the OpenAI Images API is the explicit direct image backend, and Codex remains an agent handoff workflow unless a stable direct bridge is proven. The implementation adds audit fields, fixes behavior mismatches, improves CLI messaging, and makes the public repo release-ready without tagging or pushing the default branch.

**Tech Stack:** Go 1.26, standard library tests, GitHub Actions, shell scripts under `script/`, Markdown docs.

---

## File Structure

- Modify `internal/model/model.go`: add manifest audit structures and fields.
- Modify `internal/quality/quality.go`: validate the new manifest fields without making historical bundles ambiguous.
- Modify `internal/model/model_test.go`: update model fixture tests for new manifest fields.
- Modify `internal/quality/quality_test.go`: cover valid and invalid audit metadata.
- Modify `internal/backend/backend.go`: add prompt metadata to `ImageRequest` results if needed by the backend interface.
- Modify `internal/backend/openai.go`: expose prompt truncation state through a helper and keep `Generate` API stable unless tests show a small interface change is cleaner.
- Modify `internal/backend/backend_test.go`: cover prompt truncation metadata and keep `gpt-image-2` request behavior.
- Modify `internal/pipeline/pipeline.go`: fail on zero evidence, implement `auto` and `hybrid` fallback after OpenAI generation errors, populate manifest audit fields.
- Modify `internal/pipeline/pipeline_test.go`: add behavior tests for empty evidence, fallback after OpenAI failure, explicit OpenAI failure, and disk manifest audit fields.
- Modify `internal/cli/cli.go`: make `doctor` output separate local renderer, OpenAI Images API, and Codex CLI agent workflow lanes.
- Modify `internal/cli/cli_test.go`: assert doctor wording and Codex backend rejection.
- Modify `README.md`: rewrite first-run docs, backend matrix, Codex handoff, limits, examples.
- Create `docs/agent-workflows.md`: Codex and agent recipes, including built-in image handoff.
- Create `docs/backends.md`: local, offline, OpenAI, auto, hybrid, and Codex handoff behavior.
- Modify `docs/release-readiness.md`: make it public-facing and remove stale maintainer-only release decisions.
- Create `docs/maintainer-release.md`: move release checklist, tagging notes, and maintainer-only decisions here.
- Create `.github/ISSUE_TEMPLATE/bug_report.yml`: structured bug reports.
- Create `.github/ISSUE_TEMPLATE/feature_request.yml`: structured feature requests.
- Create `.github/pull_request_template.md`: concise PR checklist.
- Modify `.github/workflows/ci.yml`: run `script/security` with Go 1.26.3 rather than only `govulncheck`.
- Modify `.github/workflows/release.yml`: run `script/security` before release assets.
- Create `examples/sample-bundle/README.md`: explain the checked-in sample.
- Create `examples/sample-bundle/input/notes.md`: stable sample input.
- Create `examples/sample-bundle/output/scaffold.html`: generated sample scaffold.
- Create `examples/sample-bundle/output/visual-packet.json`: generated sample packet.
- Create `examples/sample-bundle/output/manifest.json`: generated sample manifest.

## Task 1: Manifest Audit Fields And Prompt Truncation

**Owner:** Subagent C.

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/quality/quality.go`
- Modify: `internal/model/model_test.go`
- Modify: `internal/quality/quality_test.go`
- Modify: `internal/backend/openai.go`
- Modify: `internal/backend/backend_test.go`

- [ ] **Step 1: Write failing model and quality tests for manifest audit fields**

Add this test to `internal/model/model_test.go` after the existing manifest JSON test:

```go
func TestManifestAuditFieldsMarshal(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "manifest/v1",
		Sources: []SourceSpec{{
			ID:    "src-abc",
			Kind:  SourceMarkdown,
			Input: "notes.md",
		}},
		Backend:  BackendInfo{Name: "local", Remote: false},
		Renderer: "html",
		Style:    "analytic",
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
	for _, want := range []string{
		`"audit"`,
		`"tool_version":"0.1.0"`,
		`"requested_backend":"auto"`,
		`"selected_backend":"local"`,
		`"source_count":1`,
		`"evidence_item_count":2`,
		`"warning_count":1`,
		`"remote_image_attempted":true`,
		`"fallback_used":true`,
		`"prompt_truncated":true`,
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("manifest JSON missing %s: %s", want, data)
		}
	}
}
```

Update the import list in `internal/model/model_test.go` to include `strings` if it is not already present.

Add this test to `internal/quality/quality_test.go` near the existing manifest validation tests:

```go
func TestValidateBundleAcceptsManifestAuditFields(t *testing.T) {
	dir := writeValidBundle(t)
	manifest := readManifestFixture(t, dir)
	manifest["audit"] = map[string]any{
		"tool_version":              "0.1.0",
		"requested_backend":         "auto",
		"selected_backend":          "local",
		"source_count":              float64(1),
		"evidence_item_count":       float64(2),
		"warning_count":             float64(1),
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
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/model ./internal/quality
```

Expected: `internal/model` fails because `ManifestAudit` and `Manifest.Audit` do not exist. `internal/quality` may also fail if the helper functions need minor adjustment for the new test.

- [ ] **Step 3: Add manifest audit types**

In `internal/model/model.go`, extend `Manifest` and add `ManifestAudit` below `Manifest`:

```go
type Manifest struct {
	SchemaVersion string        `json:"schema_version"`
	Sources       []SourceSpec  `json:"sources,omitempty"`
	Backend       BackendInfo   `json:"backend"`
	Renderer      string        `json:"renderer"`
	Style         string        `json:"style"`
	Warnings      []string      `json:"warnings,omitempty"`
	Audit         ManifestAudit `json:"audit"`
	OutputFiles   []OutputFile  `json:"output_files"`
}

type ManifestAudit struct {
	ToolVersion             string `json:"tool_version"`
	RequestedBackend        string `json:"requested_backend"`
	SelectedBackend         string `json:"selected_backend"`
	SourceCount             int    `json:"source_count"`
	EvidenceItemCount       int    `json:"evidence_item_count"`
	WarningCount            int    `json:"warning_count"`
	RedactionCount          int    `json:"redaction_count"`
	RemoteImageAttempted    bool   `json:"remote_image_attempted"`
	FallbackUsed            bool   `json:"fallback_used"`
	PromptTruncated         bool   `json:"prompt_truncated"`
	PromptTruncationMessage string `json:"prompt_truncation_message,omitempty"`
}
```

- [ ] **Step 4: Update quality manifest parsing**

In `internal/quality/quality.go`, add this struct near `manifestSummary`:

```go
type auditSummary struct {
	ToolVersion             string `json:"tool_version"`
	RequestedBackend        string `json:"requested_backend"`
	SelectedBackend         string `json:"selected_backend"`
	SourceCount             int    `json:"source_count"`
	EvidenceItemCount       int    `json:"evidence_item_count"`
	WarningCount            int    `json:"warning_count"`
	RedactionCount          int    `json:"redaction_count"`
	RemoteImageAttempted    bool   `json:"remote_image_attempted"`
	FallbackUsed            bool   `json:"fallback_used"`
	PromptTruncated         bool   `json:"prompt_truncated"`
	PromptTruncationMessage string `json:"prompt_truncation_message"`
}
```

Add this field to `manifestSummary`:

```go
Audit auditSummary `json:"audit"`
```

Add this validation block inside `validateManifest` after renderer/style validation:

```go
issues = append(issues, validateAudit(manifest.Audit, len(manifest.Sources), manifest.Backend.Name)...)
```

Add this helper below `validateManifest`:

```go
func validateAudit(audit auditSummary, sourceCount int, selectedBackend string) []Issue {
	var issues []Issue
	if strings.TrimSpace(audit.ToolVersion) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.tool_version must not be empty"})
	}
	if strings.TrimSpace(audit.RequestedBackend) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.requested_backend must not be empty"})
	}
	if strings.TrimSpace(audit.SelectedBackend) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.selected_backend must not be empty"})
	}
	if audit.SelectedBackend != selectedBackend {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.selected_backend must match backend.name"})
	}
	if audit.SourceCount != sourceCount {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.source_count must match sources length"})
	}
	if audit.EvidenceItemCount < 0 || audit.WarningCount < 0 || audit.RedactionCount < 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit counts must not be negative"})
	}
	if audit.PromptTruncated && strings.TrimSpace(audit.PromptTruncationMessage) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.prompt_truncation_message must be set when prompt_truncated is true"})
	}
	return issues
}
```

If fixture helpers in `internal/quality/quality_test.go` build manifests without audit data, update them to include:

```go
"audit": map[string]any{
	"tool_version":           "0.1.0",
	"requested_backend":      "local",
	"selected_backend":       "local",
	"source_count":           float64(1),
	"evidence_item_count":    float64(1),
	"warning_count":          float64(0),
	"redaction_count":        float64(0),
	"remote_image_attempted": false,
	"fallback_used":          false,
	"prompt_truncated":       false,
},
```

- [ ] **Step 5: Add prompt build metadata**

In `internal/backend/openai.go`, add:

```go
type PromptBuildResult struct {
	Prompt             string
	Truncated          bool
	TruncationMessage  string
}
```

Replace `buildPrompt` with:

```go
func buildPrompt(request ImageRequest) (string, error) {
	result, err := BuildPrompt(request)
	if err != nil {
		return "", err
	}
	return result.Prompt, nil
}

func BuildPrompt(request ImageRequest) (PromptBuildResult, error) {
	contentBrief := strings.TrimSpace(request.Prompt)
	scaffoldHTML := strings.TrimSpace(request.ScaffoldHTML)
	if contentBrief == "" && scaffoldHTML == "" {
		return PromptBuildResult{}, errors.New("openai image generation requires a prompt or scaffold HTML")
	}

	var sections []string
	if contentBrief != "" {
		sections = append(sections, "Content brief:\n"+contentBrief)
	}
	if scaffoldHTML != "" {
		sections = append(sections, "Audit scaffold HTML:\n"+scaffoldHTML)
	}

	prompt, truncated, message := capPrompt(strings.Join(sections, "\n\n"))
	return PromptBuildResult{Prompt: prompt, Truncated: truncated, TruncationMessage: message}, nil
}
```

Replace `capPrompt` with:

```go
func capPrompt(prompt string) (string, bool, string) {
	if runeCount(prompt) <= maxPromptRunes {
		return prompt, false, ""
	}

	marker := "\n\n[truncated to fit gpt-image-2 prompt limit]"
	limit := maxPromptRunes - runeCount(marker)
	if limit <= 0 {
		return takeRunes(prompt, maxPromptRunes), true, "prompt truncated to fit gpt-image-2 prompt limit"
	}
	return takeRunes(prompt, limit) + marker, true, "prompt truncated to fit gpt-image-2 prompt limit"
}
```

- [ ] **Step 6: Update backend tests**

In `internal/backend/backend_test.go`, update `TestOpenAIBackendCapsPromptLength` only if needed for the new `capPrompt` signature. Add:

```go
func TestBuildPromptReportsTruncation(t *testing.T) {
	result, err := BuildPrompt(ImageRequest{
		Prompt:       strings.Repeat("a", maxPromptRunes),
		ScaffoldHTML: strings.Repeat("b", 200),
		OutputPath:   filepath.Join(t.TempDir(), "final.png"),
	})
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}
	if !result.Truncated {
		t.Fatalf("BuildPrompt() Truncated = false, want true")
	}
	if result.TruncationMessage == "" {
		t.Fatalf("BuildPrompt() TruncationMessage empty")
	}
	if got := len([]rune(result.Prompt)); got != maxPromptRunes {
		t.Fatalf("prompt length = %d, want %d", got, maxPromptRunes)
	}
}
```

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/model ./internal/quality ./internal/backend
```

Expected: all three packages pass.

- [ ] **Step 8: Commit Task 1**

Run:

```bash
git add internal/model/model.go internal/model/model_test.go internal/quality/quality.go internal/quality/quality_test.go internal/backend/openai.go internal/backend/backend_test.go
git commit -m "feat: add manifest audit metadata"
```

Expected: commit succeeds.

## Task 2: Pipeline Behavior For Fallbacks And Empty Evidence

**Owner:** Subagent B.

**Files:**

- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/pipeline_test.go`

- [ ] **Step 1: Write failing tests for empty evidence and backend fallback**

Add these tests to `internal/pipeline/pipeline_test.go` before `TestRunLocalHTMLWritesBundle`:

```go
func TestRunFailsWhenAllSourcesProduceNoEvidence(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "missing.md")
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want zero evidence failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no evidence") {
		t.Fatalf("Run() error = %q, want no evidence explanation", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("manifest.json exists after zero evidence failure, stat error = %v", statErr)
	}
}

func TestRunAutoFallsBackWhenOpenAIGenerationFails(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nFallback should preserve a local bundle."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "auto",
		Renderer:  "image",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local fallback", manifest.Backend)
	}
	if !manifest.Audit.RemoteImageAttempted {
		t.Fatalf("RemoteImageAttempted = false, want true")
	}
	if !manifest.Audit.FallbackUsed {
		t.Fatalf("FallbackUsed = false, want true")
	}
	if !hasWarningContaining(manifest.Warnings, "openai generation failed") {
		t.Fatalf("warnings = %#v, want OpenAI fallback warning", manifest.Warnings)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "final.png")); err != nil {
		t.Fatalf("Stat(final.png) error = %v", err)
	}
}

func TestRunExplicitOpenAIFailsWhenGenerationFails(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nExplicit OpenAI should fail fast."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "openai",
		Renderer:  "image",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want explicit OpenAI failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "openai") {
		t.Fatalf("Run() error = %q, want OpenAI explanation", err)
	}
}
```

Add this helper at the bottom of `internal/pipeline/pipeline_test.go`:

```go
func hasWarningContaining(warnings []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), needle) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/pipeline
```

Expected: the empty evidence test fails because a success bundle is written, and the fallback test fails because `auto` currently returns the OpenAI error.

- [ ] **Step 3: Add image generation outcome data**

In `internal/pipeline/pipeline.go`, add this type near `Options`:

```go
type imageGenerationResult struct {
	BackendInfo             model.BackendInfo
	Warnings                []string
	RemoteImageAttempted    bool
	FallbackUsed            bool
	PromptTruncated         bool
	PromptTruncationMessage string
}
```

Change `generateImage` signature to:

```go
func generateImage(ctx context.Context, opts Options, visualPacket model.VisualPacket, packetJSON string, scaffoldHTML string, outputPath string) (imageGenerationResult, error) {
```

Update call sites in `Run` from:

```go
backendInfo, backendWarnings, err := generateImage(ctx, opts, visualPacket, string(packetJSON), string(scaffoldHTML), imagePath)
```

to:

```go
imageResult, err := generateImage(ctx, opts, visualPacket, string(packetJSON), string(scaffoldHTML), imagePath)
```

- [ ] **Step 4: Fail when no evidence is gathered**

In `Run`, after `source.GatherAll` and before `publicEvidenceBundle`, add:

```go
if len(evidence.Items) == 0 {
	return model.Manifest{}, fmt.Errorf("no evidence gathered from %d source(s); warnings: %s", len(evidence.Sources), formatWarnings(evidence.Warnings))
}
```

Add this helper near `formatIssues`:

```go
func formatWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return "none"
	}
	return strings.Join(warnings, "; ")
}
```

- [ ] **Step 5: Implement fallback behavior**

Inside `generateImage`, preserve the local/html/offline branches but return `imageGenerationResult`. For the `auto` and `hybrid` branch, use this structure:

```go
case "auto", "hybrid":
	client := backend.NewOpenAIBackend(backend.OpenAIConfig{})
	capability := client.Available(ctx)
	if capability.Available {
		promptResult, err := backend.BuildPrompt(backend.ImageRequest{Prompt: prompt, ScaffoldHTML: scaffoldHTML, OutputPath: outputPath})
		if err != nil {
			return imageGenerationResult{}, err
		}
		err = client.Generate(ctx, backend.ImageRequest{Prompt: prompt, ScaffoldHTML: scaffoldHTML, OutputPath: outputPath})
		if err == nil {
			return imageGenerationResult{
				BackendInfo:             model.BackendInfo{Name: client.Name(), Remote: true, Model: "gpt-image-2"},
				RemoteImageAttempted:    true,
				PromptTruncated:         promptResult.Truncated,
				PromptTruncationMessage: promptResult.TruncationMessage,
			}, nil
		}
		if fallbackErr := render.WriteFallbackPNG(outputPath, visualPacket); fallbackErr != nil {
			return imageGenerationResult{}, fallbackErr
		}
		return imageGenerationResult{
			BackendInfo:          model.BackendInfo{Name: "local", Remote: false},
			Warnings:             []string{"OpenAI generation failed, used local fallback: " + err.Error()},
			RemoteImageAttempted: true,
			FallbackUsed:         true,
			PromptTruncated:      promptResult.Truncated,
			PromptTruncationMessage: promptResult.TruncationMessage,
		}, nil
	}
	if err := render.WriteFallbackPNG(outputPath, visualPacket); err != nil {
		return imageGenerationResult{}, err
	}
	return imageGenerationResult{
		BackendInfo:  model.BackendInfo{Name: "local", Remote: false},
		Warnings:     []string{"OpenAI backend unavailable, used local fallback: " + capability.Reason},
		FallbackUsed: true,
	}, nil
```

For the explicit `openai` branch, compute `promptResult` before calling `Generate`, return the OpenAI backend when successful, and keep returning the error when generation fails.

- [ ] **Step 6: Populate manifest audit data**

In `Run`, replace `backendInfo` and `backendWarnings` references with `imageResult`. Build the warnings before manifest construction:

```go
warnings := append(append([]string(nil), publicEvidence.Warnings...), imageResult.Warnings...)
```

Set `Backend` and `Audit` in the manifest:

```go
Backend: imageResult.BackendInfo,
Warnings: warnings,
Audit: model.ManifestAudit{
	ToolVersion:             "0.1.0",
	RequestedBackend:        opts.Backend,
	SelectedBackend:         imageResult.BackendInfo.Name,
	SourceCount:             len(publicEvidence.Sources),
	EvidenceItemCount:       len(publicEvidence.Items),
	WarningCount:            len(warnings),
	RedactionCount:          len(publicEvidence.Redactions),
	RemoteImageAttempted:    imageResult.RemoteImageAttempted,
	FallbackUsed:            imageResult.FallbackUsed,
	PromptTruncated:         imageResult.PromptTruncated,
	PromptTruncationMessage: imageResult.PromptTruncationMessage,
},
```

Update `writeManifestFile` to include `Audit model.ManifestAudit` in its private `manifestJSON` struct and to set `Audit: manifest.Audit`.

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/pipeline ./internal/quality
```

Expected: tests pass.

- [ ] **Step 8: Commit Task 2**

Run:

```bash
git add internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go internal/quality/quality.go internal/quality/quality_test.go
git commit -m "fix: harden pipeline fallback behavior"
```

Expected: commit succeeds.

## Task 3: Doctor Output And Codex Agent Handoff Messaging

**Owner:** Subagent C.

**Files:**

- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/backend/codex.go`
- Modify: `internal/backend/backend_test.go`

- [ ] **Step 1: Write failing doctor wording test**

Replace the content assertion in `TestRunDoctorReportsBackendStatusWithoutRemoteCredentials` with:

```go
output := stdout.String()
for _, want := range []string{
	"Backend status",
	"local renderer",
	"OpenAI Images API",
	"Codex CLI",
	"agent workflow",
	"not a direct image backend",
} {
	if !strings.Contains(strings.ToLower(output), strings.ToLower(want)) {
		t.Fatalf("doctor output missing %q: %q", want, output)
	}
}
```

Update `TestRunCodexBackendReturnsClearErrorBeforeBundleWrite` to assert:

```go
if !strings.Contains(strings.ToLower(stderr.String()), "codex") ||
	!strings.Contains(strings.ToLower(stderr.String()), "agent workflow") ||
	!strings.Contains(strings.ToLower(stderr.String()), "openai") {
	t.Fatalf("stderr missing codex agent-workflow explanation: %q", stderr.String())
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/cli
```

Expected: doctor wording test fails because the current output uses generic `local`, `openai`, and `codex` labels.

- [ ] **Step 3: Update Codex backend error**

In `internal/backend/codex.go`, replace `Generate` with:

```go
func (b *CodexBackend) Generate(context.Context, ImageRequest) error {
	return errors.New("codex is available only as an agent workflow in v0.1; use --backend openai for direct API image generation or run Codex against the generated visual-packet.json and scaffold.html")
}
```

Update `TestCodexBackendGenerateIsNotWired` in `internal/backend/backend_test.go` to assert `agent workflow` instead of `not wired`.

- [ ] **Step 4: Update doctor output**

In `internal/cli/cli.go`, replace `printDoctor` with:

```go
func printDoctor(w io.Writer) {
	ctx := context.Background()
	openAI := backend.NewOpenAIBackend(backend.OpenAIConfig{}).Available(ctx)
	codex := backend.NewCodexBackend(backend.CodexConfig{}).Available(ctx)

	fmt.Fprintln(w, "Backend status")
	fmt.Fprintln(w, "local renderer: available (local, fallback PNG and scaffold output)")
	printNamedCapability(w, "OpenAI Images API", openAI, "direct image backend for --backend openai")
	printNamedCapability(w, "Codex CLI", codex, "agent workflow only, not a direct image backend")
}
```

Add this helper below `printCapability` or replace `printCapability` with it:

```go
func printNamedCapability(w io.Writer, label string, capability backend.Capability, detail string) {
	status := "unavailable"
	if capability.Available {
		status = "available"
	}
	remote := "local"
	if capability.Remote {
		remote = "remote"
	}
	reason := strings.TrimSpace(capability.Reason)
	if reason == "" {
		reason = "no details"
	}
	fmt.Fprintf(w, "%s: %s (%s, %s; %s)\n", label, status, remote, reason, detail)
}
```

Remove `printCapability` if it is no longer used.

- [ ] **Step 5: Update unsupported backend error**

In `internal/pipeline/pipeline.go`, replace the `codex` validation error with:

```go
return errors.New("codex is an agent workflow in v0.1, not a direct image backend; use visualize doctor to check Codex availability, --backend openai for direct API image generation, or --backend local for private local output")
```

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/cli ./internal/backend ./internal/pipeline
```

Expected: tests pass.

- [ ] **Step 7: Commit Task 3**

Run:

```bash
git add internal/cli/cli.go internal/cli/cli_test.go internal/backend/codex.go internal/backend/backend_test.go internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go
git commit -m "fix: clarify Codex agent workflow status"
```

Expected: commit succeeds.

## Task 4: Public Docs, Agent Workflows, And Release Notes

**Owner:** Subagent A.

**Files:**

- Modify: `README.md`
- Create: `docs/agent-workflows.md`
- Create: `docs/backends.md`
- Modify: `docs/release-readiness.md`
- Create: `docs/maintainer-release.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Rewrite README around first run**

Replace `README.md` with this structure and keep command examples exact:

````markdown
# technical-visualizer

`technical-visualizer` is a Go CLI that turns URLs or local technical sources into an auditable visualization bundle.

The v0.1 release is evidence-first. It gathers source context, writes a `visual-packet.json`, builds an inspectable `scaffold.html`, records what happened in `manifest.json`, and writes `final.png` as either an OpenAI-generated image or a local fallback preview.

## Quickstart

```bash
go install github.com/philipbankier/technical-visualizer/cmd/visualize@latest
visualize --out visualize-output https://github.com/philipbankier/technical-visualizer
```

Inspect `visualize-output/scaffold.html` first. It is the local auditable visualization scaffold.

## Local And Offline

```bash
visualize --backend local --renderer html --offline --out /tmp/visualize-smoke ./testdata/sample-repo ./testdata/notes.md
```

Offline mode skips remote source fetching and remote image generation.

## OpenAI Image Generation

```bash
OPENAI_API_KEY=... visualize --backend openai --renderer image --out visualize-output https://github.com/example/service
```

`--backend openai` calls the OpenAI Images API with `gpt-image-2`. It sends source-derived visual packet content and scaffold HTML to OpenAI. Use it only for content that can be processed remotely.

## Codex And ChatGPT Subscriptions

ChatGPT or Codex subscription access is separate from `OPENAI_API_KEY`.

- `--backend openai` is the direct Go CLI API path and requires `OPENAI_API_KEY`.
- Codex can be used as an agent around this tool.
- In supported Codex environments, Codex may have a built-in image-generation tool that does not require `OPENAI_API_KEY`.
- v0.1 does not support `visualize --backend codex`.

Codex handoff:

```bash
visualize --backend local --renderer html --out visualize-output https://github.com/example/service
codex -C . "Inspect visualize-output/visual-packet.json and visualize-output/scaffold.html, then generate a polished technical visualization image using your image generation tool. Save the final PNG back into visualize-output/final.png."
```

## Output Files

- `scaffold.html`: auditable local visualization scaffold
- `visual-packet.json`: source-backed renderer packet
- `manifest.json`: sources, backend, warnings, audit metadata, and output hashes
- `final.png`: OpenAI image output or deterministic local fallback preview

## Backends

| Backend  | Remote source fetch | Remote image call | Behavior |
| -------- | ------------------- | ----------------- | -------- |
| `local`  | yes, unless `--offline` | no              | writes scaffold and local fallback preview |
| `openai` | yes                 | yes               | calls OpenAI Images API and fails if generation fails |
| `auto`   | yes                 | when credentials exist | tries OpenAI, falls back locally with warning |
| `hybrid` | yes                 | when credentials exist | tries OpenAI, falls back locally with warning |

## Inputs

Supported v0.1 inputs:

- GitHub repo URLs
- Docs site URLs
- Markdown files and URLs
- JSON files as evidence inputs
- PDF files and URLs, accepted with warnings because text extraction is not implemented in v0.1
- Local repo paths

Direct local inputs that look like secrets, credentials, or private keys are rejected. Repo scans skip secret-like files, generated directories, symlinks, large files, and binary-looking files by default.

## Privacy

The default backend is `local`. Source-derived content is not uploaded just because `OPENAI_API_KEY` is present.

Use `--offline` to avoid remote source fetching and remote image generation. Explicit `--backend openai --offline` is rejected before bundle files are written.

## Development

Use Go 1.26.3 or newer for release checks.

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
```

The current release unit is this directory as a standalone repository root. Do not publish the wider parent workspace as this tool's GitHub repo.
````

- [ ] **Step 2: Add agent workflow doc**

Create `docs/agent-workflows.md`:

````markdown
# Agent Workflows

Agents are useful around `technical-visualizer` because the tool writes source-backed intermediate files that are easy to inspect.

## Local Review Workflow

```bash
visualize --backend local --renderer html --out visualize-output https://github.com/example/service
codex -C . "Review visualize-output/manifest.json and visualize-output/scaffold.html. Identify missing evidence, weak claims, and any privacy risks."
```

## Codex Image Handoff

In supported Codex environments, Codex may expose a built-in image-generation tool. That is different from the standalone Go CLI calling OpenAI directly.

```bash
visualize --backend local --renderer html --out visualize-output https://github.com/example/service
codex -C . "Use visualize-output/visual-packet.json and visualize-output/scaffold.html as source material. Generate one polished technical visualization image. Save the selected result as visualize-output/final.png."
```

Use this path when you want to use Codex or ChatGPT subscription access through the agent environment.

## Direct API Image Workflow

```bash
OPENAI_API_KEY=... visualize --backend openai --renderer image --out visualize-output https://github.com/example/service
```

Use this path when you want the Go CLI itself to call the OpenAI Images API. This requires `OPENAI_API_KEY` and can incur API usage.

## Boundaries

- `visualize --backend codex` is not supported in v0.1.
- A ChatGPT or Codex subscription does not automatically pay for `--backend openai`.
- Agent-generated images should be saved back into the bundle with a clear filename.
- Do not hand private source to a remote agent or remote image tool unless that is acceptable for the project.
````

- [ ] **Step 3: Add backend behavior doc**

Create `docs/backends.md`:

```markdown
# Backends

`technical-visualizer` separates source gathering from image generation.

## `local`

`local` never calls a remote image provider. It writes `scaffold.html`, `visual-packet.json`, `manifest.json`, and a deterministic fallback `final.png` preview.

## `openai`

`openai` calls the OpenAI Images API with `gpt-image-2`. It requires `OPENAI_API_KEY`. It fails if the remote generation call fails.

## `auto`

`auto` uses OpenAI when `OPENAI_API_KEY` is set. If OpenAI is unavailable or generation fails, it writes the local fallback preview and records a manifest warning.

## `hybrid`

`hybrid` follows the same fallback behavior as `auto`.

## Codex Agent Handoff

Codex is not a direct backend in v0.1. Use Codex as an agent to inspect the generated packet and scaffold, then generate or revise images using tools available in that Codex environment.

## Offline

`--offline` skips remote source fetching and remote image generation. `--backend openai --offline` is rejected because it asks for mutually exclusive behavior.
```

- [ ] **Step 4: Split release readiness docs**

Update `docs/release-readiness.md` so it is public-facing:

````markdown
# Release Readiness

Date: 2026-05-26

Scope: this repository as `github.com/philipbankier/technical-visualizer`.

## Current Release Position

`technical-visualizer` is being prepared for v0.1 as an evidence-first local CLI with optional OpenAI image generation.

## Release Gates

Run from the repository root:

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
```

## Public Safety Claims

- Default backend is local.
- Explicit OpenAI generation requires `OPENAI_API_KEY`.
- Generated bundles can contain source-derived content.
- The CLI is not ready to expose as a hosted service without private-network URL guards, authentication, and source allowlists.

## Not Released Yet

Do not tag `v0.1.0` until the release branch passes local gates and GitHub Actions.
````

Create `docs/maintainer-release.md`:

````markdown
# Maintainer Release Notes

This file is for maintainers preparing v0.1.

## Before Tagging

- Confirm the feature branch has passed GitHub Actions.
- Confirm temporary API keys used for smoke tests have been deleted or rotated.
- Confirm no generated bundle contains secrets or absolute local paths.
- Confirm repo metadata is set on GitHub.

## Manual Release

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds darwin, linux, and windows assets and publishes a GitHub release from the tag.
````

- [ ] **Step 5: Update AGENTS project notes**

Append this bullet to `AGENTS.md`:

```markdown
- Document Codex image generation as an agent handoff workflow unless a direct, tested CLI/API bridge is added.
```

- [ ] **Step 6: Scan docs for forbidden claims**

Run:

```bash
rg -n "backend codex|--backend codex.*works|subscription.*OPENAI_API_KEY|temp api key|temporary API key supplied" README.md docs AGENTS.md
```

Expected: no claims that `--backend codex` works. Mentions of `--backend codex` must say it is unsupported in v0.1. No temp API key text should appear in public docs.

- [ ] **Step 7: Commit Task 4**

Run:

```bash
git add README.md docs/agent-workflows.md docs/backends.md docs/release-readiness.md docs/maintainer-release.md AGENTS.md
git commit -m "docs: clarify release and agent workflows"
```

Expected: commit succeeds.

## Task 5: Examples, GitHub Templates, And Workflow Gates

**Owner:** Subagent A for docs/templates, main agent for generated sample verification.

**Files:**

- Create: `examples/sample-bundle/README.md`
- Create: `examples/sample-bundle/input/notes.md`
- Create: `examples/sample-bundle/output/scaffold.html`
- Create: `examples/sample-bundle/output/visual-packet.json`
- Create: `examples/sample-bundle/output/manifest.json`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/pull_request_template.md`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Add stable example input**

Create `examples/sample-bundle/input/notes.md`:

```markdown
# Acme Orders Service

The Acme Orders Service accepts order requests, validates inventory, stores order state, and publishes fulfillment events.

## Components

- HTTP API receives order submissions.
- Inventory client checks stock before confirmation.
- Orders database stores order status.
- Event publisher emits fulfillment events.

## Risks

- Inventory failures delay confirmation.
- Duplicate submissions require idempotency keys.
- Event publishing must not happen before order state is committed.
```

Create `examples/sample-bundle/README.md`:

````markdown
# Sample Bundle

This sample shows the shape of a local `technical-visualizer` output bundle.

Regenerate it from the repository root:

```bash
go run ./cmd/visualize --backend local --renderer html --offline --out examples/sample-bundle/output examples/sample-bundle/input/notes.md
```

Inspect `output/scaffold.html` first. `output/final.png` is intentionally not checked in because the local PNG is only a fallback preview in v0.1.
````

- [ ] **Step 2: Generate sample output**

Run:

```bash
rm -rf examples/sample-bundle/output
go run ./cmd/visualize --backend local --renderer html --offline --out examples/sample-bundle/output examples/sample-bundle/input/notes.md
rm -f examples/sample-bundle/output/final.png
```

Expected: `scaffold.html`, `visual-packet.json`, and `manifest.json` exist under `examples/sample-bundle/output`; `final.png` is removed before commit to avoid checking in a fallback binary preview.

- [ ] **Step 3: Add issue templates**

Create `.github/ISSUE_TEMPLATE/bug_report.yml`:

```yaml
name: Bug report
description: Report a reproducible problem with technical-visualizer.
title: "bug: "
labels: ["bug"]
body:
  - type: textarea
    id: command
    attributes:
      label: Command
      description: Paste the exact command you ran.
      render: bash
    validations:
      required: true
  - type: textarea
    id: expected
    attributes:
      label: Expected behavior
    validations:
      required: true
  - type: textarea
    id: actual
    attributes:
      label: Actual behavior
    validations:
      required: true
  - type: textarea
    id: manifest
    attributes:
      label: Manifest or warnings
      description: Paste relevant manifest warnings. Do not paste secrets.
      render: json
```

Create `.github/ISSUE_TEMPLATE/feature_request.yml`:

```yaml
name: Feature request
description: Suggest a focused improvement.
title: "feat: "
labels: ["enhancement"]
body:
  - type: textarea
    id: problem
    attributes:
      label: Problem
      description: What are you trying to do?
    validations:
      required: true
  - type: textarea
    id: proposal
    attributes:
      label: Proposed behavior
    validations:
      required: true
  - type: dropdown
    id: area
    attributes:
      label: Area
      options:
        - Source gathering
        - Visual packet
        - Local rendering
        - OpenAI backend
        - Codex or agent workflow
        - Documentation
```

- [ ] **Step 4: Add PR template**

Create `.github/pull_request_template.md`:

```markdown
This PR...

Checklist:

- [ ] Keeps the default backend local unless remote generation is explicit.
- [ ] Updates tests for source gathering, backend selection, manifest output, or docs changed here.
- [ ] Does not commit generated caches, API keys, or private bundle output.
- [ ] Ran `script/lint`, `script/test`, and `script/smoke`.
```

- [ ] **Step 5: Update CI workflow**

In `.github/workflows/ci.yml`, replace the final `govulncheck` step:

```yaml
      - run: go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

with:

```yaml
      - run: script/security
```

Keep `go-version: "1.26.3"`.

- [ ] **Step 6: Update release workflow**

In `.github/workflows/release.yml`, add after `script/smoke`:

```yaml
      - run: script/security
```

- [ ] **Step 7: Run focused validation**

Run:

```bash
script/lint
script/test
script/smoke
```

Expected: all three pass.

- [ ] **Step 8: Commit Task 5**

Run:

```bash
git add examples .github README.md docs AGENTS.md
git commit -m "chore: add release-ready examples and templates"
```

Expected: commit succeeds.

## Task 6: Integration Validation, Remote Branch, And PR

**Owner:** Main agent.

**Files:**

- No planned source edits unless validation exposes defects.

- [ ] **Step 1: Check worktree and branch**

Run:

```bash
git status --short --branch
git rev-parse --abbrev-ref HEAD
gh repo view philipbankier/technical-visualizer --json defaultBranchRef,url
```

Expected: branch is `codex/v0.1-evidence-hardening`. Default branch remains `codex/oss-release-prep`; do not push to it.

- [ ] **Step 2: Run complete local gates**

Run:

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
```

Expected: every command exits 0. Coverage output should list all Go packages.

- [ ] **Step 3: Run URL smoke**

Run:

```bash
rm -rf /tmp/technical-visualizer-url-smoke
go run ./cmd/visualize --backend local --renderer html --out /tmp/technical-visualizer-url-smoke https://github.com/philipbankier/technical-visualizer
test -s /tmp/technical-visualizer-url-smoke/scaffold.html
test -s /tmp/technical-visualizer-url-smoke/visual-packet.json
test -s /tmp/technical-visualizer-url-smoke/manifest.json
```

Expected: command succeeds and generated files exist.

- [ ] **Step 4: Run missing evidence smoke**

Run:

```bash
rm -rf /tmp/technical-visualizer-empty-smoke
if go run ./cmd/visualize --backend local --renderer html --out /tmp/technical-visualizer-empty-smoke /tmp/definitely-missing-technical-visualizer.md; then
  echo "expected missing evidence failure" >&2
  exit 1
fi
test ! -e /tmp/technical-visualizer-empty-smoke/manifest.json
```

Expected: command exits non-zero and `manifest.json` does not exist.

- [ ] **Step 5: Inspect manifest audit fields**

Run:

```bash
jq '.audit | {tool_version, requested_backend, selected_backend, source_count, evidence_item_count, warning_count, redaction_count, remote_image_attempted, fallback_used, prompt_truncated}' /tmp/technical-visualizer-url-smoke/manifest.json
```

Expected: audit fields are present. `requested_backend` and `selected_backend` should match the URL smoke behavior.

- [ ] **Step 6: Scan for stale release and secret language**

Run:

```bash
rg -n "sk-[A-Za-z0-9_-]+|temporary API key supplied|temp api key|confirm the final GitHub repo name|backend codex.*works|subscription.*pays" .
```

Expected: no secrets. Any remaining `temporary API key` text must be in maintainer-only context and must not include a key value.

- [ ] **Step 7: Set GitHub repo metadata**

Run:

```bash
gh repo edit philipbankier/technical-visualizer \
  --description "Go CLI for source-backed technical visualization bundles" \
  --homepage "https://github.com/philipbankier/technical-visualizer" \
  --add-topic go \
  --add-topic cli \
  --add-topic visualization \
  --add-topic architecture \
  --add-topic openai \
  --add-topic technical-diagrams \
  --enable-wiki=false \
  --enable-projects=false
```

Expected: repo metadata updates successfully. This does not push to any branch.

- [ ] **Step 8: Push feature branch**

Run:

```bash
git push -u origin codex/v0.1-evidence-hardening
```

Expected: push succeeds. Do not push `codex/oss-release-prep` and do not create tags.

- [ ] **Step 9: Open PR**

Run:

```bash
gh pr create \
  --repo philipbankier/technical-visualizer \
  --base codex/oss-release-prep \
  --head codex/v0.1-evidence-hardening \
  --title "chore: prepare v0.1 evidence-first release" \
  --body "This PR prepares technical-visualizer for a v0.1 evidence-first open-source release. It aligns backend behavior with docs, clarifies OpenAI and Codex agent workflows, adds release-ready repo docs and templates, and updates validation gates."
```

Expected: PR is created. Open the returned URL in the default browser.

- [ ] **Step 10: Check GitHub Actions**

Run:

```bash
gh run list --repo philipbankier/technical-visualizer --branch codex/v0.1-evidence-hardening --limit 5
```

Expected: CI run appears. If still in progress, wait until it completes:

```bash
gh run watch --repo philipbankier/technical-visualizer
```

Expected: CI completes successfully.

- [ ] **Step 11: Final completion audit**

Inspect:

```bash
git status --short --branch
gh pr view --repo philipbankier/technical-visualizer --json url,state,headRefName,baseRefName,statusCheckRollup
```

Expected:

- Worktree clean.
- PR branch is `codex/v0.1-evidence-hardening`.
- PR base is `codex/oss-release-prep`.
- Checks are successful.
- No tag was created.
- No default branch push was made.

If all expectations are met, report the PR URL and local validation evidence.
