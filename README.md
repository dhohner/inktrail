# inktrail

`inktrail` writes a JSONL impact report for code touched by Git diffs. It is designed for coding agents and reviewers that need a compact map of changed files, changed Go and Java symbols, moved or deleted symbols, removed call edges, entry points, and relevant call graph nodes.

## Quick start

```sh
go run ./cmd/inktrail
```

`inktrail` writes JSONL to stdout by default. With no arguments it analyzes the staged diff.

To analyze `HEAD` when there is no staged diff and the worktree is clean, use `--fallback-to-head`:

```sh
go run ./cmd/inktrail --fallback-to-head > inktrail.jsonl
```

With `--fallback-to-head`, `inktrail` analyzes staged changes first. If nothing is staged and the worktree is clean, it falls back to `HEAD`. If unstaged or untracked changes exist, it refuses the fallback so the report cannot accidentally describe a mixed worktree.

## Usage

```sh
go run ./cmd/inktrail                         # staged diff
go run ./cmd/inktrail HEAD                    # one commit
go run ./cmd/inktrail main feature            # commit range
go run ./cmd/inktrail --base main --head feature # named commit range
go run ./cmd/inktrail --fallback-to-head      # staged diff, or HEAD when clean and nothing is staged
go run ./cmd/inktrail --best-effort          # emit partial records plus structured warnings for analysis gaps
go run ./cmd/inktrail --review-summary       # add a compact agent-targeted review summary record
go run ./cmd/inktrail --include 'internal/**/*.go'
go run ./cmd/inktrail --exclude '**/*_test.go' --exclude-vendor
go run ./cmd/inktrail --changed-only
go run ./cmd/inktrail --max-lines-per-hunk 40 --max-context-lines 60 --max-records 500 --budget-tokens 12000
```

## Agent workflows

Prefer explicit output files in automated workflows so later steps can retry parsing without rerunning Git analysis:

```sh
go run ./cmd/inktrail --fallback-to-head --review-summary --output inktrail.jsonl
```

Use JSON output when a single persisted object is easier for your runner to pass between steps:

```sh
go run ./cmd/inktrail --fallback-to-head --review-summary --format json --output inktrail.json
```

For resilient automation over mixed-language changes, combine `--best-effort` with `--review-summary` and check warning records before relying on symbol-level details:

```sh
go run ./cmd/inktrail --fallback-to-head --best-effort --review-summary --output inktrail.jsonl
```

Recommended parsing flow:

1. Read the first `summary` record for counts and omissions.
2. If `--review-summary` was used, read `review_summary` to decide whether the change needs deeper inspection.
3. Inspect `file` records for changed production files, changed test files, classifications, diffstat, hunks, and lazy `content_ref` paths.
4. Inspect `changed_symbol` records and matching `declaration_context` records for changed declaration source.
5. Inspect related `declaration_context` records for direct callers, direct callees, and enclosing declarations.
6. Inspect `removed_call` and `deleted_symbol` records last; these usually require the most reviewer judgment.

Unknown record types are safe to ignore. Prefer the typed record stream over ad-hoc line matching; each JSONL line is a complete JSON object with a `type` field.

## Path filtering

Use path filters to narrow the changed files that seed the report before symbol, context, and graph records are built.

- `--include <glob>` keeps only changed paths that match the glob.
- `--exclude <glob>` removes changed paths that match the glob. Repeat it to exclude multiple patterns.
- `--exclude-vendor` removes vendor-scoped paths such as `vendor/`, `node_modules/`, `third_party/`, `dist/`, and `build/`.
- `--changed-only` limits emitted context to changed files. Without `--changed-only`, include and exclude filters still allow related callers, callees, entry points, and graph nodes when they explain included changed files.

Glob matching is slash-normalized and supports recursive `**` segments. For example, `**/*_test.go` matches test files at any depth.

```sh
go run ./cmd/inktrail --include 'internal/**/*.go'
go run ./cmd/inktrail --exclude '**/*_test.go' --exclude 'vendor/**'
go run ./cmd/inktrail --include 'service/**' --changed-only
```

## Size and token budget controls

Use size controls when an agent needs a bounded prompt input:

- `--max-lines-per-hunk N` emits at most `N` changed lines per hunk and records omitted line counts on the hunk/file plus summary `omissions`, without changing file metadata or diffstat counts.
- `--max-context-lines N` emits at most `N` lines per declaration context excerpt and records truncation details in `excerpt` plus summary `omissions`.
- `--max-records N` keeps the summary and highest-priority detail records, then reports omitted detail counts in summary `omissions`.
- `--budget-tokens N` applies an approximate planning budget using `ceil(serialized_character_count / 4)` over the JSONL or JSON report. This is model-agnostic guidance, not exact tokenizer output.

When records must be omitted, Inktrail preserves the summary first, then the optional `review_summary`, then higher-priority detail records before lower-priority graph-detail records. Priority order is: file records, changed symbols, deleted symbols, declaration contexts, moved symbols, removed calls, entry points, then graph nodes. File records retain their symbol lists even when graph nodes are trimmed. The `review_summary` is not counted as a detail record by `--max-records`, so agents can still see the full first-pass file/symbol/removal map before deciding whether trimmed detail records are sufficient. Very large context omission lists may be grouped by path/relationship; under token-budget pressure, verbose omission metadata may be further aggregated by omission kind/reason so the summary remains compact and machine-readable. If the summary plus optional `review_summary` exceeds `--budget-tokens`, Inktrail emits best-effort output with `budget_floor_exceeded` omission metadata.

## Output format

Output is JSONL: one compact JSON object per line. The first record is always `summary`, followed by zero or more detail records. When `--review-summary` is set, `review_summary` is emitted immediately after `summary` in JSONL output. In `--format json`, it appears as the top-level `review_summary` object.

Strict analysis is the default: unsupported production languages, parse errors, and graph build errors fail the invocation. Use `--best-effort` when partial output is acceptable; Inktrail then emits available report records plus machine-readable `warning` records.

Declaration context and warning records are additive to the existing stream: consumers that only need file, symbol, or graph records can ignore unknown record types and continue processing the report. Context records are factual source/declaration excerpts, not review guidance, risk scoring, policy advice, or semantic framework analysis.

### Record types

- `summary`: counts for files, test files, changed symbols, deleted symbols, moved symbols, removed calls, entry points, nodes, and emitted context records.
  - `context_records.total`: total emitted context records.
  - `context_records.declaration_context`: changed-declaration context records.
  - `context_records.related_declaration_context`: unchanged declarations directly related to changed declarations.
- `review_summary`: optional compact agent-targeted record emitted by `--review-summary`. It groups changed production files, changed test files, changed symbols, deleted symbols, risky removed call edges, and unsupported files when present.
- `warning`: best-effort analysis warning with stable `code`, human-readable `message`, and optional `path` or `symbol` context. Current codes include `unsupported_language`, `parse_error`, and `graph_build_failed`.
- `file`: changed file metadata and changed hunks.
  - `status`: `added`, `modified`, `deleted`, or `renamed`.
  - `old_path`: source path for renamed or deleted files when available.
  - `path`: current path.
  - `test`: whether the path is test-only scope.
  - `language`: detected from extension, when known.
  - `classification`: one or more of `source`, `test`, `generated`, `vendor`, or `binary`.
  - `change_intent`: currently emitted for detected symbol moves/package reorganization.
  - `diffstat`: added/deleted line and byte counts.
  - `symbols`: relevant impacted symbols in the file when known.
  - `content_ref`: lazy-read handle for current workspace files (`kind: "workspace_file"`, `path`, optional `sha256`).
  - `hunks`: old/new line ranges plus changed `lines` (`op`, `old_line`, `new_line`, `content`); hunk `omitted_lines` is present when `--max-lines-per-hunk` truncates a hunk.
  - `hunks_omitted`, `preview`, `omitted_lines`: present when a large added, generated, vendor, or binary file is summarized instead of emitted in full, or when hunk lines are truncated.
  - `moved_lines_omitted`: number of hunk lines omitted because they were part of a detected equal-body symbol move.
- `changed_symbol`: current symbol containing added or modified production-code lines.
- `deleted_symbol`: symbol present in the base graph but absent from the current graph.
- `declaration_context`: bounded source context for a changed or directly related declaration.
  - `id`: symbol ID for the declaration.
  - `path`, `name`, `kind`: source location and declaration identity.
  - `line_range`: declaration start and end lines in the current workspace.
  - `relationship`: `changed_declaration`, `direct_caller`, `direct_callee`, or `enclosing_declaration`.
  - `related_to`: changed symbol ID for related unchanged declarations.
  - `changed_lines`: changed line ranges for changed declarations.
  - `excerpt.content`: declaration source excerpt.
  - `excerpt.truncated`: whether the excerpt was bounded.
  - `excerpt.omitted_lines`: number of source lines omitted from a truncated excerpt.
- `moved_symbol`: symbol whose body moved from one symbol ID to another, with `body_sha256_equal` when the body content is unchanged.
- `removed_call`: call edge present in the base graph but absent from the current graph.
- `entry_point`: root caller that reaches changed symbols.
- `node`: relevant current call graph node with call sites and changed line ranges.

## Example

The summary's `context_records` object is always present. Agents can inspect `context_records.total` on the first JSONL line to decide whether declaration context exists before scanning detail records. `declaration_context` counts changed-declaration context records; `related_declaration_context` counts related caller/callee declaration context records.

```jsonl
{"type":"summary","schema_version":"1.0","files":2,"test_files":1,"changed_symbols":1,"deleted_symbols":1,"moved_symbols":1,"removed_calls":1,"entry_points":1,"nodes":1,"context_records":{"total":2,"declaration_context":1,"related_declaration_context":1}}
{"type":"review_summary","schema_version":"1.0","changed_production_files":[{"path":"service/b.go","status":"modified","language":"go","symbols":["service/b.go::service.ServiceB.Do"]}],"changed_test_files":[{"path":"service/b_test.go","status":"modified","language":"go"}],"changed_symbols":["service/b.go::service.ServiceB.Do"],"deleted_symbols":["repository/old.go::repository.RepositoryOld.Get"],"risky_removed_call_edges":[{"from":"service/b.go::service.ServiceB.Do","to":"repository/old.go::repository.RepositoryOld.Get","call_site":{"path":"service/b.go","line":18}}]}
{"type":"file","status":"modified","path":"service/b.go","test":false,"language":"go","classification":["source"],"diffstat":{"added_lines":1,"deleted_lines":1,"added_bytes":28,"deleted_bytes":25},"symbols":["service/b.go::service.ServiceB.Do"],"content_ref":{"kind":"workspace_file","path":"service/b.go"},"hunks":[{"old_start":18,"old_lines":1,"new_start":18,"new_lines":1,"lines":[{"op":"delete","old_line":18,"content":"old.RepositoryOld{}.Get()"},{"op":"add","new_line":18,"content":"repository.RepositoryB{}.Get()"}]}]}
{"type":"changed_symbol","id":"service/b.go::service.ServiceB.Do"}
{"type":"deleted_symbol","id":"repository/old.go::repository.RepositoryOld.Get"}
{"type":"declaration_context","id":"service/b.go::service.ServiceB.Do","path":"service/b.go","name":"Do","kind":"method","line_range":{"start":16,"end":22},"relationship":"changed_declaration","changed_lines":[{"start":18,"end":20}],"excerpt":{"content":"func (s ServiceB) Do() {\n\trepository.RepositoryB{}.Get()\n}","truncated":false,"omitted_lines":0}}
{"type":"declaration_context","id":"repository/b.go::repository.RepositoryB.Get","path":"repository/b.go","name":"Get","kind":"method","line_range":{"start":8,"end":91},"relationship":"direct_callee","related_to":"service/b.go::service.ServiceB.Do","excerpt":{"content":"func (r RepositoryB) Get() {\n\t// first 80 lines of the declaration excerpt\n}","truncated":true,"omitted_lines":3}}
{"type":"moved_symbol","from":"service/old.go::service.OldName","to":"service/new.go::service.NewName","body_sha256_equal":true}
{"type":"removed_call","from":"service/b.go::service.ServiceB.Do","to":"repository/old.go::repository.RepositoryOld.Get","call_site":{"path":"service/b.go","line":18}}
{"type":"entry_point","id":"controller/a.go::controller.ControllerA.Handle"}
{"type":"node","id":"service/b.go::service.ServiceB.Do","path":"service/b.go","name":"Do","kind":"method","start_line":16,"end_line":22,"calls":[{"to":"repository/b.go::repository.RepositoryB.Get","call_site":{"path":"service/b.go","line":19}}],"changed":true,"changed_lines":[{"start":18,"end":20}],"package":"service"}
```

## Analysis scope

Diff scope:

- staged changes by default, one commit against its parent, or an explicit two-ref range (`<base> <head>` or `--base <ref> --head <ref>`)
- file metadata and hunk ranges for production and test files
- target-side added or modified Go and Java production lines for `changed_symbol`
- deleted files and deleted-only hunks in `file` records
- renamed files as `status: "renamed"`
- whitespace-only and blank-line changes ignored by default (`--ignore-all-space`, `--ignore-blank-lines`)

Graph scope:

- production Go and Java files in the current workspace build `changed_symbol`, `entry_point`, and `node` records
- production Go and Java files at the base ref build `deleted_symbol`, `moved_symbol`, and `removed_call` records
- parsing uses bundled Tree-sitter grammars; symbol and call extraction only runs for languages with an Inktrail analyzer
- base ref selection:
  - staged diff: `HEAD`
  - one commit: `<commit>^`
  - range: `<base>`

Declaration context scope:

- Rich symbol-level context is available for supported Go and Java production analysis.
- With `--best-effort`, unsupported languages still emit `file` records with file metadata, hunks when compact enough, `content_ref` for lazy workspace reads, and `warning` records. They do not emit symbol-level declaration context.
- Context excerpts are bounded to keep reports compact. When an excerpt is truncated, `excerpt.truncated` is `true` and `excerpt.omitted_lines` reports how many lines were left out.
- Large added, generated, vendor, binary, or otherwise compacted file records may omit hunks or previews as described above; declaration context is not emitted for compacted added files.

Java scope:

- Maven and Gradle source layouts are recognized by path convention.
- Production Java is any `.java` file outside test-only Java source sets.
- Test-only Java source sets include `src/test/java`, `src/integrationTest/java`, `src/functionalTest/java`, and `src/e2eTest/java`.
- Tree-sitter provides parsing. Java call resolution is syntactic and repository-local: it uses package/class/method names plus source-level imports for repository-local types and static methods. It does not read Maven/Gradle dependency graphs or model dependency scopes, generated sources, or annotation processors. Reports omit uncertain external call edges.

Skipped for symbol and call analysis:

- unsupported languages: fail by default for production files; with `--best-effort`, emitted as `file` records plus structured `warning` records
- test-only code: `*_test.go`, `test/`, `tests/`, `testdata/`, and Java test source sets listed above
- vendor directories
- blank added lines

## Development

```sh
go test ./...
```
