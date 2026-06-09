package report

import (
	"encoding/json"
	"html"
	"io"
	"sort"
	"strings"
)

// WriteHTML writes a self-contained human-readable HTML report.
func WriteHTML(w io.Writer, r Report) error {
	_, err := io.WriteString(w, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Inktrail Report</title>
<style>
:root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;line-height:1.45}body{margin:2rem;max-width:1100px}h1,h2{line-height:1.1}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(130px,1fr));gap:.75rem}.card{border:1px solid #9995;border-radius:8px;padding:.75rem}.num{font-size:1.6rem;font-weight:700}table{border-collapse:collapse;width:100%;margin:1rem 0}th,td{border-bottom:1px solid #9995;text-align:left;padding:.45rem;vertical-align:top}code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}pre{white-space:pre-wrap;background:#9992;border-radius:8px;padding:.75rem;overflow:auto}.muted{color:#777}.pill{display:inline-block;border:1px solid #9995;border-radius:999px;padding:.1rem .45rem;margin:.05rem}.graph-wrap{border:1px solid #9995;border-radius:10px;padding:1rem}.impact-note{margin-top:0}.impact-grid{display:grid;gap:1rem}.impact-stats{display:flex;flex-wrap:wrap;gap:.5rem}.impact-stats .pill{background:#9991}.impact-section{border-top:1px solid #9995;padding-top:.8rem}.impact-section h3{margin:.1rem 0 .5rem}.impact-list{display:grid;gap:.35rem;margin:.5rem 0;padding-left:1.2rem}.impact-chains{display:grid;gap:.75rem}.impact-tree{overflow:auto}.impact-branch{display:grid;gap:.45rem;margin:.35rem 0 .35rem 1rem;padding-left:1rem;border-left:1px solid #9995}.impact-row{display:flex;gap:.45rem;align-items:flex-start}.impact-row>.impact-node{min-width:220px;max-width:320px}.impact-arrow{color:#777;font-weight:700;padding-top:.45rem}.impact-node{border-left:4px solid #999;padding:.25rem .5rem;background:#9991}.impact-node.entry{border-left-color:#4d86d9}.impact-node.changed{border-left-color:#c58a13}.impact-node.context{border-left-color:#777}.impact-node.deleted,.impact-node.removed{border-left-color:#b33}.impact-node.moved{border-left-color:#6b43aa}.graph-label{font:13px ui-monospace,SFMono-Regular,Menlo,monospace;color:CanvasText}.graph-meta{font-size:12px;color:#777;margin-top:.1rem}.graph-edge.call td:first-child{border-left:4px solid #777}.graph-edge.removed td:first-child{border-left:4px solid #b33}.graph-edge.moved td:first-child{border-left:4px solid #6b43aa}.graph-edge.context td:first-child{border-left:4px solid #777}details{margin:.5rem 0}summary{cursor:pointer;font-weight:600}
</style>
</head>
<body>
<h1>Inktrail Report</h1>
<h2>Summary</h2>
<div class="grid">
`)
	if err != nil {
		return err
	}
	writeCard(w, "Files", r.Summary.Files)
	writeCard(w, "Test files", r.Summary.TestFiles)
	writeCard(w, "Changed symbols", r.Summary.ChangedSymbols)
	writeCard(w, "Deleted symbols", r.Summary.DeletedSymbols)
	writeCard(w, "Moved symbols", r.Summary.MovedSymbols)
	writeCard(w, "Removed calls", r.Summary.RemovedCalls)
	writeCard(w, "Entry points", r.Summary.EntryPoints)
	writeCard(w, "Nodes", r.Summary.Nodes)

	if _, err := io.WriteString(w, `</div>
<h2>Files</h2>
<table><thead><tr><th>Status</th><th>Path</th><th>Lines</th><th>Tags</th></tr></thead><tbody>
`); err != nil {
		return err
	}
	for _, f := range r.Files {
		path := f.Path
		if path == "" {
			path = f.OldPath
		}
		record := fileRecord(f, nil)
		if _, err := io.WriteString(w, "<tr><td>"+html.EscapeString(record.Status)+"</td><td><code>"+html.EscapeString(path)+"</code></td><td>+"+itoa(record.DiffStat.AddedLines)+" / -"+itoa(record.DiffStat.DeletedLines)+"</td><td>"+joinPills(fileTags(record))+"</td></tr>\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, `</tbody></table>
`); err != nil {
		return err
	}
	if err := writeImpactGraph(w, r); err != nil {
		return err
	}
	if len(r.DeletedSymbols) != 0 {
		if _, err := io.WriteString(w, `<h2>Deleted symbols</h2>
`); err != nil {
			return err
		}
		writeList(w, r.DeletedSymbols)
	}
	if len(r.MovedSymbols) != 0 {
		if _, err := io.WriteString(w, `<h2>Moved symbols</h2>
<table><thead><tr><th>From</th><th>To</th><th>Body equal</th></tr></thead><tbody>
`); err != nil {
			return err
		}
		for _, m := range r.MovedSymbols {
			if _, err := io.WriteString(w, "<tr><td><code>"+html.EscapeString(m.From)+"</code></td><td><code>"+html.EscapeString(m.To)+"</code></td><td>"+itoaBool(m.BodySHA256Equal)+"</td></tr>\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, `</tbody></table>
`); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, `</body></html>
`)
	return err
}

func writeImpactGraph(w io.Writer, r Report) error {
	data, err := json.Marshal(htmlReportData(r))
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, `<h2>Impact graph</h2>
<div id="impact-graph" class="graph-wrap" aria-label="Complete impact graph">`+impactGraphHTML(r)+`</div>
<script type="application/json" id="inktrail-report-data">`+string(data)+`</script>
`)
	return err
}

type impactNode struct {
	id, label, kind, path, pkg string
	classes                    []string
	layer                      int
}

type impactEdge struct{ from, to, kind string }

func impactGraphHTML(r Report) string {
	nodes, edges := impactGraphModel(r)
	if len(nodes) == 0 {
		return `<p class="muted">No graph nodes available.</p>`
	}
	sort.SliceStable(nodes, func(i, j int) bool { return impactNodeKey(nodes[i]) < impactNodeKey(nodes[j]) })
	sort.SliceStable(edges, func(i, j int) bool {
		return edges[i].kind+edges[i].from+edges[i].to < edges[j].kind+edges[j].from+edges[j].to
	})
	layers := map[int][]impactNode{}
	outgoing := map[string][]impactEdge{}
	for _, n := range nodes {
		layers[n.layer] = append(layers[n.layer], n)
	}
	for _, e := range edges {
		outgoing[e.from] = append(outgoing[e.from], e)
	}
	var layerIDs []int
	for l := range layers {
		layerIDs = append(layerIDs, l)
	}
	sort.Ints(layerIDs)
	var b strings.Builder
	b.WriteString(`<div class="impact-grid">`)
	b.WriteString(`<p class="impact-note muted">Graph layout is replaced with reviewer-focused tables: start with entry points, then inspect relationships and grouped symbols. All nodes and edges are still rendered.</p>`)
	b.WriteString(`<div class="impact-stats"><span class="pill">` + itoa(len(nodes)) + ` nodes</span><span class="pill">` + itoa(len(edges)) + ` relationships</span><span class="pill">` + itoa(len(r.EntryPoints)) + ` entry points</span></div>`)
	b.WriteString(`<section class="impact-section"><h3>Entry point impact chains</h3>` + renderEntryPointChains(r.EntryPoints, edges, nodes) + renderNonCallImpact(edges, nodes) + `</section>`)
	b.WriteString(`</div>`)
	return b.String()
}

func renderEntryPointChains(entryPoints []string, edges []impactEdge, nodes []impactNode) string {
	if len(entryPoints) == 0 {
		return `<p class="muted">No entry points reported.</p>`
	}
	adj := map[string][]impactEdge{}
	for _, e := range edges {
		if e.kind == "call" {
			adj[e.from] = append(adj[e.from], e)
		}
	}
	for id := range adj {
		sort.SliceStable(adj[id], func(i, j int) bool { return adj[id][i].to < adj[id][j].to })
	}
	var b strings.Builder
	b.WriteString(`<div class="impact-chains">`)
	for _, entry := range sortedStrings(entryPoints) {
		b.WriteString(`<details open><summary>` + html.EscapeString(edgeTargetLabel(entry, nodes)) + `</summary><div class="impact-tree">`)
		b.WriteString(renderCallTree(entry, adj, nodes, map[string]bool{entry: true}))
		b.WriteString(`</div></details>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderCallTree(id string, adj map[string][]impactEdge, nodes []impactNode, seen map[string]bool) string {
	var b strings.Builder
	b.WriteString(`<div class="impact-row">` + renderImpactNode(findImpactNode(id, nodes)) + `</div>`)
	if len(adj[id]) == 0 {
		return b.String()
	}
	b.WriteString(`<div class="impact-branch">`)
	for _, e := range adj[id] {
		b.WriteString(`<div data-from="` + html.EscapeString(e.from) + `" data-to="` + html.EscapeString(e.to) + `" class="graph-edge ` + html.EscapeString(e.kind) + `"><div class="impact-row"><span class="impact-arrow">→</span>`)
		if seen[e.to] {
			b.WriteString(renderImpactNode(impactNode{id: e.to, label: edgeTargetLabel(e.to, nodes), kind: "cycle", classes: []string{"derived"}}))
		} else {
			nextSeen := map[string]bool{}
			for k, v := range seen {
				nextSeen[k] = v
			}
			nextSeen[e.to] = true
			b.WriteString(`<div>` + renderCallTree(e.to, adj, nodes, nextSeen) + `</div>`)
		}
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderNonCallImpact(edges []impactEdge, nodes []impactNode) string {
	var filtered []impactEdge
	for _, e := range edges {
		if e.kind != "call" && e.kind != "context" {
			filtered = append(filtered, e)
		}
	}
	var items []impactNode
	for _, n := range nodes {
		if hasClass(n, "deleted") || hasClass(n, "moved") || hasClass(n, "removed") {
			items = append(items, n)
		}
	}
	if len(filtered) == 0 && len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<details><summary>Other impact items (` + itoa(len(items)) + `)</summary>`)
	if len(filtered) != 0 {
		b.WriteString(renderRelationshipTable(filtered, nodes))
	}
	if len(items) != 0 {
		b.WriteString(`<ul class="impact-list">`)
		for _, n := range items {
			b.WriteString(`<li>` + renderImpactNode(n) + `</li>`)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</details>`)
	return b.String()
}

func renderRelationshipTable(edges []impactEdge, nodes []impactNode) string {
	if len(edges) == 0 {
		return `<p class="muted">No relationships reported.</p>`
	}
	var b strings.Builder
	b.WriteString(`<table><thead><tr><th>From</th><th>Relationship</th><th>To</th></tr></thead><tbody>`)
	for _, e := range edges {
		from := findImpactNode(e.from, nodes)
		to := findImpactNode(e.to, nodes)
		b.WriteString(`<tr data-from="` + html.EscapeString(e.from) + `" data-to="` + html.EscapeString(e.to) + `" class="graph-edge ` + html.EscapeString(e.kind) + `"><td>` + renderImpactNode(from) + `</td><td>` + html.EscapeString(edgeVerb(e.kind)) + `</td><td>` + renderImpactNode(to) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func renderImpactNode(n impactNode) string {
	if n.id == "" {
		return `<span class="muted">Unknown</span>`
	}
	classes := append([]string{"impact-node"}, n.classes...)
	meta := strings.Join(nonEmpty(n.kind, n.pkg, n.path), " · ")
	return `<div data-id="` + html.EscapeString(n.id) + `" class="` + strings.Join(classes, " ") + `"><div class="graph-label">` + html.EscapeString(firstNonEmpty(n.label, n.id)) + `</div><div class="graph-meta">` + html.EscapeString(meta) + `</div></div>`
}

func renderOutgoingSummary(id string, outgoing map[string][]impactEdge, nodes []impactNode) string {
	edges := outgoing[id]
	if len(edges) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="graph-meta">`)
	for i, e := range edges {
		if i != 0 {
			b.WriteString(`; `)
		}
		b.WriteString(html.EscapeString(edgeVerb(e.kind)) + ` → <code>` + html.EscapeString(edgeTargetLabel(e.to, nodes)) + `</code>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func findImpactNode(id string, nodes []impactNode) impactNode {
	for _, n := range nodes {
		if n.id == id {
			return n
		}
	}
	return impactNode{id: id, label: shortGraphLabel(id), kind: "missing endpoint", classes: []string{"derived"}}
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func impactGraphModel(r Report) ([]impactNode, []impactEdge) {
	changed := stringSet(r.ChangedSymbols)
	entries := stringSet(r.EntryPoints)
	callers := map[string]bool{}
	for _, n := range r.Nodes {
		for _, c := range n.Calls {
			if changed[c.To] {
				callers[n.ID] = true
			}
		}
	}
	nodeMap := map[string]impactNode{}
	upsert := func(n impactNode) {
		if n.id == "" {
			return
		}
		if old, ok := nodeMap[n.id]; ok {
			n.classes = appendUnique(old.classes, n.classes...)
			if n.label == "" {
				n.label = old.label
			}
			if n.kind == "" {
				n.kind = old.kind
			}
			if n.path == "" {
				n.path = old.path
			}
			if n.pkg == "" {
				n.pkg = old.pkg
			}
			if old.layer < n.layer {
				n.layer = old.layer
			}
		}
		nodeMap[n.id] = n
	}
	var edges []impactEdge
	for _, n := range r.Nodes {
		classes := []string{}
		if entries[n.ID] {
			classes = append(classes, "entry")
		}
		if changed[n.ID] || n.Changed {
			classes = append(classes, "changed")
		}
		upsert(impactNode{id: n.ID, label: firstNonEmpty(n.Name, n.ID), kind: n.Kind, path: n.Path, pkg: n.Package, classes: classes, layer: graphLayerID(n.ID, len(n.Calls) != 0, changed[n.ID] || n.Changed, entries, callers)})
		for _, c := range n.Calls {
			edges = append(edges, impactEdge{n.ID, c.To, "call"})
			if _, ok := nodeMap[c.To]; !ok {
				upsert(impactNode{id: c.To, label: shortGraphLabel(c.To), kind: "callee", classes: []string{"derived"}, layer: 3})
			}
		}
		if n.Package != "" {
			ctx := "package:" + n.Package
			upsert(impactNode{id: ctx, label: n.Package, kind: "package group", classes: []string{"context"}, layer: 5})
		}
		if n.Path != "" {
			ctx := "file:" + n.Path
			upsert(impactNode{id: ctx, label: n.Path, kind: "file group", path: n.Path, classes: []string{"context"}, layer: 5})
		}
	}
	for _, s := range r.ChangedSymbols {
		upsert(impactNode{id: s, label: shortGraphLabel(s), kind: "changed symbol", classes: []string{"changed"}, layer: 2})
	}
	for _, s := range r.DeletedSymbols {
		upsert(impactNode{id: s, label: shortGraphLabel(s), kind: "deleted symbol", classes: []string{"deleted"}, layer: 4})
	}
	for _, m := range r.MovedSymbols {
		upsert(impactNode{id: m.From, label: shortGraphLabel(m.From), kind: "moved from", classes: []string{"moved"}, layer: 4})
		upsert(impactNode{id: m.To, label: shortGraphLabel(m.To), kind: "moved to", classes: []string{"moved"}, layer: 4})
		edges = append(edges, impactEdge{m.From, m.To, "moved"})
	}
	for _, c := range r.RemovedCalls {
		upsert(impactNode{id: c.From, label: shortGraphLabel(c.From), kind: "removed-call caller", classes: []string{"removed"}, layer: 1})
		upsert(impactNode{id: c.To, label: shortGraphLabel(c.To), kind: "removed-call callee", classes: []string{"removed"}, layer: 4})
		edges = append(edges, impactEdge{c.From, c.To, "removed"})
	}
	for _, c := range r.Contexts {
		if c.Path != "" {
			ctx := "file:" + c.Path
			upsert(impactNode{id: ctx, label: c.Path, kind: "file group", path: c.Path, classes: []string{"context"}, layer: 5})
		}
	}
	for _, f := range r.Files {
		path := firstNonEmpty(f.Path, f.OldPath)
		if path != "" {
			upsert(impactNode{id: "file:" + path, label: path, kind: "changed file", path: path, classes: []string{"context"}, layer: 5})
		}
	}
	var nodes []impactNode
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}
	return nodes, edges
}

func normalizeImpactLayers(nodes map[string]impactNode, edges []impactEdge) {
	for range nodes {
		changed := false
		for _, e := range edges {
			if e.kind == "context" {
				continue
			}
			from, okFrom := nodes[e.from]
			to, okTo := nodes[e.to]
			if !okFrom || !okTo || to.layer > from.layer {
				continue
			}
			to.layer = from.layer + 1
			nodes[e.to] = to
			changed = true
		}
		if !changed {
			return
		}
	}
}

func edgeVerb(kind string) string {
	switch kind {
	case "removed":
		return "removed call"
	case "moved":
		return "moved"
	case "context":
		return "groups"
	default:
		return "calls"
	}
}

func edgeTargetLabel(id string, nodes []impactNode) string {
	for _, n := range nodes {
		if n.id == id {
			return firstNonEmpty(n.label, n.id)
		}
	}
	return shortGraphLabel(id)
}

func graphLayerLabel(layer int, nodes []impactNode) string {
	if allHaveClass(nodes, "context") {
		return "File / package groups"
	}
	if anyHaveClass(nodes, "deleted", "moved", "removed") {
		return "Deleted / moved / removed"
	}
	if anyHaveClass(nodes, "entry") {
		return "Entry points"
	}
	if layer == 1 {
		return "Callers"
	}
	if anyHaveClass(nodes, "changed") {
		return "Changed symbols"
	}
	return "Callees / related"
}

func anyHaveClass(nodes []impactNode, classes ...string) bool {
	want := stringSet(classes)
	for _, n := range nodes {
		for _, class := range n.classes {
			if want[class] {
				return true
			}
		}
	}
	return false
}

func allHaveClass(nodes []impactNode, class string) bool {
	if len(nodes) == 0 {
		return false
	}
	for _, n := range nodes {
		if !hasClass(n, class) {
			return false
		}
	}
	return true
}

func hasClass(n impactNode, class string) bool {
	for _, c := range n.classes {
		if c == class {
			return true
		}
	}
	return false
}

func shortGraphLabel(id string) string {
	if i := strings.LastIndex(id, "::"); i >= 0 && i+2 < len(id) {
		return id[i+2:]
	}
	if i := strings.LastIndex(id, "."); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return id
}

func impactNodeKey(n impactNode) string {
	return itoa(n.layer) + "\x00" + n.pkg + "\x00" + n.path + "\x00" + n.kind + "\x00" + n.label + "\x00" + n.id
}

func graphLayerID(id string, hasCalls, isChanged bool, entries, callers map[string]bool) int {
	if entries[id] {
		return 0
	}
	if callers[id] {
		return 1
	}
	if isChanged {
		return 2
	}
	if hasCalls {
		return 3
	}
	return 4
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}

func appendUnique(values []string, more ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range append(values, more...) {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func htmlReportData(r Report) map[string]any {
	files := make([]FileRecord, 0, len(r.Files))
	for _, file := range r.Files {
		files = append(files, fileRecord(file, nil))
	}
	return map[string]any{
		"summary":         r.Summary,
		"files":           files,
		"changed_symbols": r.ChangedSymbols,
		"deleted_symbols": r.DeletedSymbols,
		"moved_symbols":   r.MovedSymbols,
		"removed_calls":   r.RemovedCalls,
		"entry_points":    r.EntryPoints,
		"contexts":        r.Contexts,
		"nodes":           r.Nodes,
	}
}

func writeCard(w io.Writer, label string, n int) {
	_, _ = io.WriteString(w, "<div class=\"card\"><div class=\"num\">"+itoa(n)+"</div><div>"+html.EscapeString(label)+"</div></div>\n")
}

func writeList(w io.Writer, values []string) {
	if len(values) == 0 {
		_, _ = io.WriteString(w, "<p class=\"muted\">None</p>\n")
		return
	}
	_, _ = io.WriteString(w, "<ul>\n")
	for _, v := range values {
		_, _ = io.WriteString(w, "<li><code>"+html.EscapeString(v)+"</code></li>\n")
	}
	_, _ = io.WriteString(w, "</ul>\n")
}

func fileTags(record FileRecord) []string {
	if len(record.ChangeIntent) != 0 {
		return record.ChangeIntent
	}
	return record.Classification
}

func joinPills(values []string) string {
	out := ""
	for _, v := range values {
		out += "<span class=\"pill\">" + html.EscapeString(v) + "</span>"
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func itoaBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
