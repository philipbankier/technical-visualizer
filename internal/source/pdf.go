package source

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type pdfExtractor interface {
	ExtractPDFText(context.Context, string, GatherOptions) (pdfExtraction, error)
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
	defaultPDFExtractor        pdfExtractor = unavailablePDFExtractor{}
)

type unavailablePDFExtractor struct{}

func (unavailablePDFExtractor) ExtractPDFText(context.Context, string, GatherOptions) (pdfExtraction, error) {
	return pdfExtraction{}, errPDFExtractorUnavailable
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
