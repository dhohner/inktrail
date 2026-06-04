package report

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"

	"inktrail/internal/diff"
)

const (
	largeAddedFileBytes = 15 * 1024
	largeAddedFileLines = 300
	previewLines        = 20
)

func fileRecord(file diff.FileChange, symbols []string) FileRecord {
	stat := diffStat(file)
	added := addedContents(file)
	compact := file.Status == "added" && (stat.AddedBytes > largeAddedFileBytes || stat.AddedLines > largeAddedFileLines || isGenerated(file.Path, added) || isVendorPath(file.Path) || isBinaryPath(file.Path))
	record := FileRecord{
		Type:           "file",
		Status:         file.Status,
		OldPath:        file.OldPath,
		Path:           file.Path,
		Test:           file.Test,
		Language:       language(file.Path),
		Classification: classification(file, added),
		DiffStat:       stat,
		Symbols:        symbols,
		RiskFlags:      riskFlags(file, added, compact),
		ContentRef:     contentRef(file, added),
	}
	if compact {
		record.HunksOmitted = true
		record.OmittedLines = totalHunkLines(file.Hunks)
		record.Preview = preview(added)
		return record
	}
	record.Hunks = file.Hunks
	return record
}

func diffStat(file diff.FileChange) DiffStat {
	var stat DiffStat
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			switch line.Op {
			case "add":
				stat.AddedLines++
				stat.AddedBytes += len(line.Content)
			case "delete":
				stat.DeletedLines++
				stat.DeletedBytes += len(line.Content)
			}
		}
	}
	return stat
}

func addedContents(file diff.FileChange) []string {
	var lines []string
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Op == "add" {
				lines = append(lines, line.Content)
			}
		}
	}
	return lines
}

func contentRef(file diff.FileChange, added []string) *ContentRef {
	if file.Path == "" || file.Status == "deleted" {
		return nil
	}
	ref := &ContentRef{Kind: "workspace_file", Path: file.Path}
	if file.Status == "added" {
		sum := sha256.Sum256([]byte(strings.Join(added, "\n")))
		ref.SHA256 = hex.EncodeToString(sum[:])
	}
	return ref
}

func preview(lines []string) *FilePreview {
	if len(lines) == 0 {
		return nil
	}
	limit := previewLines
	if len(lines) <= limit*2 {
		return &FilePreview{HeadLines: lines, OmittedLines: 0}
	}
	return &FilePreview{
		HeadLines:    append([]string(nil), lines[:limit]...),
		TailLines:    append([]string(nil), lines[len(lines)-limit:]...),
		OmittedLines: len(lines) - limit*2,
	}
}

func totalHunkLines(hunks []diff.Hunk) int {
	total := 0
	for _, hunk := range hunks {
		total += len(hunk.Lines)
	}
	return total
}

func symbolsByPath(nodes []Node) map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, node := range nodes {
		if seen[node.Path] == nil {
			seen[node.Path] = map[string]bool{}
		}
		if seen[node.Path][node.ID] {
			continue
		}
		seen[node.Path][node.ID] = true
		out[node.Path] = append(out[node.Path], node.ID)
	}
	for path := range out {
		sort.Strings(out[path])
	}
	return out
}

func classification(file diff.FileChange, added []string) []string {
	var out []string
	if file.Test {
		out = append(out, "test")
	}
	if isVendorPath(file.Path) {
		out = append(out, "vendor")
	}
	if isGenerated(file.Path, added) {
		out = append(out, "generated")
	}
	if isBinaryPath(file.Path) {
		out = append(out, "binary")
	}
	if len(out) == 0 {
		out = append(out, "source")
	}
	return out
}

func riskFlags(file diff.FileChange, added []string, compact bool) []string {
	flags := map[string]bool{}
	if compact {
		flags["large_added_file"] = true
	}
	for _, c := range classification(file, added) {
		if c != "source" {
			flags[c] = true
		}
	}
	content := strings.ToLower(strings.Join(added, "\n"))
	checks := map[string][]string{
		"auth":           {"auth", "oauth", "jwt", "session", "permission"},
		"secret":         {"secret", "password", "passwd", "api_key", "apikey", "token"},
		"crypto":         {"crypto", "sha256", "md5", "encrypt", "decrypt", "cipher"},
		"external_input": {"http.", "url.", "json.", "formvalue", "query", "body"},
		"command_exec":   {"exec.", "os/exec", "system("},
		"sql":            {"select ", "insert ", "update ", "delete ", "sql."},
	}
	for flag, needles := range checks {
		for _, needle := range needles {
			if strings.Contains(content, needle) {
				flags[flag] = true
				break
			}
		}
	}
	return sortedKeys(flags)
}

func isGenerated(path string, lines []string) bool {
	lowerPath := strings.ToLower(path)
	if strings.Contains(lowerPath, "generated") || strings.Contains(lowerPath, ".pb.go") || strings.Contains(lowerPath, "_gen.") {
		return true
	}
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "code generated") || strings.Contains(lower, "do not edit") || strings.Contains(lower, "auto-generated") {
			return true
		}
	}
	return false
}

func isVendorPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		switch part {
		case "vendor", "node_modules", "third_party", "dist", "build":
			return true
		}
	}
	return false
}

func isBinaryPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz", ".tar", ".jar", ".woff", ".woff2", ".ttf", ".otf", ".mp4", ".mov":
		return true
	default:
		return false
	}
}

func language(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	default:
		return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	}
}
