// Package report renders a run's Result to a self-contained HTML report. Values
// are escaped by html/template and there are no external resources (no scripts,
// fonts or stylesheets fetched), so the report opens offline (FR-05).
//
// It shows the target, the SLO verdict, and the N-run latency aggregate
// (percentiles with mean/min/max/stddev) plus the error rate. A per-window
// timeline is a future enhancement: it needs the collector to bucket samples by
// time, which v1 does not do — so the report does not fake one.
package report

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

// Render produces the self-contained HTML for a result.
func Render(r model.Result) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, r); err != nil {
		return "", fmt.Errorf("report: rendering: %w", err)
	}
	return buf.String(), nil
}

var tmpl = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sapper report — {{.Scenario}}</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 15px/1.5 system-ui, sans-serif; margin: 2rem auto; max-width: 52rem; padding: 0 1rem; }
  h1 small { font-weight: 400; color: #888; }
  .verdict { display: inline-block; padding: .3rem .9rem; border-radius: .4rem; font-weight: 700; color: #fff; }
  .verdict.pass { background: #1a7f37; }
  .verdict.fail { background: #b60205; }
  .abort { background: #b60205; color: #fff; padding: .5rem .9rem; border-radius: .4rem; margin: 1rem 0; }
  table { border-collapse: collapse; width: 100%; margin: .5rem 0 1.5rem; }
  th, td { text-align: left; padding: .4rem .6rem; border-bottom: 1px solid #8884; }
  tr.bad td { color: #b60205; }
  footer { color: #888; font-size: .85rem; margin-top: 2rem; }
</style>
</head>
<body>
<h1>{{.Scenario}} <small>({{.Profile}})</small></h1>
{{if .Aborted}}<div class="abort">ABORTED: {{.AbortReason}}</div>{{end}}
<p class="verdict {{if .Verdict.Passed}}pass{{else}}fail{{end}}">{{if .Verdict.Passed}}PASS{{else}}FAIL{{end}}</p>

<h2>Target</h2>
<table>
  <tr><td>Base URL</td><td>{{.Target.BaseURL}}</td></tr>
  <tr><td>Tier</td><td>{{.Target.Tier}}</td></tr>
  <tr><td>Caps</td><td>{{.Target.Caps.MaxConcurrency}} concurrent · {{.Target.Caps.MaxDuration}} max</td></tr>
</table>

<h2>SLOs</h2>
<table>
  <tr><th>SLO</th><th>result</th><th>detail</th></tr>
  {{range .Verdict.Results}}<tr class="{{if .Passed}}ok{{else}}bad{{end}}"><td>{{.Name}}</td><td>{{if .Passed}}ok{{else}}FAIL{{end}}</td><td>{{.Detail}}</td></tr>
  {{end}}
</table>

<h2>Latency — mean/min/max/stddev over {{.Metrics.Runs}} runs</h2>
<table>
  <tr><th>percentile</th><th>mean</th><th>min</th><th>max</th><th>stddev</th></tr>
  <tr><td>p50</td><td>{{.Metrics.P50.Mean}}</td><td>{{.Metrics.P50.Min}}</td><td>{{.Metrics.P50.Max}}</td><td>{{.Metrics.P50.StdDev}}</td></tr>
  <tr><td>p90</td><td>{{.Metrics.P90.Mean}}</td><td>{{.Metrics.P90.Min}}</td><td>{{.Metrics.P90.Max}}</td><td>{{.Metrics.P90.StdDev}}</td></tr>
  <tr><td>p99</td><td>{{.Metrics.P99.Mean}}</td><td>{{.Metrics.P99.Min}}</td><td>{{.Metrics.P99.Max}}</td><td>{{.Metrics.P99.StdDev}}</td></tr>
</table>

<h2>Errors</h2>
<p>transport error rate: mean {{printf "%.4f" .Metrics.ErrorRate.Mean}} (max {{printf "%.4f" .Metrics.ErrorRate.Max}} across runs)</p>

<footer>Sapper — sustained adversarial load, asserted against declared SLOs. A per-window timeline is a future enhancement; this report shows the N-run aggregate.</footer>
</body>
</html>
`))
