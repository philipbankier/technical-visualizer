package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image/png"
	"os"
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
	SchemaVersion string              `json:"schema_version"`
	Sources       []sourceSummary     `json:"sources"`
	Backend       backendSummary      `json:"backend"`
	Renderer      string              `json:"renderer"`
	Style         string              `json:"style"`
	Warnings      []string            `json:"warnings"`
	NextSteps     []string            `json:"next_steps"`
	Audit         auditSummary        `json:"audit"`
	OutputFiles   []outputFileSummary `json:"output_files"`
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

type backendSummary struct {
	Name string `json:"name"`
}

type outputFileSummary struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
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
	for _, file := range manifest.OutputFiles {
		if strings.TrimSpace(file.Kind) != "" {
			outputs[file.Kind] = file
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

	return issues
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
	var issues []Issue
	cleanPath := filepath.Clean(file.Path)
	if cleanPath == "." || filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) || cleanPath == ".." {
		return []Issue{{Path: "manifest.json", Message: fmt.Sprintf("output file %q has unsafe path %q", kind, file.Path)}}
	}
	if cleanPath != expectedPath {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output file %q path = %q, want %q", kind, file.Path, expectedPath)})
	}
	if kind == "manifest" {
		return issues
	}

	if strings.TrimSpace(file.SHA256) == "" {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output file %q sha256 must not be empty", kind)})
		return issues
	}
	actual, err := fileSHA256(filepath.Join(dir, cleanPath))
	if err != nil {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output file %q sha256 could not be checked: %v", kind, err)})
		return issues
	}
	if !strings.EqualFold(file.SHA256, actual) {
		issues = append(issues, Issue{Path: "manifest.json", Message: fmt.Sprintf("output file %q sha256 mismatch", kind)})
	}
	return issues
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
