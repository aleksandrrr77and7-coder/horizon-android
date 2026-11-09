// actionlint-check — Voxelix workflow linter via direct Go API.
//
// This tool checks GitHub Actions workflow YAML files using actionlint's
// Go API (Linter.LintFile()) instead of the CLI binary.
//
// Rationale: on Android/Termux the actionlint CLI binary is broken because
// os.Args contains a duplicate of the binary's absolute path as args[1],
// which shifts real file arguments and causes the CLI to try parsing its
// own ELF binary as YAML:
//   "could not parse as YAML: control characters are not allowed [syntax-check]"
//
// The direct Go API avoids this problem entirely.
//
// Key design decisions:
//   - Linter is created via NewLinter(out, &actionlint.LinterOptions{})
//     (never nil opts — that panics on actionlint v1.7.12).
//   - ColorOptionKindNever is set to avoid ANSI color issues on Android/Termux.
//   - Lint errors are returned in []*Error, not through the error return value.
//     Both err != nil AND len(errs) > 0 must be checked.
//   - Exit code: 0 = no lint problems, 1 = problems found or fatal error.
//
// Usage:
//
//	go run ./tools/actionlint-check [file1.yml] [file2.yml] ...
//
// With no arguments, lints the default Voxelix workflow files:
//   .github/workflows/android-source.yml
//   .github/workflows/android-dev.yml
//
// This tool is NOT connected to the main Voxelix build. It is a standalone
// diagnostic/checking utility. No source files of actionlint or Voxelix
// are modified.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rhysd/actionlint"
)

// defaultFiles are the Voxelix workflow files checked when no arguments are given.
var defaultFiles = []string{
	".github/workflows/android-source.yml",
	".github/workflows/android-dev.yml",
}

// lintFiles runs actionlint.LintFile() on each path and reports results.
// Returns overall exit code: 0 if clean, 1 if any errors found.
func lintFiles(l *actionlint.Linter, paths []string) int {
	overallExit := 0

	for _, path := range paths {
		fmt.Printf("=== %s ===\n", path)

		errs, err := l.LintFile(path, nil)

		if err != nil {
			fmt.Printf("  FATAL Go error: %v\n", err)
			overallExit = 1
			continue
		}

		if len(errs) > 0 {
			fmt.Printf("  %d lint error(s) found\n", len(errs))
			for i, e := range errs {
				fmt.Printf("  err[%d]: %s:%d:%d: %s [%s]\n",
					i, e.Filepath, e.Line, e.Column, e.Message, e.Kind)
			}
			overallExit = 1
		} else {
			fmt.Printf("  OK — no lint errors\n")
		}
		fmt.Println()
	}

	return overallExit
}

func main() {
	// Determine files to lint: command-line arguments, or defaults.
	// On Android/Termux, os.Args contains a duplicate of the binary's
	// absolute path as args[1]. cleanArgs removes it so real file
	// arguments are not shifted.
	paths := cleanArgs(os.Args)[1:]
	if len(paths) == 0 {
		paths = defaultFiles
	}

	// Create Linter with non-nil options. ColorOptionKindNever avoids
	// ANSI escape sequences that can corrupt output on Android/Termux.
	opts := &actionlint.LinterOptions{
		Color: actionlint.ColorOptionKindNever,
	}

	l, err := actionlint.NewLinter(os.Stdout, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: NewLinter: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Voxelix v1.0 — actionlint-check (Go API, v1.7.12)")
	fmt.Printf("Platform: %s/%s\n\n", goOS(), goArch())

	exitCode := lintFiles(l, paths)

	fmt.Println("============================================================")
	fmt.Printf("OVERALL EXIT CODE: %d\n", exitCode)
	if exitCode == 0 {
		fmt.Println("RESULT: all files passed lint check")
	} else {
		fmt.Println("RESULT: lint errors detected — see above")
	}
	fmt.Println("============================================================")

	os.Exit(exitCode)
}

// goOS and goArch are small helpers to report the platform without importing
// runtime in the main flow (kept simple for readability).
func goOS() string {
	switch os.Getenv("GOOS") {
	case "":
		return "unknown"
	default:
		return os.Getenv("GOOS")
	}
}

func goArch() string {
	switch os.Getenv("GOARCH") {
	case "":
		return "unknown"
	default:
		return os.Getenv("GOARCH")
	}
}

// cleanArgs removes duplicate entries from os.Args that point to the same
// executable as the current process. On Android/Termux, os.Args[1] is the
// absolute path to the binary itself, which shifts real file arguments.
// We resolve args[0] to an absolute path and skip any subsequent arg that
// resolves to the same path.
func cleanArgs(args []string) []string {
	if len(args) <= 1 {
		return args
	}

	progAbs, err := filepath.Abs(args[0])
	if err != nil {
		progAbs = args[0]
	}

	clean := make([]string, 0, len(args))
	clean = append(clean, args[0])

	seen := make(map[string]bool)
	seen[progAbs] = true

	for _, a := range args[1:] {
		abs, err := filepath.Abs(a)
		if err != nil {
			clean = append(clean, a)
			continue
		}
		if abs == progAbs || seen[abs] {
			continue
		}
		seen[abs] = true
		clean = append(clean, a)
	}

	return clean
}
