package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	jsonexp "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// goListAll executes `go list -m -json all` in dir.
func goListAll(ctx context.Context, dir string) (ms []Module, err error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			err = fmt.Errorf("%w: %s", err, exitErr.Stderr)
		}
		err = fmt.Errorf("failed to list modules: %w", err)
		return
	}
	d := jsontext.NewDecoder(bytes.NewReader(b))
	for ctx.Err() == nil {
		var m Module
		if err = jsonexp.UnmarshalDecode(d, &m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			err = fmt.Errorf("failed to decode json: %w", err)
			return
		}
		ms = append(ms, m)
	}
	err = ctx.Err()
	return
}

type Module struct {
	Path    ModulePath
	Replace *Replaced
	Main    bool
}

type Replaced struct {
	Path string
}

func (r Replaced) isRelative() bool { return strings.HasPrefix(r.Path, ".") }
