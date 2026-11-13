package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rhysd/actionlint"
)

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func main() {
	files := []string{
		"../.github/workflows/android-source.yml",
		"../.github/workflows/android-dev.yml",
	}

	for _, f := range files {
		fmt.Printf("=== %s ===\n", f)

		// 1. Читаем через os.ReadFile
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("  os.ReadFile ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  os.ReadFile: len=%d sha256=%s\n", len(data), sha256hex(data))

		// 2. Передаем те же байты в actionlint.Parse
		_, parseErrs := actionlint.Parse(data)
		if len(parseErrs) > 0 {
			for _, e := range parseErrs {
				fmt.Printf("  Parse ERROR: %s (line %d, col %d)\n", e.Message, e.Line, e.Column)
			}
		} else {
			fmt.Printf("  actionlint.Parse: OK\n")
		}

		// 3. Копируем в TMPDIR и запускаем LintFile
		tmpDir := os.TempDir()
		tmpFile := filepath.Join(tmpDir, filepath.Base(f))
		tmpData, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("  tmp read ERROR: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(tmpFile, tmpData, 0644); err != nil {
			fmt.Printf("  tmp write ERROR: %v\n", err)
			os.Exit(1)
		}

		// Проверяем, что скопированный файл идентичен
		tmpData2, err := os.ReadFile(tmpFile)
		if err != nil {
			fmt.Printf("  tmp re-read ERROR: %v\n", err)
			os.Exit(1)
		}
		if sha256hex(tmpData2) != sha256hex(data) {
			fmt.Printf("  TMP FILE MISMATCH! orig=%s tmp=%s\n", sha256hex(data), sha256hex(tmpData2))
			os.Exit(1)
		}
		fmt.Printf("  tmp file: len=%d sha256=%s (matches)\n", len(tmpData2), sha256hex(tmpData2))

		// Запускаем LintFile на оригинале с ColorOptionKindNever
		opts := &actionlint.LinterOptions{Color: actionlint.ColorOptionKindNever}
		linter, err := actionlint.NewLinter(os.Stderr, opts)
		if err != nil {
			fmt.Printf("  NewLinter ERROR: %v\n", err)
			os.Exit(1)
		}
		errsOrig, err := linter.LintFile(f, nil)
		if err != nil {
			fmt.Printf("  LintFile(orig) ERROR: %v\n", err)
		} else if len(errsOrig) > 0 {
			fmt.Printf("  LintFile(orig) errors: %d\n", len(errsOrig))
			for _, e := range errsOrig {
				fmt.Printf("    %s:%d:%d: %s [%s]\n", e.Filepath, e.Line, e.Column, e.Message, e.Kind)
			}
		} else {
			fmt.Printf("  LintFile(orig): OK (no errors)\n")
		}

		// Запускаем LintFile на копии в TMPDIR
		errsTmp, err := linter.LintFile(tmpFile, nil)
		if err != nil {
			fmt.Printf("  LintFile(tmp) ERROR: %v\n", err)
		} else if len(errsTmp) > 0 {
			fmt.Printf("  LintFile(tmp) errors: %d\n", len(errsTmp))
			for _, e := range errsTmp {
				fmt.Printf("    %s:%d:%d: %s [%s]\n", e.Filepath, e.Line, e.Column, e.Message, e.Kind)
			}
		} else {
			fmt.Printf("  LintFile(tmp): OK (no errors)\n")
		}

		fmt.Println()
	}
}
