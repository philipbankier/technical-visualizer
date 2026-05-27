package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Issue struct {
	Path    string
	Message string
}

func ValidateBundle(dir string) []Issue {
	var issues []Issue
	for _, name := range []string{"final.png", "scaffold.html", "visual-packet.json", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, Issue{Path: name, Message: "required file is missing"})
				continue
			}
			issues = append(issues, Issue{Path: name, Message: err.Error()})
		}
	}

	if !hasIssue(issues, "final.png") {
		issues = append(issues, validatePNG(filepath.Join(dir, "final.png"))...)
	}
	if !hasIssue(issues, "manifest.json") {
		issues = append(issues, validateManifest(dir)...)
	}

	packet, ok := readPacket(filepath.Join(dir, "visual-packet.json"), &issues)
	if ok && !hasIssue(issues, "scaffold.html") {
		issues = append(issues, validateScaffold(filepath.Join(dir, "scaffold.html"), packet)...)
	}

	return issues
}

type packetSummary struct {
	Title        string   `json:"title"`
	RequiredText []string `json:"required_text"`
}

func validatePNG(path string) []Issue {
	// #nosec G304 -- quality validation reads the generated bundle file selected by the caller.
	file, err := os.Open(path)
	if err != nil {
		return []Issue{{Path: "final.png", Message: err.Error()}}
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return []Issue{{Path: "final.png", Message: "final.png is not a valid png: " + err.Error()}}
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return []Issue{{Path: "final.png", Message: fmt.Sprintf("final.png has zero dimensions: %dx%d", bounds.Dx(), bounds.Dy())}}
	}
	return nil
}

type manifestSummary struct {
	SchemaVersion     string                    `json:"schema_version"`
	Sources           []sourceSummary           `json:"sources"`
	Backend           backendSummary            `json:"backend"`
	Renderer          string                    `json:"renderer"`
	Style             string                    `json:"style"`
	Warnings          []string                  `json:"warnings"`
	NextSteps         []string                  `json:"next_steps"`
	Audit             auditSummary              `json:"audit"`
	SourceDiagnostics []sourceDiagnosticSummary `json:"source_diagnostics"`
	OutputFiles       []outputFileSummary       `json:"output_files"`
}

type auditSummary struct {
	ToolVersion             *string `json:"tool_version"`
	RequestedBackend        *string `json:"requested_backend"`
	SelectedBackend         *string `json:"selected_backend"`
	SourceCount             *int    `json:"source_count"`
	EvidenceItemCount       *int    `json:"evidence_item_count"`
	WarningCount            *int    `json:"warning_count"`
	RedactionCount          *int    `json:"redaction_count"`
	RemoteImageAttempted    *bool   `json:"remote_image_attempted"`
	FallbackUsed            *bool   `json:"fallback_used"`
	PromptTruncated         *bool   `json:"prompt_truncated"`
	PromptTruncationMessage *string `json:"prompt_truncation_message"`
}

type sourceSummary struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Input string `json:"input"`
}

type sourceDiagnosticSummary struct {
	SourceID       string   `json:"source_id"`
	Kind           string   `json:"kind"`
	Engine         string   `json:"engine"`
	Version        string   `json:"version"`
	PagesAttempted int      `json:"pages_attempted"`
	PagesExtracted int      `json:"pages_extracted"`
	Truncated      bool     `json:"truncated"`
	Warnings       []string `json:"warnings"`
}

type backendSummary struct {
	Name string `json:"name"`
}

type outputFileSummary struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type contentPackSummary struct {
	SchemaVersion string              `json:"schema_version"`
	Targets       []packTargetSummary `json:"targets"`
}

type packTargetSummary struct {
	ID         string `json:"id"`
	BriefPath  string `json:"brief_path"`
	OutputPath string `json:"output_path"`
	State      string `json:"state"`
}

func validateManifest(dir string) []Issue {
	// #nosec G304 -- quality validation reads the generated manifest inside the caller-selected output directory.
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return []Issue{{Path: "manifest.json", Message: err.Error()}}
	}

	var manifest manifestSummary
	if err := json.Unmarshal(data, &manifest); err != nil {
		return []Issue{{Path: "manifest.json", Message: "invalid manifest json: " + err.Error()}}
	}

	var issues []Issue
	if manifest.SchemaVersion != "manifest/v1" {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("schema_version = %q, want manifest/v1", manifest.SchemaVersion)})
	}
	if len(manifest.Sources) == 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "sources must not be empty"})
	}
	for index, source := range manifest.Sources {
		issues = append(issues, validateSource(index, source)...)
	}
	issues = append(issues, validateSourceDiagnostics(manifest.SourceDiagnostics, manifest.Sources)...)
	if !allowedValue(manifest.Backend.Name, []string{"local", "openai"}) {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("backend.name = %q, want local or openai", manifest.Backend.Name)})
	}
	if !allowedValue(manifest.Renderer, []string{"html", "hybrid", "image"}) {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("renderer = %q, want html, hybrid, or image", manifest.Renderer)})
	}
	if !validStyle(manifest.Style) {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("style = %q is not a valid v0.1 style", manifest.Style)})
	}
	if len(manifest.NextSteps) == 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "next_steps must not be empty"})
	}
	issues = append(issues, validateAudit(manifest.Audit, len(manifest.Sources), manifest.Backend.Name, len(manifest.Warnings))...)

	outputs := map[string]outputFileSummary{}
	outputsByKind := map[string][]outputFileSummary{}
	for _, file := range manifest.OutputFiles {
		kind := strings.TrimSpace(file.Kind)
		if kind != "" {
			if _, ok := outputs[kind]; !ok {
				outputs[kind] = file
			}
			outputsByKind[kind] = append(outputsByKind[kind], file)
		}
	}

	for kind, expectedPath := range map[string]string{
		"scaffold":      "scaffold.html",
		"visual_packet": "visual-packet.json",
		"image":         "final.png",
		"manifest":      "manifest.json",
	} {
		file, ok := outputs[kind]
		if !ok {
			issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output_files missing %q", kind)})
			continue
		}
		issues = append(issues, validateOutputFile(dir, kind, expectedPath, file)...)
	}
	issues = append(issues, validateHandoffOutputFiles(dir, outputs)...)
	issues = append(issues, validatePackOutputFiles(dir, outputsByKind)...)

	return issues
}

func validateHandoffOutputFiles(dir string, outputs map[string]outputFileSummary) []Issue {
	expected := map[string]string{
		"handoff_prompt":    "handoff/codex-prompt.md",
		"handoff_brief":     "handoff/image-brief.md",
		"handoff_checklist": "handoff/qa-checklist.md",
		"handoff_style":     "handoff/style.md",
	}
	declared := false
	for kind := range expected {
		if _, ok := outputs[kind]; ok {
			declared = true
			break
		}
	}
	if !declared {
		return nil
	}
	var issues []Issue
	for kind, expectedPath := range expected {
		file, ok := outputs[kind]
		if !ok {
			issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output_files missing %q", kind)})
			continue
		}
		issues = append(issues, validateOutputFile(dir, kind, expectedPath, file)...)
	}
	return issues
}

func validatePackOutputFiles(dir string, outputs map[string][]outputFileSummary) []Issue {
	var issues []Issue
	contentPacks := outputs["content_pack"]
	var contentPack contentPackSummary
	contentPackOK := false
	if len(contentPacks) > 0 {
		if len(contentPacks) > 1 {
			issues = append(issues, Issue{Path: "manifest.json", Message: "output_files must declare one content_pack"})
		}
		contentPackIssues := validateOutputFile(dir, "content_pack", "content-pack.json", contentPacks[0])
		issues = append(issues, contentPackIssues...)
		if len(contentPackIssues) == 0 {
			var ok bool
			contentPack, ok = readContentPack(filepath.Join(dir, "content-pack.json"), &issues)
			if ok {
				contentPackOK = true
				issues = append(issues, validateContentPackTargets(contentPack)...)
			}
		}
	}

	for _, file := range outputs["pack_brief"] {
		issues = append(issues, validateDeclaredOutputFile(dir, "pack_brief", file)...)
	}

	targetsByOutput := map[string]packTargetSummary{}
	if contentPackOK {
		for _, target := range contentPack.Targets {
			cleanPath, ok := cleanSafeRelPath(target.OutputPath)
			if ok {
				targetsByOutput[cleanPath] = target
			}
		}
	}
	for _, file := range outputs["pack_image"] {
		cleanPath, pathIssues := validateOutputFilePath("pack_image", file.Path)
		issues = append(issues, pathIssues...)
		if len(pathIssues) == 0 {
			if target, ok := targetsByOutput[cleanPath]; ok && (target.State == "planned" || target.State == "handoff_ready") {
				issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("pack_image %q cannot be declared for target %q in state %q", file.Path, target.ID, target.State)})
			}
			issues = append(issues, validateOutputFileHash(dir, "pack_image", cleanPath, file)...)
		}
	}
	return issues
}

func readContentPack(path string, issues *[]Issue) (contentPackSummary, bool) {
	// #nosec G304 -- quality validation reads the generated content pack inside the output bundle.
	data, err := os.ReadFile(path)
	if err != nil {
		*issues = append(*issues, Issue{Path: "content-pack.json", Message: err.Error()})
		return contentPackSummary{}, false
	}

	var contentPack contentPackSummary
	if err := json.Unmarshal(data, &contentPack); err != nil {
		*issues = append(*issues, Issue{Path: "content-pack.json", Message: "invalid content pack json: " + err.Error()})
		return contentPackSummary{}, false
	}
	if contentPack.SchemaVersion != "content-pack/v1" {
		*issues = append(*issues, Issue{Path: "content-pack.json", Message: fmt.Sprintf("schema_version = %q, want content-pack/v1", contentPack.SchemaVersion)})
		return contentPack, false
	}
	return contentPack, true
}

func validateContentPackTargets(contentPack contentPackSummary) []Issue {
	var issues []Issue
	for index, target := range contentPack.Targets {
		prefix := fmt.Sprintf("target %d", index)
		if strings.TrimSpace(target.ID) == "" {
			issues = append(issues, Issue{Path: "content-pack.json", Message: prefix + " id must not be empty"})
		}
		if _, ok := cleanSafeRelPath(target.BriefPath); !ok {
			issues = append(issues, Issue{Path: "content-pack.json", Message: fmt.Sprintf("%s brief_path has unsafe path %q", prefix, target.BriefPath)})
		}
		if _, ok := cleanSafeRelPath(target.OutputPath); !ok {
			issues = append(issues, Issue{Path: "content-pack.json", Message: fmt.Sprintf("%s output_path has unsafe path %q", prefix, target.OutputPath)})
		}
		if !allowedValue(target.State, []string{"planned", "handoff_ready", "generated", "verified", "failed"}) {
			issues = append(issues, Issue{Path: "content-pack.json", Message: fmt.Sprintf("%s state = %q is not supported", prefix, target.State)})
		}
	}
	return issues
}

func validateDeclaredOutputFile(dir string, kind string, file outputFileSummary) []Issue {
	cleanPath, issues := validateOutputFilePath(kind, file.Path)
	if len(issues) > 0 {
		return issues
	}
	return validateOutputFileHash(dir, kind, cleanPath, file)
}

func validateAudit(audit auditSummary, sourceCount int, selectedBackend string, warningCount int) []Issue {
	var issues []Issue
	requireString(&issues, audit.ToolVersion, "audit.tool_version")
	requireString(&issues, audit.RequestedBackend, "audit.requested_backend")
	auditSelectedBackend := requireString(&issues, audit.SelectedBackend, "audit.selected_backend")
	if auditSelectedBackend != "" && auditSelectedBackend != strings.TrimSpace(selectedBackend) {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.selected_backend must match backend.name"})
	}
	auditSourceCount, hasSourceCount := requireInt(&issues, audit.SourceCount, "audit.source_count")
	if hasSourceCount && auditSourceCount != sourceCount {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.source_count must match sources length"})
	}
	evidenceItemCount, hasEvidenceItemCount := requireInt(&issues, audit.EvidenceItemCount, "audit.evidence_item_count")
	if hasEvidenceItemCount && evidenceItemCount < 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.evidence_item_count must not be negative"})
	}
	auditWarningCount, hasWarningCount := requireInt(&issues, audit.WarningCount, "audit.warning_count")
	if hasWarningCount && auditWarningCount < 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.warning_count must not be negative"})
	}
	if hasWarningCount && auditWarningCount >= 0 && auditWarningCount != warningCount {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.warning_count must match warnings length"})
	}
	redactionCount, hasRedactionCount := requireInt(&issues, audit.RedactionCount, "audit.redaction_count")
	if hasRedactionCount && redactionCount < 0 {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.redaction_count must not be negative"})
	}
	requireBool(&issues, audit.RemoteImageAttempted, "audit.remote_image_attempted")
	requireBool(&issues, audit.FallbackUsed, "audit.fallback_used")
	promptTruncated, hasPromptTruncated := requireBool(&issues, audit.PromptTruncated, "audit.prompt_truncated")
	if hasPromptTruncated && promptTruncated && (audit.PromptTruncationMessage == nil || strings.TrimSpace(*audit.PromptTruncationMessage) == "") {
		issues = append(issues, Issue{Path: "manifest.json", Message: "audit.prompt_truncation_message must be set when prompt_truncated is true"})
	}
	return issues
}

func requireString(issues *[]Issue, value *string, name string) string {
	if value == nil {
		*issues = append(*issues, Issue{Path: "manifest.json", Message: name + " must be set"})
		return ""
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		*issues = append(*issues, Issue{Path: "manifest.json", Message: name + " must not be empty"})
	}
	return trimmed
}

func requireInt(issues *[]Issue, value *int, name string) (int, bool) {
	if value == nil {
		*issues = append(*issues, Issue{Path: "manifest.json", Message: name + " must be set"})
		return 0, false
	}
	return *value, true
}

func requireBool(issues *[]Issue, value *bool, name string) (bool, bool) {
	if value == nil {
		*issues = append(*issues, Issue{Path: "manifest.json", Message: name + " must be set"})
		return false, false
	}
	return *value, true
}

func validateSource(index int, source sourceSummary) []Issue {
	var issues []Issue
	prefix := fmt.Sprintf("source %d", index)
	if strings.TrimSpace(source.ID) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: prefix + " id must not be empty"})
	}
	if !allowedValue(source.Kind, []string{"github_repo", "local_repo", "docs_site", "markdown", "json_visual_packet", "pdf"}) {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("%s kind = %q is not supported", prefix, source.Kind)})
	}
	if strings.TrimSpace(source.Input) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: prefix + " input must not be empty"})
	}
	return issues
}

func validateSourceDiagnostics(diagnostics []sourceDiagnosticSummary, sources []sourceSummary) []Issue {
	sourceIDs := map[string]bool{}
	for _, source := range sources {
		sourceIDs[source.ID] = true
	}
	var issues []Issue
	for index, diagnostic := range diagnostics {
		prefix := fmt.Sprintf("source_diagnostics %d", index)
		if strings.TrimSpace(diagnostic.SourceID) == "" {
			issues = append(issues, Issue{Path: "manifest.json", Message: prefix + " source_id must not be empty"})
		} else if !sourceIDs[diagnostic.SourceID] {
			issues = append(issues, Issue{Path: "manifest.json", Message: prefix + " source_id must reference a manifest source"})
		}
		if !allowedValue(diagnostic.Kind, []string{"pdf"}) {
			issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("%s kind = %q is not supported", prefix, diagnostic.Kind)})
		}
		if diagnostic.PagesAttempted < 0 {
			issues = append(issues, Issue{Path: "manifest.json", Message: prefix + " pages_attempted must not be negative"})
		}
		if diagnostic.PagesExtracted < 0 {
			issues = append(issues, Issue{Path: "manifest.json", Message: prefix + " pages_extracted must not be negative"})
		}
	}
	return issues
}

func validStyle(style string) bool {
	trimmed := strings.TrimSpace(style)
	if trimmed == "" {
		return false
	}
	return allowedValue(trimmed, []string{"executive-dark", "analytic", "gist-aparente"})
}

func allowedValue(value string, allowed []string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateOutputFile(dir string, kind string, expectedPath string, file outputFileSummary) []Issue {
	cleanPath, issues := validateOutputFilePath(kind, file.Path)
	if len(issues) > 0 {
		return issues
	}
	if cleanPath != expectedPath {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output file %q path = %q, want %q", kind, file.Path, expectedPath)})
		return issues
	}
	if kind == "manifest" {
		return issues
	}

	return append(issues, validateOutputFileHash(dir, kind, cleanPath, file)...)
}

func validateOutputFilePath(kind string, filePath string) (string, []Issue) {
	if strings.Contains(filePath, "\\") {
		return "", []Issue{{Path: "manifest.json", Message: fmt.Sprintf("output file %q has unsafe path %q", kind, filePath)}}
	}
	cleanPath, ok := cleanSafeRelPath(filePath)
	if !ok {
		return "", []Issue{{Path: "manifest.json", Message: fmt.Sprintf("output file %q has unsafe path %q", kind, filePath)}}
	}
	return cleanPath, nil
}

func cleanSafeRelPath(filePath string) (string, bool) {
	if strings.Contains(filePath, "\\") {
		return "", false
	}
	for _, part := range strings.Split(filePath, "/") {
		if part == ".." {
			return "", false
		}
	}
	cleanPath := path.Clean(filePath)
	if cleanPath == "." || path.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "../") || cleanPath == ".." {
		return "", false
	}
	return cleanPath, true
}

func validateOutputFileHash(dir string, kind string, cleanPath string, file outputFileSummary) []Issue {
	if strings.TrimSpace(file.SHA256) == "" {
		return []Issue{{Path: "manifest.json", Message: fmt.Sprintf("output file %q sha256 must not be empty", kind)}}
	}
	actual, err := fileSHA256(filepath.Join(dir, filepath.FromSlash(cleanPath)))
	if err != nil {
		return []Issue{{Path: "manifest.json", Message: fmt.Sprintf("output file %q sha256 could not be checked: %v", kind, err)}}
	}
	if !strings.EqualFold(file.SHA256, actual) {
		return []Issue{{Path: "manifest.json", Message: fmt.Sprintf("output file %q sha256 mismatch", kind)}}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	// #nosec G304 -- quality validation hashes generated bundle files listed in the manifest.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func readPacket(path string, issues *[]Issue) (packetSummary, bool) {
	// #nosec G304 -- quality validation reads the generated visual packet inside the output bundle.
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			*issues = append(*issues, Issue{Path: "visual-packet.json", Message: err.Error()})
		}
		return packetSummary{}, false
	}

	var packet packetSummary
	if err := json.Unmarshal(data, &packet); err != nil {
		*issues = append(*issues, Issue{Path: "visual-packet.json", Message: "invalid visual packet json: " + err.Error()})
		return packetSummary{}, false
	}
	return packet, true
}

func validateScaffold(path string, packet packetSummary) []Issue {
	// #nosec G304 -- quality validation reads the generated scaffold inside the output bundle.
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{Path: "scaffold.html", Message: err.Error()}}
	}
	text := html.UnescapeString(string(data))

	var issues []Issue
	for _, required := range scaffoldRequiredText(packet) {
		if !strings.Contains(text, required) {
			issues = append(issues, Issue{Path: "scaffold.html", Message: fmt.Sprintf("missing required text %q", required)})
		}
	}
	return issues
}

func scaffoldRequiredText(packet packetSummary) []string {
	seen := map[string]bool{}
	var required []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		required = append(required, value)
	}

	add(packet.Title)
	for _, value := range packet.RequiredText {
		add(value)
	}
	return required
}

func hasIssue(issues []Issue, path string) bool {
	for _, issue := range issues {
		if issue.Path == path {
			return true
		}
	}
	return false
}
