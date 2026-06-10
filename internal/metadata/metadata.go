package metadata

import (
	"os/exec"
	"runtime/debug"
	"strings"
)

const (
	Name           = "inktrail"
	SchemaVersion  = "1.0"
	defaultVersion = "0.3.0"
)

// Version and Commit may be overridden at build time with:
//
//	-ldflags "-X github.com/dhohner/inktrail/internal/metadata.Version=... -X github.com/dhohner/inktrail/internal/metadata.Commit=..."
//
// When not overridden, Current falls back to Go build metadata. If local Go VCS
// stamping is unavailable, it asks git only for immutable refs (describe/rev-parse),
// not diffs or worktree state.
var (
	Version = ""
	Commit  = ""
)

var SupportedLanguages = []string{"go", "java"}

type Info struct {
	Name               string   `json:"name"`
	Version            string   `json:"version"`
	Commit             string   `json:"commit"`
	SchemaVersion      string   `json:"schema_version"`
	SupportedLanguages []string `json:"supported_languages"`
}

func Current() Info {
	version := Version
	commit := Commit
	if version == "" || commit == "" {
		if build, ok := debug.ReadBuildInfo(); ok {
			if version == "" {
				version = build.Main.Version
			}
			if commit == "" {
				commit = vcsRevision(build.Settings)
			}
		}
	}
	if version == "" || version == "(devel)" {
		version = gitVersion()
	}
	if version == "" {
		version = defaultVersion
	}
	if commit == "" {
		commit = gitCommit()
	}
	if commit == "" {
		commit = "unknown"
	}
	return Info{
		Name:               Name,
		Version:            version,
		Commit:             shortCommit(commit),
		SchemaVersion:      SchemaVersion,
		SupportedLanguages: append([]string(nil), SupportedLanguages...),
	}
}

func vcsRevision(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return ""
}

func gitVersion() string {
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*").Output()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short=7", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func shortCommit(commit string) string {
	if len(commit) > 7 && commit != "unknown" {
		return commit[:7]
	}
	return commit
}
