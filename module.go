package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type module struct {
	Dep

	deps  []Dep
	rels  []RelativePath
	dones []<-chan struct{}

	logs bytes.Buffer
}

type result struct {
	Dep

	logs bytes.Buffer

	skipped bool
	*command

	err error
}
type command struct {
	name string
	//TODO separate err/out?
	output bytes.Buffer
}

func (m *module) newResult(err error) *result {
	return &result{Dep: m.Dep, logs: m.logs, err: err}
}

func (m *module) logf(format string, a ...any) {
	fmt.Fprintf(&m.logs, format, a...)
}

// listRelative populates m.mod and m.deps.
func (m *module) listRelative(ctx context.Context) error {
	list, err := goListAll(ctx, string(m.rel))
	if err != nil {
		return err
	}
	for _, l := range list {
		if l.Main {
			m.mod = l.Path
			//TODO but now the log is non-deterministic
			if verbose { //TODO helpers for these
				m.logf("%s/go.mod: module %s\n", m.rel, m.mod)
			}
			continue
		}
		if l.Replace != nil && l.Replace.isRelative() {
			m.deps = append(m.deps, Dep{
				rel: RelativePath(filepath.Join(string(m.rel), l.Replace.Path)),
				mod: l.Path,
			})
		}
	}
	return nil
}

// ensureDones ensures that done chans exist for this mod and its dependencies, and populated s.dones with dependencies.
func (m *module) ensureDones(getChan func(RelativePath) chan struct{}) error {
	for _, dep := range m.deps {
		rel := dep.rel
		if _, err := os.Stat(filepath.Join(string(rel), "go.mod")); err != nil {
			err = fmt.Errorf("%s go.mod not found: %w", rel, err)
			if verbose {
				m.logf("\t%s\n", err)
			}
			if force {
				//TODO log about ignoring?
				continue
			}
			return err
		}
		m.dones = append(m.dones, getChan(rel))
		if verbose {
			m.logf("\t%s => %s\n", dep.mod, rel)
		}
	}
	return nil
}

func (m *module) run(ctx context.Context, args []string) *result {
	if len(args) == 0 { //TODO or dry run (-n?)
		return m.newResult(nil)
	}
	return m.execute(ctx, args)
}

// waitForDeps blocks until all done chans are closed, unless an error result is encountered,
// in which case it will return early. In force mode, errors are ignored.
func (m *module) waitForDeps(ctx context.Context, confirmResult func(Dep) error) error {
	for i, done := range m.dones {
		select {
		case <-ctx.Done():
			return fmt.Errorf("stopped waiting for %s: %w", m.rels[i], ctx.Err())
		case <-done:
			dep := m.deps[i]
			if err := confirmResult(dep); err != nil {
				if force {
					m.logf("\tignoring: %s\n", err)
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (m *module) execute(ctx context.Context, args []string) *result {
	r := m.newResult(nil)

	var cmd *exec.Cmd
	if without {
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	} else if cmdSh {
		cmd = exec.CommandContext(ctx, "sh", "-c")
		cmd.Args = append(cmd.Args, args...)
	} else {
		cmd = exec.CommandContext(ctx, "go", "mod")
		cmd.Args = append(cmd.Args, args...)
	}
	cmd.Dir = string(m.rel)
	r.command = new(command)
	cmd.Stdout = &r.output
	cmd.Stderr = &r.output

	r.name = strings.Join(cmd.Args, " ")
	r.err = cmd.Run() //TODO why not just CombinedOutput?
	return r
}
