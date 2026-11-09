# actionlint-check — Voxelix workflow linter

Standalone Go tool that checks GitHub Actions workflow YAML files using
[actionlint v1.7.12](https://github.com/rhysd/actionlint) through the direct
Go API (`Linter.LintFile()`), bypassing the CLI binary.

## Why Go API instead of CLI

On Android/Termux, the actionlint CLI binary is broken because `os.Args`
contains a duplicate of the binary's absolute path as `args[1]`. This shifts
real file arguments and causes the CLI to try parsing its own ELF binary as
YAML:

```
could not parse as YAML: control characters are not allowed [syntax-check]
```

The direct Go API (`Linter.LintFile()`) avoids this problem entirely.

## Project structure

```
tools/actionlint-check/
├── main.go       # Tool entry point
├── go.mod        # Go module definition
├── go.sum        # Dependency checksums
├── README.md     # This file
└── testdata/     # Test YAML files
    ├── broken.yml   # Intentionally broken YAML (for error-detection tests)
```

## Usage

### From the Voxelix project root

```bash
# Lint default workflow files (android-source.yml, android-dev.yml)
go run ./tools/actionlint-check

# Lint specific files
go run ./tools/actionlint-check .github/workflows/android-source.yml .github/workflows/android-dev.yml

# Lint from another directory (absolute paths)
go run ./tools/actionlint-check /path/to/workflow.yml
```

### Build a standalone binary

```bash
cd tools/actionlint-check
go build -o actionlint-check .
./actionlint-check .github/workflows/android-source.yml .github/workflows/android-dev.yml
```

### On Android/Termux

```bash
cd /path/to/voxelix/horizon-android
go run ./tools/actionlint-check .github/workflows/android-source.yml .github/workflows/android-dev.yml
```

Or build and run:

```bash
cd tools/actionlint-check
go build -o actionlint-check .
./actionlint-check .github/workflows/android-source.yml .github/workflows/android-dev.yml
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0    | All files passed lint check |
| 1    | Lint errors found or fatal error occurred |

## Key implementation details

### Linter creation

```go
opts := &actionlint.LinterOptions{
    Color: actionlint.ColorOptionKindNever,
}
l, err := actionlint.NewLinter(os.Stdout, opts)
```

**Never pass `nil` as options** — `NewLinter(out, nil)` panics with a null
pointer dereference on `opts.Verbose` in actionlint v1.7.12.

### Error handling

`LintFile()` returns errors in the `[]*Error` slice, not through the Go
`error` return value. Both must be checked:

```go
errs, err := l.LintFile(path, nil)
if err != nil {
    // Fatal error (e.g., file not found)
}
if len(errs) > 0 {
    // Lint errors — errs contains the details
}
```

### Android/Termux os.Args fix

On Android/Termux, `os.Args[1]` is the absolute path to the binary itself.
The `cleanArgs()` function removes this duplicate before processing file
arguments, preventing the tool from trying to lint its own executable.

## Test results (Android/Termux)

| File | Exit code | Errors |
|------|-----------|--------|
| `.github/workflows/android-source.yml` | 0 | 0 |
| `.github/workflows/android-dev.yml` | 0 | 0 |
| `minimal.yml` | 0 | 0 |
| `testdata/broken.yml` | 1 | 1 |

## Integration status

This tool is **NOT connected** to the main Voxelix build or CI. It is a
standalone diagnostic/checking utility. To integrate:

1. Add a call to this tool in CI workflows or build scripts
2. Use the exit code to determine success/failure
3. No changes to existing workflow files are required

## License

Same as Voxelix v1.0 project.
