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
	"strconv"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/backend"
	"github.com/philipbankier/technical-visualizer/internal/handoff"
	"github.com/philipbankier/technical-visualizer/internal/model"
	"github.com/philipbankier/technical-visualizer/internal/pack"
	"github.com/philipbankier/technical-visualizer/internal/packet"
	"github.com/philipbankier/technical-visualizer/internal/quality"
	"github.com/philipbankier/technical-visualizer/internal/render"
	"github.com/philipbankier/technical-visualizer/internal/source"
)

const toolVersion = "0.1.0"

var generatedBundleFiles = []string{"scaffold.html", "visual-packet.json", "final.png", "manifest.json"}
var generatedHandoffFiles = []string{
	"handoff/codex-prompt.md",
	"handoff/image-brief.md",
	"handoff/qa-checklist.md",
	"handoff/style.md",
}
var generatedPackFiles = []string{
	"content-pack.json",
	"pack/linkedin-dense/brief.md",
	"pack/social-teaser/brief.md",
	"pack/blog-og/brief.md",
}

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
	Handoff   string
	Quick     bool
	Pack      string
}

type imageGenerationResult struct {
	BackendInfo             model.BackendInfo
	Warnings                []string
	RemoteImageAttempted    bool
	FallbackUsed            bool
	PromptTruncated         bool
	PromptTruncationMessage string
	HandoffWritten          bool
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

	backup, err := backupGeneratedBundleFiles(opts.OutputDir)
	if err != nil {
		return model.Manifest{}, err
	}
	committed := false
	defer func() {
		if !committed {
			backup.restore()
		}
	}()

	if err := render.WriteScaffoldFile(scaffoldPath, visualPacket); err != nil {
		return model.Manifest{}, err
	}
	packetJSON, err := writeJSONFile(packetPath, visualPacket)
	if err != nil {
		return model.Manifest{}, err
	}
	packResult := packWriteResult{}
	if packEnabled(opts) {
		result, err := writeContentPackBundle(opts.OutputDir, visualPacket)
		if err != nil {
			return model.Manifest{}, err
		}
		packResult = result
	} else {
		if err := removeGeneratedPackFiles(opts.OutputDir); err != nil {
			return model.Manifest{}, err
		}
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
	handoffResult := handoff.Result{}
	if opts.Handoff == "codex" {
		result, err := handoff.WriteCodexPackage(opts.OutputDir, visualPacket, handoff.Options{OutputImagePath: "final.png"})
		if err != nil {
			return model.Manifest{}, err
		}
		handoffResult = result
		imageResult.HandoffWritten = true
	}
	if !imageResult.HandoffWritten {
		if err := removeGeneratedHandoffFiles(opts.OutputDir); err != nil {
			return model.Manifest{}, err
		}
	}

	warnings := append(append([]string(nil), publicEvidence.Warnings...), imageResult.Warnings...)
	manifest := model.Manifest{
		SchemaVersion:     "manifest/v1",
		Sources:           publicEvidence.Sources,
		Backend:           imageResult.BackendInfo,
		Renderer:          opts.Renderer,
		Style:             opts.Style,
		Warnings:          warnings,
		NextSteps:         nextSteps(opts, imageResult),
		Audit:             baselineAudit(opts, imageResult, publicEvidence, warnings),
		SourceDiagnostics: sourceDiagnostics(publicEvidence),
		OutputFiles: []model.OutputFile{
			outputFile("scaffold", "scaffold.html", scaffoldPath),
			outputFile("visual_packet", "visual-packet.json", packetPath),
			outputFile("image", "final.png", imagePath),
			{Kind: "manifest", Path: "manifest.json"},
		},
	}
	if imageResult.HandoffWritten {
		manifest.OutputFiles = append(manifest.OutputFiles, handoffOutputFiles(opts.OutputDir, handoffResult)...)
	}
	if packResult.Written {
		manifest.OutputFiles = append(manifest.OutputFiles, packOutputFiles(opts.OutputDir, packResult)...)
	}
	if _, err := writeManifestFile(manifestPath, manifest); err != nil {
		return model.Manifest{}, err
	}

	if issues := quality.ValidateBundle(opts.OutputDir); len(issues) > 0 {
		return manifest, fmt.Errorf("quality validation failed: %s", formatIssues(issues))
	}
	committed = true
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

func sourceDiagnostics(evidence model.EvidenceBundle) []model.SourceDiagnostic {
	warningsBySource := map[string][]string{}
	for _, warning := range evidence.Warnings {
		sourceID, ok := warningSourceID(warning)
		if !ok {
			continue
		}
		warningsBySource[sourceID] = append(warningsBySource[sourceID], warning)
	}

	diagnosticsBySource := map[string]model.SourceDiagnostic{}
	for _, item := range evidence.Items {
		if item.Kind != "pdf_text" {
			continue
		}
		metadata := item.Metadata
		diagnosticsBySource[item.SourceID] = model.SourceDiagnostic{
			SourceID:       item.SourceID,
			Kind:           "pdf",
			Engine:         metadata["pdf_engine"],
			Version:        metadata["pdf_engine_version"],
			PagesAttempted: metadataInt(metadata, "pdf_page_count"),
			PagesExtracted: metadataInt(metadata, "pdf_pages_extracted"),
			Truncated:      metadataBool(metadata, "pdf_truncated"),
			Warnings:       append([]string(nil), warningsBySource[item.SourceID]...),
		}
	}

	diagnostics := make([]model.SourceDiagnostic, 0, len(diagnosticsBySource)+len(warningsBySource))
	for _, spec := range evidence.Sources {
		if spec.Kind != model.SourcePDF {
			continue
		}
		diagnostic, ok := diagnosticsBySource[spec.ID]
		if !ok {
			if len(warningsBySource[spec.ID]) == 0 {
				continue
			}
			diagnostic = model.SourceDiagnostic{
				SourceID: spec.ID,
				Kind:     "pdf",
				Warnings: append([]string(nil), warningsBySource[spec.ID]...),
			}
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

func warningSourceID(warning string) (string, bool) {
	const prefix = "source "
	if !strings.HasPrefix(warning, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(warning, prefix)
	sourceID, _, ok := strings.Cut(rest, ":")
	sourceID = strings.TrimSpace(sourceID)
	return sourceID, ok && sourceID != ""
}

func metadataInt(metadata map[string]string, key string) int {
	value, err := strconv.Atoi(strings.TrimSpace(metadata[key]))
	if err != nil {
		return 0
	}
	return value
}

func metadataBool(metadata map[string]string, key string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(metadata[key]))
	return err == nil && value
}

func nextSteps(opts Options, imageResult imageGenerationResult) []string {
	var steps []string
	if imageResult.BackendInfo.Name == "local" {
		steps = append(steps, "Open scaffold.html first; visual-packet.json contains the source-backed content packet for this run.")
		steps = append(steps, "For a polished OpenAI image, set OPENAI_API_KEY and rerun with --backend openai --renderer image.")
		if imageResult.HandoffWritten {
			steps = append(steps, "For Codex image generation, open handoff/codex-prompt.md and run it in interactive Codex.")
		} else {
			steps = append(steps, "For a guided Codex package, rerun with --handoff codex.")
		}
	}
	if imageResult.BackendInfo.Name == "openai" {
		steps = append(steps, "Inspect final.png for the generated image and manifest.json for source and backend audit details.")
	}
	if imageResult.FallbackUsed {
		if imageResult.HandoffWritten {
			steps = append(steps, "The run used a local fallback; open handoff/codex-prompt.md for the interactive Codex path.")
		} else {
			steps = append(steps, "The run used a local fallback; use explicit --backend openai or --handoff codex for a polished image path.")
		}
	}
	if opts.Offline {
		steps = append(steps, "offline mode was enabled; no remote source fetching or remote image generation was performed.")
	}
	if packEnabled(opts) {
		steps = append(steps, "Open content-pack.json and pack/ target brief directories before producing target-specific images.")
	}
	return steps
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
	opts.Handoff = strings.ToLower(strings.TrimSpace(opts.Handoff))
	opts.Pack = strings.ToLower(strings.TrimSpace(opts.Pack))
	return opts
}

func validateOptions(opts Options) error {
	switch opts.Backend {
	case "auto", "hybrid", "local", "openai":
	default:
		if opts.Backend == "codex" {
			return errors.New("codex is an agent workflow in v0.1, not a direct image backend; use visualize doctor to check Codex availability, --backend openai for direct API image generation, or --backend local for private local output")
		}
		return fmt.Errorf("unsupported backend %q", opts.Backend)
	}

	switch opts.Renderer {
	case "html", "hybrid", "image":
	default:
		return fmt.Errorf("unsupported renderer %q", opts.Renderer)
	}

	switch opts.Handoff {
	case "", "codex":
	default:
		return fmt.Errorf("unsupported handoff %q", opts.Handoff)
	}

	switch opts.Pack {
	case "", "off", "auto":
	default:
		return fmt.Errorf("unsupported pack %q", opts.Pack)
	}

	if opts.Quick && opts.Handoff != "codex" {
		return errors.New("--quick requires --handoff codex")
	}

	if opts.Offline && opts.Backend == "openai" {
		return errors.New("--offline cannot be used with remote backend openai")
	}

	return nil
}

func packEnabled(opts Options) bool {
	return opts.Pack == "auto"
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

type packWriteResult struct {
	Written         bool
	ContentPackPath string
	BriefPaths      []string
}

func writeContentPackBundle(outputDir string, visualPacket model.VisualPacket) (packWriteResult, error) {
	contentPack, err := pack.Plan(visualPacket, pack.Options{Mode: "auto"})
	if err != nil {
		return packWriteResult{}, err
	}

	contentPackPath := "content-pack.json"
	if _, err := writeJSONFile(filepath.Join(outputDir, contentPackPath), contentPack); err != nil {
		return packWriteResult{}, err
	}

	result := packWriteResult{
		Written:         true,
		ContentPackPath: contentPackPath,
		BriefPaths:      make([]string, 0, len(contentPack.Targets)),
	}
	for _, target := range contentPack.Targets {
		if err := writePackBriefFile(outputDir, visualPacket, contentPack, target); err != nil {
			return packWriteResult{}, err
		}
		result.BriefPaths = append(result.BriefPaths, target.BriefPath)
	}
	return result, nil
}

func writePackBriefFile(outputDir string, visualPacket model.VisualPacket, contentPack pack.ContentPack, target pack.TargetSpec) error {
	if !safeBundleRelPath(target.BriefPath) {
		return fmt.Errorf("unsafe pack brief path %q", target.BriefPath)
	}
	path := filepath.Join(outputDir, filepath.FromSlash(target.BriefPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := ensureGeneratedPackDirsSafe(outputDir); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(packBriefMarkdown(visualPacket, contentPack, target)), 0o600)
}

func packBriefMarkdown(visualPacket model.VisualPacket, contentPack pack.ContentPack, target pack.TargetSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", visualPacket.Title)
	fmt.Fprintf(&b, "- Target intent: %s\n", target.Intent)
	fmt.Fprintf(&b, "- Density: %s\n", target.Density)
	fmt.Fprintf(&b, "- Aspect ratio: %s\n", target.AspectRatio)
	fmt.Fprintf(&b, "- Planned output path: %s\n", target.OutputPath)
	fmt.Fprintf(&b, "- Pack status: %s\n\n", contentPack.Status)

	b.WriteString("## Required content\n\n")
	writeMarkdownList(&b, target.RequiredContent)
	b.WriteString("\n## Avoid\n\n")
	writeMarkdownList(&b, target.Avoid)
	b.WriteString("\n## Source references\n\n")
	writeMarkdownList(&b, visualPacketSourceReferences(visualPacket))
	b.WriteString("\nNote: planned local output is not a final polished image. Treat this brief as source-backed planning input for a later image generation or handoff step.\n")
	return b.String()
}

func writeMarkdownList(b *strings.Builder, values []string) {
	if len(values) == 0 {
		b.WriteString("- None declared.\n")
		return
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		fmt.Fprintf(b, "- %s\n", value)
	}
}

func visualPacketSourceReferences(packet model.VisualPacket) []string {
	var refs []string
	seen := map[string]bool{}
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			refs = append(refs, value)
		}
	}
	for _, ref := range packet.SourceRefs {
		label := strings.TrimSpace(ref.Label)
		locator := strings.TrimSpace(ref.Locator)
		switch {
		case label != "" && locator != "":
			add(label + " (" + locator + ")")
		case label != "":
			add(label)
		case locator != "":
			add(locator)
		case strings.TrimSpace(ref.ID) != "":
			add(ref.ID)
		}
	}
	for _, claim := range packet.RankedClaims {
		add(claim.SourceRefs...)
	}
	for _, fact := range packet.Facts {
		add(fact.SourceRefs...)
	}
	for _, block := range packet.ContentBlocks {
		add(block.SourceRefs...)
	}
	for _, metric := range packet.Metrics {
		add(metric.SourceRefs...)
	}
	for _, event := range packet.Timeline {
		add(event.SourceRefs...)
	}
	for _, entity := range packet.Entities {
		add(entity.SourceRefs...)
	}
	for _, table := range packet.Tables {
		add(table.SourceRefs...)
	}
	for _, diagram := range packet.Diagrams {
		add(diagram.SourceRefs...)
	}
	for _, question := range packet.OpenQuestions {
		add(question.SourceRefs...)
	}
	for _, risk := range packet.Risks {
		add(risk.SourceRefs...)
	}
	for _, tradeoff := range packet.Tradeoffs {
		add(tradeoff.SourceRefs...)
	}
	for _, unknown := range packet.Unknowns {
		add(unknown.SourceRefs...)
	}
	if len(refs) == 0 {
		return []string{"visual-packet.json"}
	}
	return refs
}

func safeBundleRelPath(relPath string) bool {
	if strings.Contains(relPath, "\\") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if part == ".." {
			return false
		}
	}
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	return cleanPath != "." && !filepath.IsAbs(cleanPath) && cleanPath != ".." && !strings.HasPrefix(cleanPath, "../")
}

func removeGeneratedBundleFiles(outputDir string) error {
	for _, name := range generatedBundleFiles {
		if err := os.Remove(filepath.Join(outputDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	if err := removeGeneratedHandoffFiles(outputDir); err != nil {
		return err
	}
	return removeGeneratedPackFiles(outputDir)
}

func removeGeneratedHandoffFiles(outputDir string) error {
	if err := ensureGeneratedHandoffParentSafe(outputDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, name := range generatedHandoffFiles {
		if err := os.Remove(filepath.Join(outputDir, filepath.FromSlash(name))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return removeEmptyGeneratedHandoffDir(outputDir)
}

func removeGeneratedPackFiles(outputDir string) error {
	if err := os.Remove(filepath.Join(outputDir, "content-pack.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove content-pack.json: %w", err)
	}
	if err := ensureGeneratedPackDirsSafe(outputDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, name := range generatedPackFiles {
		if name == "content-pack.json" {
			continue
		}
		if err := os.Remove(filepath.Join(outputDir, filepath.FromSlash(name))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return removeEmptyGeneratedPackDirs(outputDir)
}

func ensureGeneratedHandoffParentSafe(outputDir string) error {
	handoffDir := filepath.Join(outputDir, "handoff")
	info, err := os.Lstat(handoffDir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("handoff directory is a symlink: %s", handoffDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("handoff path is not a directory: %s", handoffDir)
	}
	return nil
}

func ensureGeneratedPackParentSafe(outputDir string) error {
	return ensureGeneratedDirSafe(outputDir, "pack")
}

func ensureGeneratedPackDirsSafe(outputDir string) error {
	for _, relDir := range generatedPackDirs() {
		if err := ensureGeneratedDirSafe(outputDir, relDir); err != nil {
			if errors.Is(err, os.ErrNotExist) && relDir != "pack" {
				continue
			}
			return err
		}
	}
	return nil
}

func ensureGeneratedDirSafe(outputDir string, relDir string) error {
	dir := filepath.Join(outputDir, filepath.FromSlash(relDir))
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("generated directory is a symlink: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("generated path is not a directory: %s", dir)
	}
	return nil
}

func generatedPackDirs() []string {
	return []string{"pack", "pack/linkedin-dense", "pack/social-teaser", "pack/blog-og"}
}

func generatedHandoffParentIsSafe(outputDir string) bool {
	err := ensureGeneratedHandoffParentSafe(outputDir)
	return err == nil || errors.Is(err, os.ErrNotExist)
}

func generatedPackParentIsSafe(outputDir string) bool {
	err := ensureGeneratedPackParentSafe(outputDir)
	return err == nil || errors.Is(err, os.ErrNotExist)
}

func generatedPackDirsAreSafe(outputDir string) bool {
	err := ensureGeneratedPackDirsSafe(outputDir)
	return err == nil || errors.Is(err, os.ErrNotExist)
}

func relPathUsesGeneratedHandoffDir(relPath string) bool {
	return strings.HasPrefix(filepath.ToSlash(relPath), "handoff/")
}

func relPathUsesGeneratedPackDir(relPath string) bool {
	return strings.HasPrefix(filepath.ToSlash(relPath), "pack/")
}

func removeEmptyGeneratedHandoffDir(outputDir string) error {
	handoffDir := filepath.Join(outputDir, "handoff")
	if err := ensureGeneratedHandoffParentSafe(outputDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	err := os.Remove(handoffDir)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if entries, readErr := os.ReadDir(handoffDir); readErr == nil && len(entries) > 0 {
		return nil
	}
	return fmt.Errorf("remove handoff: %w", err)
}

func removeEmptyGeneratedPackDirs(outputDir string) error {
	if err := ensureGeneratedPackDirsSafe(outputDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, relDir := range []string{"pack/linkedin-dense", "pack/social-teaser", "pack/blog-og", "pack"} {
		if err := removeGeneratedDirIfEmpty(outputDir, relDir); err != nil {
			return err
		}
	}
	return nil
}

func removeGeneratedDirIfEmpty(outputDir string, relDir string) error {
	dir := filepath.Join(outputDir, filepath.FromSlash(relDir))
	err := os.Remove(dir)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if entries, readErr := os.ReadDir(dir); readErr == nil && len(entries) > 0 {
		return nil
	}
	return fmt.Errorf("remove %s: %w", relDir, err)
}

func backupGeneratedBundleFiles(outputDir string) (generatedBundleBackup, error) {
	if err := ensureGeneratedHandoffParentSafe(outputDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return generatedBundleBackup{}, err
	}
	if err := ensureGeneratedPackDirsSafe(outputDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return generatedBundleBackup{}, err
	}
	backup := generatedBundleBackup{
		outputDir: outputDir,
		files:     make([]generatedFileBackup, 0, len(generatedBundleRelPaths())),
	}
	for _, relPath := range generatedBundleRelPaths() {
		if relPathUsesGeneratedHandoffDir(relPath) && !generatedHandoffParentIsSafe(outputDir) {
			return generatedBundleBackup{}, fmt.Errorf("handoff directory is unsafe: %s", filepath.Join(outputDir, "handoff"))
		}
		if relPathUsesGeneratedPackDir(relPath) && !generatedPackParentIsSafe(outputDir) {
			return generatedBundleBackup{}, fmt.Errorf("pack directory is unsafe: %s", filepath.Join(outputDir, "pack"))
		}
		path := filepath.Join(outputDir, filepath.FromSlash(relPath))
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			backup.files = append(backup.files, generatedFileBackup{relPath: relPath})
			continue
		}
		if err != nil {
			return generatedBundleBackup{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return generatedBundleBackup{}, fmt.Errorf("generated bundle file is a symlink: %s", relPath)
		}
		if !info.Mode().IsRegular() {
			return generatedBundleBackup{}, fmt.Errorf("generated bundle path is not a regular file: %s", relPath)
		}
		// #nosec G304 -- relPath is selected from the fixed generated bundle file list.
		data, err := os.ReadFile(path)
		if err != nil {
			return generatedBundleBackup{}, err
		}
		backup.files = append(backup.files, generatedFileBackup{
			relPath: relPath,
			exists:  true,
			data:    data,
			mode:    info.Mode().Perm(),
		})
	}
	return backup, nil
}

type generatedBundleBackup struct {
	outputDir string
	files     []generatedFileBackup
}

type generatedFileBackup struct {
	relPath string
	exists  bool
	data    []byte
	mode    os.FileMode
}

func generatedBundleRelPaths() []string {
	paths := make([]string, 0, len(generatedBundleFiles)+len(generatedHandoffFiles)+len(generatedPackFiles))
	paths = append(paths, generatedBundleFiles...)
	paths = append(paths, generatedHandoffFiles...)
	paths = append(paths, generatedPackFiles...)
	return paths
}

func (backup generatedBundleBackup) restore() {
	handoffParentSafe := generatedHandoffParentIsSafe(backup.outputDir)
	packDirsSafe := generatedPackDirsAreSafe(backup.outputDir)
	for _, file := range backup.files {
		if relPathUsesGeneratedHandoffDir(file.relPath) && !handoffParentSafe {
			continue
		}
		if relPathUsesGeneratedPackDir(file.relPath) && !packDirsSafe {
			continue
		}
		path := filepath.Join(backup.outputDir, filepath.FromSlash(file.relPath))
		_ = os.Remove(path)
	}
	for _, file := range backup.files {
		if !file.exists {
			continue
		}
		if relPathUsesGeneratedHandoffDir(file.relPath) && !handoffParentSafe {
			continue
		}
		if relPathUsesGeneratedPackDir(file.relPath) && !packDirsSafe {
			continue
		}
		path := filepath.Join(backup.outputDir, filepath.FromSlash(file.relPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			continue
		}
		_ = os.WriteFile(path, file.data, file.mode)
		_ = os.Chmod(path, file.mode)
	}
	_ = removeEmptyGeneratedHandoffDir(backup.outputDir)
	if packDirsSafe {
		_ = removeEmptyGeneratedPackDirs(backup.outputDir)
	}
}

func handoffOutputFiles(outputDir string, result handoff.Result) []model.OutputFile {
	return []model.OutputFile{
		outputFile("handoff_prompt", result.PromptPath, filepath.Join(outputDir, filepath.FromSlash(result.PromptPath))),
		outputFile("handoff_brief", result.BriefPath, filepath.Join(outputDir, filepath.FromSlash(result.BriefPath))),
		outputFile("handoff_checklist", result.ChecklistPath, filepath.Join(outputDir, filepath.FromSlash(result.ChecklistPath))),
		outputFile("handoff_style", result.StylePath, filepath.Join(outputDir, filepath.FromSlash(result.StylePath))),
	}
}

func packOutputFiles(outputDir string, result packWriteResult) []model.OutputFile {
	files := []model.OutputFile{
		outputFile("content_pack", result.ContentPackPath, filepath.Join(outputDir, result.ContentPackPath)),
	}
	for _, briefPath := range result.BriefPaths {
		files = append(files, outputFile("pack_brief", briefPath, filepath.Join(outputDir, filepath.FromSlash(briefPath))))
	}
	return files
}

func writeManifestFile(path string, manifest model.Manifest) ([]byte, error) {
	type manifestJSON struct {
		SchemaVersion     string                   `json:"schema_version"`
		Sources           []model.SourceSpec       `json:"sources"`
		Backend           model.BackendInfo        `json:"backend"`
		Renderer          string                   `json:"renderer"`
		Style             string                   `json:"style"`
		Warnings          []string                 `json:"warnings"`
		NextSteps         []string                 `json:"next_steps,omitempty"`
		Audit             model.ManifestAudit      `json:"audit"`
		SourceDiagnostics []model.SourceDiagnostic `json:"source_diagnostics,omitempty"`
		OutputFiles       []model.OutputFile       `json:"output_files"`
	}
	warnings := append([]string(nil), manifest.Warnings...)
	if warnings == nil {
		warnings = []string{}
	}
	return writeJSONFile(path, manifestJSON{
		SchemaVersion:     manifest.SchemaVersion,
		Sources:           manifest.Sources,
		Backend:           manifest.Backend,
		Renderer:          manifest.Renderer,
		Style:             manifest.Style,
		Warnings:          warnings,
		NextSteps:         manifest.NextSteps,
		Audit:             manifest.Audit,
		SourceDiagnostics: manifest.SourceDiagnostics,
		OutputFiles:       manifest.OutputFiles,
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
