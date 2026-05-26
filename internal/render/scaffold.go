package render

import (
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

var scaffoldTemplate = template.Must(template.New("scaffold").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root {
      color-scheme: dark;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #101418;
      color: #eef4f8;
    }
    body {
      margin: 0;
      background: #101418;
    }
    main {
      box-sizing: border-box;
      max-width: 1200px;
      min-height: 100vh;
      margin: 0 auto;
      padding: 44px;
    }
    header {
      border-bottom: 1px solid #31404a;
      padding-bottom: 24px;
      margin-bottom: 28px;
    }
    h1 {
      margin: 0 0 16px;
      font-size: 42px;
      line-height: 1.05;
      letter-spacing: 0;
    }
    h2 {
      margin: 0 0 14px;
      font-size: 20px;
      letter-spacing: 0;
      color: #9fd9ff;
    }
    h3 {
      margin: 16px 0 8px;
      font-size: 16px;
      letter-spacing: 0;
      color: #d6e1e8;
    }
    p {
      line-height: 1.55;
      color: #d6e1e8;
    }
    section {
      margin: 0 0 28px;
      padding: 22px;
      border: 1px solid #2c3942;
      border-radius: 8px;
      background: #171d22;
    }
    ul {
      margin: 0;
      padding-left: 20px;
    }
    li {
      margin: 10px 0;
      line-height: 1.45;
    }
    pre {
      overflow-x: auto;
      padding: 14px;
      border: 1px solid #2c3942;
      border-radius: 6px;
      background: #101418;
      color: #d6e1e8;
      white-space: pre-wrap;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      color: #d6e1e8;
    }
    th,
    td {
      border-bottom: 1px solid #2c3942;
      padding: 8px 10px;
      text-align: left;
      vertical-align: top;
    }
    th {
      color: #9fd9ff;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 16px;
    }
    .meta {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 18px;
      color: #aebcc5;
      font-size: 13px;
    }
    .pill {
      border: 1px solid #3e515e;
      border-radius: 999px;
      padding: 6px 10px;
      background: #1e2930;
    }
    .label {
      color: #9aaab4;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    .text {
      display: block;
      margin-top: 3px;
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div class="label">Visualization Scaffold</div>
      <h1>{{.Title}}</h1>
      <p>{{.Thesis}}</p>
      <div class="meta">
        <span class="pill">goal: {{.ArtifactGoal}}</span>
        <span class="pill">audience: {{.Audience}}</span>
        <span class="pill">style: {{.Style.Name}}</span>
        <span class="pill">renderer: {{.Style.Renderer}}</span>
        <span class="pill">layout: {{.Layout.Format}} {{.Layout.Orientation}}</span>
      </div>
    </header>

    <section>
      <h2>Required Text</h2>
      <ul>
        {{range .RequiredText}}<li>{{.}}</li>{{else}}<li>No required text supplied.</li>{{end}}
      </ul>
    </section>

    {{if .Metrics}}
    <section>
      <h2>Key Metrics</h2>
      <ul>
{{range .Metrics}}        <li><span class="text">{{.Label}}: {{.Value}}</span><span class="label">{{.Context}}</span></li>
{{end}}
      </ul>
    </section>
    {{end}}

    {{if .Timeline}}
    <section>
      <h2>Timeline</h2>
      <ul>
{{range .Timeline}}        <li><span class="text">{{.Date}} - {{.Label}}</span><span class="label">{{.Summary}}</span></li>
{{end}}
      </ul>
    </section>
    {{end}}

    {{if .Tables}}
    <section>
      <h2>Tables</h2>
{{range .Tables}}      <h3>{{.Title}}</h3>
      <table>
        {{if .Headers}}<thead><tr>{{range .Headers}}<th>{{.}}</th>{{end}}</tr></thead>{{end}}
        <tbody>
{{range .Rows}}          <tr>{{range .}}<td>{{.}}</td>{{end}}</tr>
{{end}}        </tbody>
      </table>
{{end}}
    </section>
    {{end}}

    {{if .Diagrams}}
    <section>
      <h2>Diagrams</h2>
{{range .Diagrams}}      <h3>{{.Title}}</h3>
      <pre>{{.Text}}</pre>
{{end}}
    </section>
    {{end}}

    {{if .OpenQuestions}}
    <section>
      <h2>Open Questions</h2>
      <ul>
{{range .OpenQuestions}}        <li>{{.Text}}</li>
{{end}}
      </ul>
    </section>
    {{end}}

    {{if .ContentBlocks}}
    <section>
      <h2>Content Blocks</h2>
      <ul>
{{range .ContentBlocks}}        <li><span class="text">{{.Title}}</span><span class="label">{{.Summary}}</span></li>
{{end}}
      </ul>
    </section>
    {{end}}

    <section>
      <h2>Claims</h2>
      <ul>
{{range .RankedClaims}}        <li><span class="text">{{.Text}}</span><span class="label">confidence: {{.Confidence}} | kind: {{.Kind}} | refs: {{range .SourceRefs}}{{.}} {{end}}</span></li>
{{else}}        <li>No ranked claims supplied.</li>
{{end}}
      </ul>
    </section>

    <section>
      <h2>Facts</h2>
      <ul>
{{range .Facts}}        <li><span class="text">{{.Text}}</span><span class="label">refs: {{range .SourceRefs}}{{.}} {{end}}</span></li>
{{else}}        <li>No facts supplied.</li>
{{end}}
      </ul>
    </section>

    <div class="grid">
      <section>
        <h2>Risks</h2>
        <ul>
{{range .Risks}}          <li><span class="text">{{.Text}}</span><span class="label">severity: {{.Severity}} | refs: {{range .SourceRefs}}{{.}} {{end}}</span></li>
{{else}}          <li>No risks supplied.</li>
{{end}}
        </ul>
      </section>

      <section>
        <h2>Tradeoffs</h2>
        <ul>
{{range .Tradeoffs}}          <li><span class="text">{{.Choice}}</span><span class="text">{{.Reason}}</span><span class="label">refs: {{range .SourceRefs}}{{.}} {{end}}</span></li>
{{else}}          <li>No tradeoffs supplied.</li>
{{end}}
        </ul>
      </section>

      <section>
        <h2>Unknowns</h2>
        <ul>
{{range .Unknowns}}          <li><span class="text">{{.Text}}</span><span class="label">refs: {{range .SourceRefs}}{{.}} {{end}}</span></li>
{{else}}          <li>No unknowns supplied.</li>
{{end}}
        </ul>
      </section>
    </div>

    <section>
      <h2>Source References</h2>
      <ul>
{{range .SourceRefs}}        <li><span class="text">{{.ID}} | {{.SourceID}} | {{.Label}}</span><span class="label">{{.Locator}}</span></li>
{{else}}        <li>No source references supplied.</li>
{{end}}
      </ul>
    </section>

    <section>
      <h2>Constraints</h2>
      <ul>
        {{range .Constraints}}<li>{{.}}</li>{{else}}<li>No constraints supplied.</li>{{end}}
      </ul>
    </section>
  </main>
</body>
</html>
`))

func WriteScaffold(w io.Writer, packet model.VisualPacket) error {
	return scaffoldTemplate.Execute(w, packet)
}

func WriteScaffoldFile(path string, packet model.VisualPacket) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// #nosec G304 -- path is the caller-selected bundle output file.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	return WriteScaffold(file, packet)
}
