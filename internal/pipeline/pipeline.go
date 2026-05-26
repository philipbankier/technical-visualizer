package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/backend"
	"github.com/philipbankier/technical-visualizer/internal/model"
	"github.com/philipbankier/technical-visualizer/internal/packet"
	"github.com/philipbankier/technical-visualizer/internal/quality"
	"github.com/philipbankier/technical-visualizer/internal/render"
	"github.com/philipbankier/technical-visualizer/internal/source"
)

const toolVersion = "0.1.0"

var generatedBundleFiles = []string{"scaffold.html", "visual-packet.json", "final.png", "manifest.json"}

type openAIImageBackend interface {
	Name() string
	Available(context.Context) backend.Capability
	Generate(context.Context, backend.ImageRequest) error
}

var newOpenAIBackend = func() openAIImageBackend {
	return backend.NewOpenAIBackend(backend.OpenAIConfig{})
}

type Options struct {
	Sources   []string
	OutputDir string
	Backend   string
	Renderer  string
	Style     string
	Goal      string
	Offline   bool
}

type imageGenerationResult struct {
	BackendInfo             model.BackendInfo
	Warnings                []string
	RemoteImageAttempted    bool
	FallbackUsed            bool
	PromptTruncated         bool
	PromptTruncationMessage string
}

func Run(ctx context.Context, opts Options) (model.Manifest, error) {
	opts = normalizeOptions(opts)
	if len(opts.Sources) == 0 {
		return model.Manifest{}, errors.New("pipeline requires at least one source")
	}
	if err := validateOptions(opts); err != nil {
		return model.Manifest{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o700); err != nil {
		return model.Manifest{}, err
	}

	specs, err := source.ResolveAll(opts.Sources)
	if err != nil {
		return model.Manifest{}, err
	}
	evidence, err := source.GatherAll(ctx, specs, source.GatherOptions{Offline: opts.Offline})
	if err != nil {
		return model.Manifest{}, err
	}
	publicEvidence := publicEvidenceBundle(evidence)
	if len(publicEvidence.Items) == 0 {
		if err := removeGeneratedBundleFiles(opts.OutputDir); err != nil {
			return model.Manifest{}, fmt.Errorf("failed to clean generated bundle files after no evidence gathered: %w", err)
		}
		return model.Manifest{}, fmt.Errorf("no evidence gathered from %d source(s); warnings: %s", len(publicEvidence.Sources), formatWarnings(publicEvidence.Warnings))
	}
	visualPacket, err := packet.Build(publicEvidence, packet.BuildOptions{
		Goal:     opts.Goal,
		Style:    opts.Style,
		Renderer: opts.Renderer,
	})
	if err != nil {
		return model.Manifest{}, err
	}

	scaffoldPath := filepath.Join(opts.OutputDir, "scaffold.html")
	packetPath := filepath.Join(opts.OutputDir, "visual-packet.json")
	imagePath := filepath.Join(opts.OutputDir, "final.png")
	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")

	if err := render.WriteScaffoldFile(scaffoldPath, visualPacket); err != nil {
		return model.Manifest{}, err
	}
	packetJSON, err := writeJSONFile(packetPath, visualPacket)
	if err != nil {
		return model.Manifest{}, err
	}
	// #nosec G304 -- this reads the scaffold file the pipeline just wrote in the output bundle.
	scaffoldHTML, err := os.ReadFile(scaffoldPath)
	if err != nil {
		return model.Manifest{}, err
	}

	imageResult, err := generateImage(ctx, opts, visualPacket, string(packetJSON), string(scaffoldHTML), imagePath)
	if err != nil {
		return model.Manifest{}, err
	}

	warnings := append(append([]string(nil), publicEvidence.Warnings...), imageResult.Warnings...)
	manifest := model.Manifest{
		SchemaVersion: "manifest/v1",
		Sources:       publicEvidence.Sources,
		Backend:       imageResult.BackendInfo,
		Renderer:      opts.Renderer,
		Style:         opts.Style,
		Warnings:      warnings,
		Audit:         baselineAudit(opts, imageResult, publicEvidence, warnings),
		OutputFiles: []model.OutputFile{
			outputFile("scaffold", "scaffold.html", scaffoldPath),
			outputFile("visual_packet", "visual-packet.json", packetPath),
			outputFile("image", "final.png", imagePath),
			{Kind: "manifest", Path: "manifest.json"},
		},
	}
	if _, err := writeManifestFile(manifestPath, manifest); err != nil {
		return model.Manifest{}, err
	}

	if issues := quality.ValidateBundle(opts.OutputDir); len(issues) > 0 {
		return manifest, fmt.Errorf("quality validation failed: %s", formatIssues(issues))
	}
	return manifest, nil
}

func baselineAudit(opts Options, imageResult imageGenerationResult, publicEvidence model.EvidenceBundle, warnings []string) model.ManifestAudit {
	return model.ManifestAudit{
		ToolVersion:             toolVersion,
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
	}
}

func normalizeOptions(opts Options) Options {
	if strings.TrimSpace(opts.OutputDir) == "" {
		opts.OutputDir = "visualize-output"
	}
	if strings.TrimSpace(opts.Backend) == "" {
		opts.Backend = "local"
	}
	if strings.TrimSpace(opts.Renderer) == "" {
		opts.Renderer = "hybrid"
	}
	if strings.TrimSpace(opts.Style) == "" {
		opts.Style = "executive-dark"
	}
	if strings.TrimSpace(opts.Goal) == "" {
		opts.Goal = "architecture-map"
	}
	opts.Backend = strings.ToLower(strings.TrimSpace(opts.Backend))
	opts.Renderer = strings.ToLower(strings.TrimSpace(opts.Renderer))
	return opts
}

func validateOptions(opts Options) error {
	switch opts.Backend {
	case "auto", "hybrid", "local", "openai":
	default:
		if opts.Backend == "codex" {
			return errors.New("codex backend is doctor-only in v1; use visualize doctor to check Codex availability and --backend local or --backend openai to generate")
		}
		return fmt.Errorf("unsupported backend %q", opts.Backend)
	}

	switch opts.Renderer {
	case "html", "hybrid", "image":
	default:
		return fmt.Errorf("unsupported renderer %q", opts.Renderer)
	}

	if opts.Offline && opts.Backend == "openai" {
		return errors.New("--offline cannot be used with remote backend openai")
	}

	return nil
}

func generateImage(ctx context.Context, opts Options, visualPacket model.VisualPacket, packetJSON string, scaffoldHTML string, outputPath string) (imageGenerationResult, error) {
	if opts.Backend == "local" || opts.Renderer == "html" {
		if err := render.WriteFallbackPNG(outputPath, visualPacket); err != nil {
			return imageGenerationResult{}, err
		}
		return imageGenerationResult{BackendInfo: model.BackendInfo{Name: "local", Remote: false}}, nil
	}
	if opts.Offline {
		if err := render.WriteFallbackPNG(outputPath, visualPacket); err != nil {
			return imageGenerationResult{}, err
		}
		return imageGenerationResult{
			BackendInfo:  model.BackendInfo{Name: "local", Remote: false},
			Warnings:     []string{"offline mode used local fallback; no remote image backend was called"},
			FallbackUsed: true,
		}, nil
	}

	prompt := imagePrompt(packetJSON)
	switch opts.Backend {
	case "openai":
		client := newOpenAIBackend()
		request := backend.ImageRequest{Prompt: prompt, ScaffoldHTML: scaffoldHTML, OutputPath: outputPath}
		promptResult, err := backend.BuildPrompt(request)
		if err != nil {
			return imageGenerationResult{}, err
		}
		if err := client.Generate(ctx, request); err != nil {
			return imageGenerationResult{}, err
		}
		return imageGenerationResult{
			BackendInfo:             model.BackendInfo{Name: client.Name(), Remote: true, Model: "gpt-image-2"},
			RemoteImageAttempted:    true,
			PromptTruncated:         promptResult.Truncated,
			PromptTruncationMessage: promptResult.TruncationMessage,
		}, nil
	case "auto", "hybrid":
		client := newOpenAIBackend()
		capability := client.Available(ctx)
		if capability.Available {
			request := backend.ImageRequest{Prompt: prompt, ScaffoldHTML: scaffoldHTML, OutputPath: outputPath}
			promptResult, err := backend.BuildPrompt(request)
			if err != nil {
				return imageGenerationResult{}, err
			}
			if err := client.Generate(ctx, request); err != nil {
				if fallbackErr := render.WriteFallbackPNG(outputPath, visualPacket); fallbackErr != nil {
					return imageGenerationResult{}, fallbackErr
				}
				return imageGenerationResult{
					BackendInfo:             model.BackendInfo{Name: "local", Remote: false},
					Warnings:                []string{"OpenAI generation failed, used local fallback: " + err.Error()},
					RemoteImageAttempted:    true,
					FallbackUsed:            true,
					PromptTruncated:         promptResult.Truncated,
					PromptTruncationMessage: promptResult.TruncationMessage,
				}, nil
			}
			return imageGenerationResult{
				BackendInfo:             model.BackendInfo{Name: client.Name(), Remote: true, Model: "gpt-image-2"},
				RemoteImageAttempted:    true,
				PromptTruncated:         promptResult.Truncated,
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
	default:
		return imageGenerationResult{}, fmt.Errorf("unsupported backend %q", opts.Backend)
	}
}

func imagePrompt(packetJSON string) string {
	return "Create a source-backed technical visualization from this visual packet. Preserve required text exactly, do not invent facts, and represent uncertainty visibly.\n\nVisual packet JSON:\n" + packetJSON
}

func publicEvidenceBundle(bundle model.EvidenceBundle) model.EvidenceBundle {
	public := model.EvidenceBundle{
		SchemaVersion: bundle.SchemaVersion,
		Sources:       make([]model.SourceSpec, len(bundle.Sources)),
		Items:         make([]model.EvidenceItem, len(bundle.Items)),
		Warnings:      append([]string(nil), bundle.Warnings...),
		Redactions:    make([]model.Redaction, len(bundle.Redactions)),
	}

	originalSources := map[string]model.SourceSpec{}
	replacements := map[string]string{}
	for i, spec := range bundle.Sources {
		originalSources[spec.ID] = spec
		publicSpec := publicSourceSpec(spec)
		public.Sources[i] = publicSpec
		addPathReplacement(replacements, spec.Input, publicSpec.Input)
		addPathReplacement(replacements, spec.Resolved, publicSpec.Input)
	}

	for i, item := range bundle.Items {
		publicItem := item
		if sourceSpec, ok := originalSources[item.SourceID]; ok && item.Path != "" {
			publicPath := publicItemPath(sourceSpec, item.Path)
			addPathReplacement(replacements, item.Path, publicPath)
			publicItem.Path = publicPath
		}
		public.Items[i] = publicItem
	}

	for i, redaction := range bundle.Redactions {
		publicRedaction := redaction
		if sourceSpec, ok := originalSources[redaction.SourceID]; ok && redaction.Path != "" {
			publicRedaction.Path = publicItemPath(sourceSpec, redaction.Path)
		}
		public.Redactions[i] = publicRedaction
	}

	for i := range public.Items {
		public.Items[i].Text = applyPathReplacements(public.Items[i].Text, replacements)
	}
	for i := range public.Warnings {
		public.Warnings[i] = applyPathReplacements(public.Warnings[i], replacements)
	}
	return public
}

func publicSourceSpec(spec model.SourceSpec) model.SourceSpec {
	public := spec
	if isHTTPURL(firstNonEmpty(spec.Resolved, spec.Input)) {
		return public
	}

	public.Input = publicSourceLocator(spec)
	public.Resolved = ""
	return public
}

func publicSourceLocator(spec model.SourceSpec) string {
	for _, value := range []string{spec.Input, spec.Resolved, spec.ID} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if isHTTPURL(value) {
			return value
		}
		if !filepath.IsAbs(filepath.Clean(value)) {
			return filepath.ToSlash(filepath.Clean(value))
		}
		return filepath.Base(filepath.Clean(value))
	}
	return ""
}

func publicItemPath(spec model.SourceSpec, itemPath string) string {
	itemPath = strings.TrimSpace(itemPath)
	if itemPath == "" || isHTTPURL(itemPath) {
		return itemPath
	}
	if !filepath.IsAbs(filepath.Clean(itemPath)) {
		return filepath.ToSlash(filepath.Clean(itemPath))
	}
	for _, root := range []string{spec.Resolved, spec.Input} {
		root = strings.TrimSpace(root)
		if root == "" || isHTTPURL(root) || !filepath.IsAbs(filepath.Clean(root)) {
			continue
		}
		candidateRoot := filepath.Clean(root)
		if filepath.Ext(candidateRoot) != "" {
			candidateRoot = filepath.Dir(candidateRoot)
		}
		if rel, err := filepath.Rel(candidateRoot, filepath.Clean(itemPath)); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(filepath.Clean(itemPath))
}

func addPathReplacement(replacements map[string]string, raw string, public string) {
	raw = strings.TrimSpace(raw)
	public = strings.TrimSpace(public)
	if raw == "" || public == "" || raw == public {
		return
	}
	if isHTTPURL(raw) {
		return
	}
	if filepath.IsAbs(filepath.Clean(raw)) || strings.Contains(raw, string(filepath.Separator)) {
		replacements[raw] = public
	}
}

func applyPathReplacements(text string, replacements map[string]string) string {
	for raw, public := range replacements {
		text = strings.ReplaceAll(text, raw, public)
		text = strings.ReplaceAll(text, filepath.ToSlash(raw), public)
	}
	return text
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeJSONFile(path string, value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return data, os.WriteFile(path, data, 0o600)
}

func removeGeneratedBundleFiles(outputDir string) error {
	for _, name := range generatedBundleFiles {
		if err := os.Remove(filepath.Join(outputDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}

func writeManifestFile(path string, manifest model.Manifest) ([]byte, error) {
	type manifestJSON struct {
		SchemaVersion string              `json:"schema_version"`
		Sources       []model.SourceSpec  `json:"sources"`
		Backend       model.BackendInfo   `json:"backend"`
		Renderer      string              `json:"renderer"`
		Style         string              `json:"style"`
		Warnings      []string            `json:"warnings"`
		Audit         model.ManifestAudit `json:"audit"`
		OutputFiles   []model.OutputFile  `json:"output_files"`
	}
	warnings := append([]string(nil), manifest.Warnings...)
	if warnings == nil {
		warnings = []string{}
	}
	return writeJSONFile(path, manifestJSON{
		SchemaVersion: manifest.SchemaVersion,
		Sources:       manifest.Sources,
		Backend:       manifest.Backend,
		Renderer:      manifest.Renderer,
		Style:         manifest.Style,
		Warnings:      warnings,
		Audit:         manifest.Audit,
		OutputFiles:   manifest.OutputFiles,
	})
}

func outputFile(kind string, name string, path string) model.OutputFile {
	return model.OutputFile{Kind: kind, Path: name, SHA256: sha256File(path)}
}

func sha256File(path string) string {
	// #nosec G304 -- path is one of the generated bundle files whose hash is written to the manifest.
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func formatIssues(issues []quality.Issue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Path+": "+issue.Message)
	}
	return strings.Join(parts, "; ")
}

func formatWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return "none"
	}
	return strings.Join(warnings, "; ")
}
