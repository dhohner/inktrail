// Package graph builds repository-local symbol and call graphs.
//
// Go syntax analysis is backed by internal/parser's shared Tree-sitter layer so
// Java can reuse the same parser foundation. Production/test file filtering and
// Git/object loading are intentionally path- and repository-level concerns, not
// parser behavior. There are currently no Go AST parser exceptions retained for
// symbol or call extraction.
package graph
