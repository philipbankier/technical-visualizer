package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type GatherOptions struct {
	CacheDir         string
	MaxFiles         int
	MaxBytesPerFile  int64
	MaxDocsPages     int
	Offline          bool
	OperationTimeout time.Duration
}

func DefaultGatherOptions() GatherOptions {
	return GatherOptions{
		MaxFiles:         120,
		MaxBytesPerFile:  512 * 1024,
		MaxDocsPages:     12,
		OperationTimeout: 30 * time.Second,
	}
}

func GatherAll(ctx context.Context, specs []model.SourceSpec, opts GatherOptions) (model.EvidenceBundle, error) {
	opts = normalizeGatherOptions(opts)
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources:       append([]model.SourceSpec(nil), specs...),
	}

	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return bundle, err
		}

		result := gatherResult{}
		switch spec.Kind {
		case model.SourceMarkdown:
			if isRemoteSource(spec) {
				result = gatherRemoteMarkdown(ctx, spec, opts)
			} else {
				result = gatherLocalMarkdown(ctx, spec, opts)
			}
		case model.SourceJSONPacket:
			result = gatherLocalJSONPacket(ctx, spec, opts)
		case model.SourcePDF:
			if isRemoteSource(spec) {
				result = gatherRemotePDF(ctx, spec, opts)
			} else {
				result = gatherLocalPDF(ctx, spec, opts)
			}
		case model.SourceLocalRepo:
			result = gatherLocalRepo(ctx, spec, opts)
		case model.SourceDocsSite:
			result = gatherDocsSite(ctx, spec, opts)
		case model.SourceGitHubRepo:
			result = gatherGitHubRepo(ctx, spec, opts)
		default:
			result.Warnings = append(result.Warnings, fmt.Sprintf("source %s has unsupported kind %q", spec.ID, spec.Kind))
		}

		bundle.Items = append(bundle.Items, result.Items...)
		bundle.Warnings = append(bundle.Warnings, result.Warnings...)
		bundle.Redactions = append(bundle.Redactions, result.Redactions...)
	}

	return bundle, nil
}

type gatherResult struct {
	Items      []model.EvidenceItem
	Warnings   []string
	Redactions []model.Redaction
}

func normalizeGatherOptions(opts GatherOptions) GatherOptions {
	defaults := DefaultGatherOptions()
	if opts.MaxFiles <= 0 {
		opts.MaxFiles = defaults.MaxFiles
	}
	if opts.MaxBytesPerFile <= 0 {
		opts.MaxBytesPerFile = defaults.MaxBytesPerFile
	}
	if opts.MaxDocsPages <= 0 {
		opts.MaxDocsPages = defaults.MaxDocsPages
	}
	if opts.OperationTimeout <= 0 {
		opts.OperationTimeout = defaults.OperationTimeout
	}
	return opts
}

func operationContext(ctx context.Context, opts GatherOptions) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(opts.OperationTimeout)
	if existing, ok := ctx.Deadline(); ok && existing.Before(deadline) {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, opts.OperationTimeout)
}

func sourceTarget(spec model.SourceSpec) string {
	if spec.Resolved != "" {
		return spec.Resolved
	}
	return spec.Input
}

func isRemoteSource(spec model.SourceSpec) bool {
	return isHTTPURL(sourceTarget(spec))
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func sourceWarning(spec model.SourceSpec, format string, args ...any) string {
	return fmt.Sprintf("source %s: %s", spec.ID, fmt.Sprintf(format, args...))
}

func evidenceID(parts ...string) string {
	return stableID(strings.Join(parts, "\x00"))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
