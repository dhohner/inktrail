# inktrail

AI-agent JSON report for Go code touched by git diffs.

`inktrail` helps autonomous coding agents review changes by producing a content-free impact report: changed files, hunk ranges, changed symbols, deleted symbols, removed call edges, entry points, and relevant call graph nodes.

## Usage

```sh
go run ./cmd/inktrail                # report for staged diff vs HEAD
go run ./cmd/inktrail <commit>       # report for one commit vs <commit>^
go run ./cmd/inktrail <base> <head>  # report for commit range
```

Output is JSON. Changed source content is intentionally not included.

## Report fields

- `summary`: counts for files, test files, changed symbols, deleted symbols, removed calls, entry points, and nodes
- `files`: changed file metadata
  - `status`: `added`, `modified`, `deleted`, or `renamed`
  - `old_path`: source path for modified/renamed/deleted files when available
  - `path`: current path
  - `test`: whether file is test-only scope
  - `hunks`: old/new line ranges without content
- `changed_symbols`: current symbols containing added/modified production-code lines
- `deleted_symbols`: symbols present in the base AST but absent from current AST
- `removed_calls`: call edges present in the base AST but absent from current AST
- `entry_points`: root callers that reach changed symbols
- `nodes`: relevant current call graph nodes with call sites and changed line ranges

## Example

```json
{
  "summary": {
    "files": 2,
    "test_files": 1,
    "changed_symbols": 1,
    "deleted_symbols": 1,
    "removed_calls": 1,
    "entry_points": 1,
    "nodes": 2
  },
  "files": [
    {
      "status": "modified",
      "old_path": "service/b.go",
      "path": "service/b.go",
      "test": false,
      "hunks": [
        {
          "old_start": 16,
          "old_lines": 4,
          "new_start": 16,
          "new_lines": 5
        }
      ]
    }
  ],
  "changed_symbols": ["service/b.go::service.ServiceB.Do"],
  "deleted_symbols": ["repository/old.go::repository.RepositoryOld.Get"],
  "removed_calls": [
    {
      "from": "service/b.go::service.ServiceB.Do",
      "to": "repository/old.go::repository.RepositoryOld.Get",
      "call_site": { "path": "service/b.go", "line": 18 }
    }
  ],
  "entry_points": ["controller/a.go::controller.ControllerA.Handle"],
  "nodes": [
    {
      "id": "service/b.go::service.ServiceB.Do",
      "path": "service/b.go",
      "name": "Do",
      "kind": "method",
      "start_line": 16,
      "end_line": 22,
      "calls": [
        {
          "to": "repository/b.go::repository.RepositoryB.Get",
          "call_site": { "path": "service/b.go", "line": 19 }
        }
      ],
      "changed": true,
      "changed_lines": [{ "path": "service/b.go", "start": 18, "end": 20 }],
      "boundary": null,
      "package": "service",
      "file": "service/b.go",
      "lineRange": { "start": 16, "end": 22 }
    }
  ]
}
```

## Scope

Diff scope:

- target-side added/modified Go lines for `changed_symbols`
- file metadata and hunk ranges for production and test files
- deleted files and deleted-only hunks included in `files`
- renamed files included as `status: "renamed"`

AST scope:

- current AST builds `changed_symbols`, `entry_points`, and `nodes`
- base AST builds `deleted_symbols` and `removed_calls`
- base ref selection:
  - staged diff: `HEAD`
  - one commit: `<commit>^`
  - range: `<base>`

Skipped for symbol/call analysis:

- test-only code: `*_test.go`, `test/`, `tests/`, `testdata/`
- formatting-only diffs by default (`--ignore-all-space`, `--ignore-blank-lines`)
- blank added lines
