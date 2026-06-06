package analyzer

import "github.com/dhohner/inktrail/internal/parser"

// Registry maps parser languages to analyzers.
type Registry struct {
	byLanguage map[parser.Language]Analyzer
}

// NewRegistry creates a registry from explicit analyzer implementations.
func NewRegistry(analyzers ...Analyzer) Registry {
	byLanguage := make(map[parser.Language]Analyzer, len(analyzers))
	for _, analyzer := range analyzers {
		byLanguage[analyzer.Language()] = analyzer
	}
	return Registry{byLanguage: byLanguage}
}

// ForPath returns the analyzer and parser language for path, if supported.
func (r Registry) ForPath(path string) (Analyzer, parser.Language, bool) {
	language, ok := parser.LanguageForPath(path)
	if !ok {
		return nil, "", false
	}
	analyzer, ok := r.byLanguage[language]
	return analyzer, language, ok
}
