package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

const minUsefulPDFLetters = 200

type pdfExtractor interface {
	ExtractPDFText(context.Context, string, GatherOptions) (pdfExtraction, error)
}

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error)
	LookPath(file string) (string, error)
}

type pdfExtraction struct {
	Engine         string
	Version        string
	PageCount      int
	ExtractedPages int
	Truncated      bool
	Text           string
	PageRefs       []pdfPageRef
	Warnings       []string
}

type pdfPageRef struct {
	Page int
	Ref  string
}

var (
	errPDFExtractorUnavailable              = errors.New("pdf extractor unavailable")
	defaultPDFExtractor        pdfExtractor = popplerExtractor{runner: execCommandRunner{}}
)

type unavailablePDFExtractor struct{}

func (unavailablePDFExtractor) ExtractPDFText(context.Context, string, GatherOptions) (pdfExtraction, error) {
	return pdfExtraction{}, errPDFExtractorUnavailable
}

type execCommandRunner struct{}

func (execCommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	// #nosec G204 -- name is resolved through exec.LookPath for the fixed pdftotext binary; args are passed without a shell.
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type popplerExtractor struct {
	runner commandRunner
}

func (p popplerExtractor) ExtractPDFText(ctx context.Context, path string, opts GatherOptions) (pdfExtraction, error) {
	runner := p.runner
	if runner == nil {
		runner = execCommandRunner{}
	}
	pdftotext, err := runner.LookPath("pdftotext")
	if err != nil {
		return pdfExtraction{}, errPDFExtractorUnavailable
	}

	opCtx, cancel := operationContext(ctx, opts)
	defer cancel()

	versionOut, versionErr, err := runner.Run(opCtx, pdftotext, "-v")
	if err != nil {
		return pdfExtraction{}, fmt.Errorf("pdftotext version failed: %w", err)
	}
	version := strings.TrimSpace(string(versionOut))
	if version == "" {
		version = strings.TrimSpace(string(versionErr))
	}

	stdout, stderr, err := runner.Run(opCtx, pdftotext, "-layout", "-enc", "UTF-8", path, "-")
	if err != nil {
		return pdfExtraction{}, fmt.Errorf("pdftotext extraction failed: %w: %s", err, strings.TrimSpace(string(stderr)))
	}

	raw := string(stdout)
	extractedPages := countPDFTextPages(raw)
	text := normalizePDFTextToMarkdown(raw)
	return pdfExtraction{
		Engine:         "pdftotext",
		Version:        version,
		PageCount:      extractedPages,
		ExtractedPages: extractedPages,
		Text:           text,
		PageRefs:       pdfPageRefs(extractedPages),
	}, nil
}

func gatherPDFText(ctx context.Context, spec model.SourceSpec, localPath string, displayLocator string, sha256 string, opts GatherOptions) gatherResult {
	extraction, err := defaultPDFExtractor.ExtractPDFText(ctx, localPath, opts)
	if err != nil {
		if errors.Is(err, errPDFExtractorUnavailable) {
			return gatherResult{Warnings: []string{sourceWarning(spec, "PDF text extraction unavailable for %q; install Poppler pdftotext to enable PDF evidence", displayLocator)}}
		}
		return gatherResult{Warnings: []string{sourceWarning(spec, "PDF text extraction failed for %q: %v", displayLocator, err)}}
	}

	result := gatherResult{}
	for _, warning := range extraction.Warnings {
		if strings.TrimSpace(warning) != "" {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "%s", warning))
		}
	}

	text, redactions := redactLikelySecrets(spec.ID, displayLocator, extraction.Text)
	result.Redactions = append(result.Redactions, redactions...)
	text = strings.TrimSpace(text)
	if text == "" {
		result.Warnings = append(result.Warnings, sourceWarning(spec, "PDF text extraction produced no usable text for %q", displayLocator))
		return result
	}
	if !hasUsefulPDFText(text) {
		result.Warnings = append(result.Warnings, sourceWarning(spec, "PDF text extraction produced too little readable text for %q", displayLocator))
		return result
	}

	item := model.EvidenceItem{
		ID:         evidenceID(spec.ID, displayLocator),
		SourceID:   spec.ID,
		Kind:       "pdf_text",
		Title:      filepath.Base(displayLocator),
		Text:       text,
		SHA256:     sha256,
		Metadata:   pdfMetadata(extraction),
		SourceRefs: pdfSourceRefs(displayLocator, extraction),
	}
	if isRemoteSource(spec) {
		item.URL = displayLocator
	} else {
		item.Path = localPath
	}
	result.Items = append(result.Items, item)
	return result
}

var (
	pdfAlphaHyphenLineBreakPattern = regexp.MustCompile(`(\pL)-\n(\pL)`)
	pdfNumberedHeadingPattern      = regexp.MustCompile(`^\d+(?:\.\d+)*\s+\pL`)
	pdfCommonHeadingPattern        = regexp.MustCompile(`(?i)^(abstract|introduction|references|appendix(?:\s+[A-Z0-9]+)?)$`)
	pdfBlankRunPattern             = regexp.MustCompile(`\n{3,}`)
)

func normalizePDFTextToMarkdown(raw string) string {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = pdfAlphaHyphenLineBreakPattern.ReplaceAllString(text, "$1$2")

	pages := strings.Split(text, "\f")
	normalizedPages := make([]string, 0, len(pages))
	for i, page := range pages {
		page = normalizePDFPageText(page)
		if strings.TrimSpace(page) == "" && i == len(pages)-1 {
			continue
		}
		if i > 0 {
			page = fmt.Sprintf("<!-- page %d -->\n\n%s", i+1, page)
		}
		normalizedPages = append(normalizedPages, page)
	}

	text = strings.Join(normalizedPages, "\n\n")
	text = pdfBlankRunPattern.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func normalizePDFPageText(page string) string {
	lines := strings.Split(page, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			normalized = append(normalized, "")
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			normalized = append(normalized, trimmed)
			continue
		}
		if isPDFHeading(trimmed) {
			normalized = append(normalized, "# "+trimmed)
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

func isPDFHeading(line string) bool {
	return pdfCommonHeadingPattern.MatchString(line) || pdfNumberedHeadingPattern.MatchString(line)
}

func hasUsefulPDFText(text string) bool {
	var letters int
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if letters >= minUsefulPDFLetters {
				return true
			}
		}
	}
	return false
}

func countPDFTextPages(raw string) int {
	raw = strings.TrimRight(raw, "\f\r\n\t ")
	if raw == "" {
		return 0
	}
	return len(strings.Split(raw, "\f"))
}

func pdfPageRefs(pageCount int) []pdfPageRef {
	refs := make([]pdfPageRef, 0, pageCount)
	for page := 1; page <= pageCount; page++ {
		refs = append(refs, pdfPageRef{Page: page})
	}
	return refs
}

func pdfMetadata(extraction pdfExtraction) map[string]string {
	metadata := map[string]string{}
	if extraction.Engine != "" {
		metadata["pdf_engine"] = extraction.Engine
	}
	if extraction.Version != "" {
		metadata["pdf_engine_version"] = extraction.Version
	}
	if extraction.PageCount > 0 {
		metadata["pdf_page_count"] = strconv.Itoa(extraction.PageCount)
	}
	if extraction.ExtractedPages > 0 {
		metadata["pdf_pages_extracted"] = strconv.Itoa(extraction.ExtractedPages)
	}
	metadata["pdf_truncated"] = strconv.FormatBool(extraction.Truncated)
	return metadata
}

func pdfSourceRefs(displayLocator string, extraction pdfExtraction) []string {
	refs := make([]string, 0, len(extraction.PageRefs))
	seen := map[string]bool{}
	add := func(ref string) {
		if ref == "" || seen[ref] {
			return
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	for _, pageRef := range extraction.PageRefs {
		if pageRef.Page > 0 {
			add(fmt.Sprintf("%s#page=%d", displayLocator, pageRef.Page))
			continue
		}
		add(pageRef.Ref)
	}
	if len(refs) == 0 {
		add(displayLocator)
	}
	return refs
}
