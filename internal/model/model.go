package model

type SourceKind string

const (
	SourceGitHubRepo SourceKind = "github_repo"
	SourceLocalRepo  SourceKind = "local_repo"
	SourceDocsSite   SourceKind = "docs_site"
	SourceMarkdown   SourceKind = "markdown"
	SourceJSONPacket SourceKind = "json_visual_packet"
	SourcePDF        SourceKind = "pdf"
)

type SourceSpec struct {
	ID       string            `json:"id"`
	Kind     SourceKind        `json:"kind"`
	Input    string            `json:"input"`
	Resolved string            `json:"resolved,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type EvidenceBundle struct {
	SchemaVersion string         `json:"schema_version"`
	Sources       []SourceSpec   `json:"sources"`
	Items         []EvidenceItem `json:"items"`
	Warnings      []string       `json:"warnings,omitempty"`
	Redactions    []Redaction    `json:"redactions,omitempty"`
}

type EvidenceItem struct {
	ID         string            `json:"id"`
	SourceID   string            `json:"source_id"`
	Kind       string            `json:"kind"`
	Title      string            `json:"title"`
	Text       string            `json:"text,omitempty"`
	Path       string            `json:"path,omitempty"`
	URL        string            `json:"url,omitempty"`
	SHA256     string            `json:"sha256,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	SourceRefs []string          `json:"source_refs,omitempty"`
}

type Redaction struct {
	SourceID string `json:"source_id"`
	Path     string `json:"path,omitempty"`
	Reason   string `json:"reason"`
}

type VisualPacket struct {
	SchemaVersion string          `json:"schema_version"`
	ArtifactGoal  string          `json:"artifact_goal"`
	Audience      string          `json:"audience"`
	Title         string          `json:"title"`
	Thesis        string          `json:"thesis"`
	RequiredText  []string        `json:"required_text"`
	RankedClaims  []Claim         `json:"ranked_claims"`
	Facts         []Fact          `json:"facts,omitempty"`
	Sections      []Section       `json:"sections,omitempty"`
	ContentBlocks []ContentBlock  `json:"content_blocks,omitempty"`
	Metrics       []Metric        `json:"metrics,omitempty"`
	Timeline      []TimelineEvent `json:"timeline,omitempty"`
	Entities      []Entity        `json:"entities,omitempty"`
	Tables        []PacketTable   `json:"tables,omitempty"`
	Diagrams      []Diagram       `json:"diagrams,omitempty"`
	OpenQuestions []OpenQuestion  `json:"open_questions,omitempty"`
	Risks         []Risk          `json:"risks,omitempty"`
	Tradeoffs     []Tradeoff      `json:"tradeoffs,omitempty"`
	Unknowns      []Unknown       `json:"unknowns,omitempty"`
	Layout        LayoutSpec      `json:"layout"`
	Style         StyleSpec       `json:"style"`
	Constraints   []string        `json:"constraints,omitempty"`
	SourceRefs    []SourceRef     `json:"source_refs,omitempty"`
}

type Claim struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Kind       string   `json:"kind"`
	Confidence string   `json:"confidence"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Fact struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Section struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Items   []string `json:"items,omitempty"`
}

type ContentBlock struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Text       string   `json:"text,omitempty"`
	Items      []string `json:"items,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Metric struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Value      string   `json:"value"`
	Unit       string   `json:"unit,omitempty"`
	Context    string   `json:"context,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type TimelineEvent struct {
	ID         string   `json:"id"`
	Date       string   `json:"date,omitempty"`
	Label      string   `json:"label"`
	Summary    string   `json:"summary,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Entity struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Detail     string   `json:"detail,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type PacketTable struct {
	ID         string     `json:"id"`
	Title      string     `json:"title,omitempty"`
	Headers    []string   `json:"headers,omitempty"`
	Rows       [][]string `json:"rows,omitempty"`
	SourceRefs []string   `json:"source_refs,omitempty"`
}

type Diagram struct {
	ID         string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	Kind       string   `json:"kind"`
	Text       string   `json:"text"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type OpenQuestion struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Context    string   `json:"context,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Risk struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Severity   string   `json:"severity"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Tradeoff struct {
	ID         string   `json:"id"`
	Choice     string   `json:"choice"`
	Reason     string   `json:"reason"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type Unknown struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type LayoutSpec struct {
	Format      string   `json:"format"`
	Orientation string   `json:"orientation"`
	Regions     []string `json:"regions,omitempty"`
}

type StyleSpec struct {
	Name     string `json:"name"`
	Renderer string `json:"renderer"`
}

type SourceRef struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Label    string `json:"label"`
	Locator  string `json:"locator,omitempty"`
}

type Manifest struct {
	SchemaVersion string        `json:"schema_version"`
	Sources       []SourceSpec  `json:"sources,omitempty"`
	Backend       BackendInfo   `json:"backend"`
	Renderer      string        `json:"renderer"`
	Style         string        `json:"style"`
	Warnings      []string      `json:"warnings,omitempty"`
	NextSteps     []string      `json:"next_steps,omitempty"`
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

type BackendInfo struct {
	Name   string `json:"name"`
	Remote bool   `json:"remote"`
	Model  string `json:"model,omitempty"`
}

type OutputFile struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}
