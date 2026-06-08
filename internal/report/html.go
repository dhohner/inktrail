package report

import (
	"html"
	"io"
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
:root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;line-height:1.45}body{margin:2rem;max-width:1100px}h1,h2{line-height:1.1}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(130px,1fr));gap:.75rem}.card{border:1px solid #9995;border-radius:8px;padding:.75rem}.num{font-size:1.6rem;font-weight:700}table{border-collapse:collapse;width:100%;margin:1rem 0}th,td{border-bottom:1px solid #9995;text-align:left;padding:.45rem;vertical-align:top}code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}pre{white-space:pre-wrap;background:#9992;border-radius:8px;padding:.75rem;overflow:auto}.muted{color:#777}.pill{display:inline-block;border:1px solid #9995;border-radius:999px;padding:.1rem .45rem;margin:.05rem}
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
	if len(r.ChangedSymbols) != 0 {
		if _, err := io.WriteString(w, `<h2>Changed symbols</h2>
`); err != nil {
			return err
		}
		writeList(w, r.ChangedSymbols)
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
