# inktrail

Detect changed code lines from git diffs.

## Usage

```sh
go run ./cmd/inktrail                # staged changes
go run ./cmd/inktrail <commit>       # one commit
go run ./cmd/inktrail <base> <head>  # commit range
go run ./cmd/inktrail -chains        # call chains for staged/commit changes
go run ./cmd/inktrail -report        # AI-agent JSON report for staged/commit changes
```

Examples:

```sh
# Show changed lines already staged for commit
go run ./cmd/inktrail

# Show changed lines introduced by one commit
go run ./cmd/inktrail a1b2c3d

# Show changed lines between two refs
go run ./cmd/inktrail main HEAD
```

Example output:

```text
internal/server/handler.go:42:return handleRequest(ctx, req)
internal/server/handler.go:43:return nil
```

Call-chain output:

```sh
go run ./cmd/inktrail -chains
```

```text
controller.ControllerA.Handle -> service.ServiceA.Do -> service.ServiceB.Do -> repository.RepositoryB.Get
```

AI-agent report output:

```sh
go run ./cmd/inktrail -report
```

```json
{
  "changed_symbols": ["service/b.go::service.ServiceB.Do"],
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
          "call_site": { "path": "service/b.go", "line": 18 }
        }
      ],
      "changed": true,
      "changed_lines": [{ "path": "service/b.go", "line": 18 }],
      "boundary": null,
      "package": "service",
      "file": "service/b.go",
      "lineRange": { "start": 16, "end": 22 }
    }
  ]
}
```

Current scope: only target-side added/modified lines. Deleted lines and unchanged context are ignored.

Formatting-only changes are skipped by default:

- whitespace-only changes
- blank-line changes
- line-break-only formatting that `git diff --ignore-all-space --ignore-blank-lines` can ignore

Test-only code is skipped by default:

- `*_test.go`
- files under `test/`, `tests/`, or `testdata/`
