# inktrail

AI-agent JSONL report for Go code touched by git diffs.

`inktrail` helps autonomous coding agents review changes by producing an impact report: changed files, changed hunk code, changed symbols, deleted symbols, removed call edges, entry points, and relevant call graph nodes.

## Usage

```sh
go run ./cmd/inktrail                # interactive target selector (or staged diff when stdio is non-TTY)
go run ./cmd/inktrail --no-ui        # agent-safe staged diff, or HEAD if nothing is staged and worktree is clean
go run ./cmd/inktrail --agent        # alias for --no-ui
go run ./cmd/inktrail <commit>       # report for one commit vs <commit>^
go run ./cmd/inktrail <base> <head>  # report for commit range
```

Output is JSONL: one compact JSON object per line. `file` records include changed hunk lines so review agents can inspect the patch without re-reading git diff output.

## Record types

- `summary`: counts for files, test files, changed symbols, deleted symbols, removed calls, entry points, and nodes
- `file`: changed file metadata
  - `status`: `added`, `modified`, `deleted`, or `renamed`
  - `old_path`: source path for renamed/deleted files when available; omitted for unchanged paths
  - `path`: current path
  - `test`: whether file is test-only scope
  - `hunks`: old/new line ranges plus changed `lines` (`op`, `old_line`, `new_line`, `content`)
- `changed_symbol`: current symbol containing added/modified production-code lines (`id`)
- `deleted_symbol`: symbol present in the base AST but absent from current AST (`id`)
- `removed_call`: call edge present in the base AST but absent from current AST
- `entry_point`: root caller that reaches changed symbols (`id`)
- `node`: relevant current call graph node with call sites and changed line ranges

## Example

```jsonl
{"type":"summary","files":2,"test_files":1,"changed_symbols":1,"deleted_symbols":1,"removed_calls":1,"entry_points":1,"nodes":1}
{"type":"file","status":"modified","path":"service/b.go","test":false,"hunks":[{"old_start":16,"old_lines":4,"new_start":16,"new_lines":5,"lines":[{"op":"delete","old_line":18,"content":"old.RepositoryOld{}.Get()"},{"op":"add","new_line":18,"content":"repository.RepositoryB{}.Get()"}]}]}
{"type":"changed_symbol","id":"service/b.go::service.ServiceB.Do"}
{"type":"deleted_symbol","id":"repository/old.go::repository.RepositoryOld.Get"}
{"type":"removed_call","from":"service/b.go::service.ServiceB.Do","to":"repository/old.go::repository.RepositoryOld.Get","call_site":{"path":"service/b.go","line":18}}
{"type":"entry_point","id":"controller/a.go::controller.ControllerA.Handle"}
{"type":"node","id":"service/b.go::service.ServiceB.Do","path":"service/b.go","name":"Do","kind":"method","start_line":16,"end_line":22,"calls":[{"to":"repository/b.go::repository.RepositoryB.Get","call_site":{"path":"service/b.go","line":19}}],"changed":true,"changed_lines":[{"start":18,"end":20}],"package":"service"}
```

## Scope

Diff scope:

- target-side added/modified Go lines for `changed_symbol`
- file metadata and hunk ranges for production and test files
- deleted files and deleted-only hunks included in `file` records
- renamed files included as `status: "renamed"`

AST scope:

- current AST builds `changed_symbol`, `entry_point`, and `node` records
- base AST builds `deleted_symbol` and `removed_call` records
- base ref selection:
  - staged diff: `HEAD`
  - one commit: `<commit>^`
  - range: `<base>`

Skipped for symbol/call analysis:

- test-only code: `*_test.go`, `test/`, `tests/`, `testdata/`
- formatting-only diffs by default (`--ignore-all-space`, `--ignore-blank-lines`)
- blank added lines
