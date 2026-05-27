package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/philipbankier/technical-visualizer/internal/backend"
	"github.com/philipbankier/technical-visualizer/internal/pipeline"
)

const Version = "0.1.0"

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintf(stdout, "visualize %s\n", Version)
		return 0
	case "doctor":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "visualize doctor does not accept arguments: %s\n", strings.Join(args[1:], " "))
			return 2
		}
		printDoctor(stdout)
		return 0
	case "make":
		return runPipeline(args[1:], stdout, stderr)
	case "gather", "packet", "render":
		fmt.Fprintf(stderr, "unsupported command %q in v0.1: use visualize make <sources...> for the full pipeline\n", args[0])
		return 2
	default:
		return runPipeline(args, stdout, stderr)
	}
}

func runPipeline(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parsePipelineArgs(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	manifest, err := pipeline.Run(context.Background(), opts)
	if err != nil {
		fmt.Fprintf(stderr, "visualize failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Wrote visualization bundle to %s\n", opts.OutputDir)
	fmt.Fprintf(stdout, "Backend: %s\n", manifest.Backend.Name)
	handoffMode := strings.ToLower(strings.TrimSpace(opts.Handoff))
	if handoffMode == "codex" {
		fmt.Fprintf(stdout, "Codex handoff: %s\n", filepath.Join(opts.OutputDir, "handoff", "codex-prompt.md"))
	}
	if strings.EqualFold(strings.TrimSpace(opts.Pack), "auto") {
		fmt.Fprintf(stdout, "Content pack: %s\n", filepath.Join(opts.OutputDir, "content-pack.json"))
		fmt.Fprintf(stdout, "Pack briefs: %s\n", filepath.Join(opts.OutputDir, "pack"))
	}
	if handoffMode == "codex" && opts.Quick {
		promptPath := filepath.Join(opts.OutputDir, "handoff", "codex-prompt.md")
		fmt.Fprintf(stdout, "POSIX shell: codex -C %s \"$(cat < %s)\"\n", shellQuote(opts.OutputDir), shellQuote(promptPath))
	}
	return 0
}

func parsePipelineArgs(args []string, stderr io.Writer) (pipeline.Options, error) {
	opts := pipeline.Options{}
	fs := flag.NewFlagSet("visualize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.OutputDir, "o", "visualize-output", "output directory")
	fs.StringVar(&opts.OutputDir, "out", "visualize-output", "output directory")
	fs.StringVar(&opts.Backend, "backend", "local", "image backend: local, openai, auto, or hybrid")
	fs.StringVar(&opts.Renderer, "renderer", "hybrid", "renderer: html, hybrid, or image")
	fs.StringVar(&opts.Style, "style", "executive-dark", "visual style name")
	fs.StringVar(&opts.Goal, "goal", "architecture-map", "artifact goal")
	fs.BoolVar(&opts.Offline, "offline", false, "skip remote source fetching")
	fs.StringVar(&opts.Handoff, "handoff", "", "handoff package: codex")
	fs.BoolVar(&opts.Quick, "quick", false, "print a ready manual command for the selected handoff")
	fs.StringVar(&opts.Pack, "pack", "off", "content pack mode: off or auto")
	fs.Usage = func() { printUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return pipeline.Options{}, err
	}
	opts.Sources = fs.Args()
	if len(opts.Sources) == 0 {
		return pipeline.Options{}, fmt.Errorf("missing source: use visualize make <sources...>")
	}
	return opts, nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: visualize [flags] <sources...>
       visualize make [flags] <sources...>
       visualize doctor
       visualize --version

Commands:
  make      Build a visualization bundle.
  doctor    Report backend status without requiring remote credentials.
  version   Print the CLI version.

Common flags:
  -o, --out       Output directory.
  --backend       local, openai, auto, or hybrid. Defaults to local.
  --renderer      html, hybrid, or image.
  --style         Visual style name.
  --goal          Artifact goal.
  --offline       Skip remote source fetching.
  --handoff       Optional handoff package, currently codex.
  --quick         Print a ready manual command for the selected handoff.
  --pack          Content pack mode: off or auto. Defaults to off.

Unsupported in v0.1:
  gather, packet, render, codex image generation
`)
}

func printDoctor(w io.Writer) {
	ctx := context.Background()
	openAI := backend.NewOpenAIBackend(backend.OpenAIConfig{}).Available(ctx)
	codex := backend.NewCodexBackend(backend.CodexConfig{}).Available(ctx)

	fmt.Fprintln(w, "Backend status")
	fmt.Fprintln(w, "local renderer: available (local, fallback PNG and scaffold output)")
	printNamedCapability(w, "OpenAI Images API", openAI, "direct image backend for --backend openai")
	printNamedCapability(w, "Codex CLI", codex, "agent workflow only, not a direct image backend")
	printPopplerStatus(w)
}

func printNamedCapability(w io.Writer, name string, capability backend.Capability, note string) {
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
	fmt.Fprintf(w, "%s: %s (%s, %s; %s)\n", name, status, remote, reason, note)
}

func printPopplerStatus(w io.Writer) {
	path, err := exec.LookPath("pdftotext")
	if err != nil {
		fmt.Fprintln(w, "Poppler pdftotext: unavailable (install Poppler for PDF-only sources)")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// #nosec G204 -- path comes from exec.LookPath for the fixed pdftotext binary.
	cmd := exec.CommandContext(ctx, path, "-v")
	out, err := cmd.CombinedOutput()
	version := firstNonEmptyLine(string(out))
	if err != nil || version == "" {
		version = "version unknown"
	}
	fmt.Fprintf(w, "Poppler pdftotext: available (local, %s)\n", version)
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
