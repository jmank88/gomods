package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

func isRelativePath(path string) bool { return strings.HasPrefix(path, ".") }

func modFile(dir string) (*modfile.File, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod file: %w", err)
	}
	f, err := modfile.Parse("go.mod", b, func(path, vers string) (string, error) {
		return "v0.0.1", nil // fake valid version to tolerate un-tidy files
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod file: %w", err)
	}
	return f, nil
}
