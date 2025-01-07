package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type module struct {
	Dep

	deps  []Dep
	dones []<-chan struct{}

	logs *bytes.Buffer // optional
}

type result struct {
	Dep

	logs *bytes.Buffer // optional

	skipped bool
	*command

	err error
}
type command struct {
	name string
	//TODO separate err/out?
	output bytes.Buffer // nil if liveLogs
}

func (m *module) prefix() string {
	return fmt.Sprintf("[%s] ", m.rel)
}

func (m *module) newResult(err error) *result {
	return &result{Dep: m.Dep, logs: m.logs, err: err}
}

func (m *module) logf(format string, a ...any) {
	if m.logs != nil {
		fmt.Fprintf(m.logs, format, a...)
		return
	}
	logf(m.prefix()+format, a...)
}

// listRelative populates m.mod and m.deps.
func (m *module) listRelative() error {
	mf, err := modFile(string(m.rel))
	if err != nil {
		return err
	}
	if mf.Module != nil {
		m.mod = ModulePath(mf.Module.Mod.Path)
		if verbose {
			m.logf("module %s\n", m.mod)
		}
	}
	for _, l := range mf.Replace {
		if isRelativePath(l.New.Path) {
			m.deps = append(m.deps, Dep{
				rel: RelativePath(filepath.Join(string(m.rel), l.New.Path)),
				mod: ModulePath(l.Old.Path),
			})
		}
	}
	return nil
}

// ensureDones ensures that done chans exist for this mod and its dependencies, and populated s.dones with dependencies.
func (m *module) ensureDones(getChan func(RelativePath) chan struct{}) error {
	for _, dep := range m.deps {
		rel := dep.rel
		if rel == m.rel {
			continue // ignore self-references
		}
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
			return fmt.Errorf("stopped waiting for deps of %s: %w", m.rel, ctx.Err())
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
	} else if goCmd {
		cmd = exec.CommandContext(ctx, "go", args...)
	} else if cmdSh {
		cmd = exec.CommandContext(ctx, "sh", "-c")
		cmd.Args = append(cmd.Args, args...)
	} else {
		cmd = exec.CommandContext(ctx, "go", "mod")
		cmd.Args = append(cmd.Args, args...)
	}
	cmd.Dir = string(m.rel)
	r.command = new(command)
	if !liveLogs {
		cmd.Stdout = &r.output
		cmd.Stderr = &r.output
	} else {
		//TODO separate err/out prefixes?
		p := &prefixer{prefix: m.prefix(), w: os.Stderr}
		cmd.Stdout = p
		cmd.Stderr = p
	}

	r.name = strings.Join(cmd.Args, " ")
	r.err = cmd.Run() //TODO why not just CombinedOutput?
	return r
}

type prefixer struct {
	w             io.Writer
	prefix        string
	buf           bytes.Buffer
	newLineSuffix bool
}

func (p *prefixer) Write(bs []byte) (int, error) {
	p.buf.Reset()
	if p.newLineSuffix {
		p.buf.WriteString(p.prefix)
		p.newLineSuffix = false
	}
	for i, b := range bs {
		p.buf.WriteByte(b)
		if b == '\n' {
			if i == len(bs)-1 {
				p.newLineSuffix = true
			} else {
				p.buf.WriteString(p.prefix)
			}
		}
	}
	n64, err := p.buf.WriteTo(p.w)
	if err != nil {
		n := max(int(n64), len(bs))
		return n, err
	}
	return len(bs), nil
}
