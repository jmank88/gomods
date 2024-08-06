package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	mod "golang.org/x/mod/module"
)

func isRelativePath(path string) bool { return strings.HasPrefix(path, ".") }

func modFile(dir string) (*modfile.File, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod file: %w", err)
	}
	f, err := modfile.Parse("go.mod", b, fixVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod file: %w", err)
	}
	return f, nil
}

// fake valid version to tolerate un-tidy files
func fixVersion(path, vers string) (string, error) {
	_, pathMajor, pathMajorOk := mod.SplitPathVersion(path)
	if vers == "" || vers != mod.CanonicalVersion(vers) {
		if pathMajor == "" {
			return "v0.0.1", nil
		}
		return fmt.Sprintf("%s.0.2", mod.PathMajorPrefix(pathMajor)), nil
	}
	if pathMajorOk {
		if err := mod.CheckPathMajor(vers, pathMajor); err != nil {
			if pathMajor == "" {
				return vers + "+incompatible", nil
			}
			return fmt.Sprintf("%s.0.3", mod.PathMajorPrefix(pathMajor)), nil
		}
	}
	return vers, nil
}
