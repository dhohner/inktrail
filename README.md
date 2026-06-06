# inktrail

`inktrail` writes a JSONL impact report for code touched by Git diffs. It is designed for coding agents and reviewers that need a compact map of changed files, changed Go and Java symbols, moved or deleted symbols, removed call edges, entry points, and relevant call graph nodes.

## Quick start

```sh
go run ./cmd/inktrail
```

When stdin and stdout are terminals, `inktrail` opens an interactive selector for staged changes, one commit, or a commit range. In non-TTY contexts it analyzes the staged diff.

For agent-safe automation, use `--no-ui` or `--agent`:

```sh
go run ./cmd/inktrail --agent > inktrail.jsonl
```

With `--agent`, `inktrail` analyzes staged changes. If nothing is staged and the worktree is clean, it falls back to `HEAD`. If unstaged or untracked changes exist, it refuses the fallback so the report cannot accidentally describe a mixed worktree.

## Usage

```sh
go run ./cmd/inktrail                # interactive selector in a TTY; staged diff otherwise
go run ./cmd/inktrail --no-ui        # staged diff, or HEAD when clean and nothing is staged
go run ./cmd/inktrail --agent        # alias for --no-ui
```

The range selector also accepts `base..head` in the interactive UI.

## Output format

Output is JSONL: one compact JSON object per line. The first record is always `summary`, followed by zero or more detail records.

### Record types

- `summary`: counts for files, test files, changed symbols, deleted symbols, moved symbols, removed calls, entry points, and nodes.
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
  - `hunks`: old/new line ranges plus changed `lines` (`op`, `old_line`, `new_line`, `content`).
  - `hunks_omitted`, `preview`, `omitted_lines`: present when a large added, generated, vendor, or binary file is summarized instead of emitted in full.
  - `moved_lines_omitted`: number of hunk lines omitted because they were part of a detected equal-body symbol move.
- `changed_symbol`: current symbol containing added or modified production-code lines.
- `deleted_symbol`: symbol present in the base graph but absent from the current graph.
- `moved_symbol`: symbol whose body moved from one symbol ID to another, with `body_sha256_equal` when the body content is unchanged.
- `removed_call`: call edge present in the base graph but absent from the current graph.
- `entry_point`: root caller that reaches changed symbols.
- `node`: relevant current call graph node with call sites and changed line ranges.

## Example

```jsonl
{"type":"summary","files":2,"test_files":1,"changed_symbols":1,"deleted_symbols":1,"moved_symbols":1,"removed_calls":1,"entry_points":1,"nodes":1}
{"type":"file","status":"modified","path":"service/b.go","test":false,"language":"go","classification":["source"],"diffstat":{"added_lines":1,"deleted_lines":1,"added_bytes":28,"deleted_bytes":25},"symbols":["service/b.go::service.ServiceB.Do"],"content_ref":{"kind":"workspace_file","path":"service/b.go"},"hunks":[{"old_start":18,"old_lines":1,"new_start":18,"new_lines":1,"lines":[{"op":"delete","old_line":18,"content":"old.RepositoryOld{}.Get()"},{"op":"add","new_line":18,"content":"repository.RepositoryB{}.Get()"}]}]}
{"type":"changed_symbol","id":"service/b.go::service.ServiceB.Do"}
{"type":"deleted_symbol","id":"repository/old.go::repository.RepositoryOld.Get"}
{"type":"moved_symbol","from":"service/old.go::service.OldName","to":"service/new.go::service.NewName","body_sha256_equal":true}
{"type":"removed_call","from":"service/b.go::service.ServiceB.Do","to":"repository/old.go::repository.RepositoryOld.Get","call_site":{"path":"service/b.go","line":18}}
{"type":"entry_point","id":"controller/a.go::controller.ControllerA.Handle"}
{"type":"node","id":"service/b.go::service.ServiceB.Do","path":"service/b.go","name":"Do","kind":"method","start_line":16,"end_line":22,"calls":[{"to":"repository/b.go::repository.RepositoryB.Get","call_site":{"path":"service/b.go","line":19}}],"changed":true,"changed_lines":[{"start":18,"end":20}],"package":"service"}
```

## Analysis scope

Diff scope:

- staged changes by default, one commit against its parent, or an explicit two-ref range
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

Java scope:

- Maven and Gradle source layouts are recognized by path convention.
- Production Java is any `.java` file outside test-only Java source sets.
- Test-only Java source sets include `src/test/java`, `src/integrationTest/java`, `src/functionalTest/java`, and `src/e2eTest/java`.
- Tree-sitter provides parsing. Java call resolution is syntactic and repository-local: it uses package/class/method names plus source-level imports for repository-local types and static methods. It does not read Maven/Gradle dependency graphs or model dependency scopes, generated sources, or annotation processors. Reports omit uncertain external call edges.

Skipped for symbol and call analysis:

- unsupported languages: emitted as `file` records only; Inktrail writes one stderr warning per run and keeps JSONL output on stdout clean
- test-only code: `*_test.go`, `test/`, `tests/`, `testdata/`, and Java test source sets listed above
- vendor directories
- blank added lines

## Development

```sh
go test ./...
```
