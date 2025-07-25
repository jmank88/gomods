package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// RelativePath holds a filepath to a module, relative to the working dir.
type RelativePath string

// ModulePath is the module path.
type ModulePath string

type Dep struct {
	rel RelativePath
	mod ModulePath
}

type gomods struct {
	rels   chan RelativePath
	resCh  chan *result
	resMap sync.Map // map[FullPath]*result
}

func newMods() *gomods {
	// TODO larger chan buffers?
	return &gomods{
		rels:  make(chan RelativePath, 10),
		resCh: make(chan *result, 10),
	}
}

func (g *gomods) run(ctx context.Context, args []string) error {
	go g.executeAll(ctx, args)

	var rels []RelativePath
	if err := filepath.WalkDir(".", walkDirFn(ctx, func(rel RelativePath) {
		g.rels <- rel
		rels = append(rels, rel)
	})); err != nil {
		return fmt.Errorf("failed to walk file tree: %w", err)
	}
	close(g.rels)

	slices.Sort(rels)
	logf("found %d go.mod files:\n", len(rels))
	for _, rel := range rels {
		logf("\t%s/go.mod\n", rel)
	}

	var sorted []*result
	for r := range g.resCh {
		i := slices.IndexFunc(sorted, func(o *result) bool { return r.rel < o.rel })
		if i == -1 {
			sorted = append(sorted, r)
		} else {
			sorted = slices.Insert(sorted, i, r)
		}
	}
	var errs error
	for _, r := range sorted {
		if !liveLogs {
			//TODO prefix with rel?
			_, err := r.logs.WriteTo(os.Stderr)
			if err != nil {
				return fmt.Errorf("failed to write logs for %s: %w", r.rel, err)
			}
		}
		if r.command != nil {
			logf("%s$ %s\n", r.rel, r.name)
			if !liveLogs {
				//OPT r.output.WriteTo() w/ prefixWriter
				if s := r.output.String(); len(s) > 0 {
					fmt.Printf("\t")
					s = strings.ReplaceAll(s, "\n", "\n\t")
					s = strings.TrimSuffix(s, "\t")
					fmt.Print(s)
				}
			}
		} else if r.skipped && verbose {
			logf("%s (skipped)\n", r.rel)
		}
		if r.err != nil {
			fmt.Printf("\terror: %s\n", r.err)
			errs = errors.Join(errs, fmt.Errorf("%s: %w", r.rel, r.err))
		}
	}
	return errs
}

// walkDirFn call fn for each RelativePath with a go.mod file.
func walkDirFn(ctx context.Context, fn func(RelativePath)) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "go.mod" {
			return nil
		}
		fn(RelativePath(filepath.Dir(path)))
		return nil
	}
}

func (g *gomods) storeResult(r *result) {
	g.resCh <- r
	g.resMap.Store(r.rel, r)
}

// verifyResult returns an error if a result for rel cannot be found, contains an error, or has an unexpected ModulePath.
func (g *gomods) verifyResult(dep Dep) error {
	rel, mod := dep.rel, dep.mod
	if !filepath.IsLocal(string(rel)) {
		return nil // nothing to check
	}
	v, ok := g.resMap.Load(rel)
	if !ok {
		return fmt.Errorf("%s is missing", rel)
	}
	res := v.(*result)
	if res.err != nil {
		return fmt.Errorf("%s error: %w", rel, res.err)
	}
	if res.skipped {
		return nil // nothing to check since we didn't parse it
	}
	if res.mod != mod {
		return fmt.Errorf("%s: expected module %q but found %q", rel, mod, res.mod)
	}
	return nil
}

func (g *gomods) executeAll(ctx context.Context, args []string) {
	defer close(g.resCh)
	dones := newDoneChans()
	var wg sync.WaitGroup
	for rel := range g.rels {
		var m module
		if !liveLogs {
			m.logs = &bytes.Buffer{}
		}
		m.rel = rel
		var done func()
		if !unordered {
			done = func() { close(dones.getChan(m.rel)) }
		}
		if slices.ContainsFunc(skips, func(pre string) bool {
			return strings.HasPrefix(string(rel), pre)
		}) {
			r := m.newResult(nil)
			r.skipped = true
			g.storeResult(r)
			done()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer done()

			if unordered {
				g.storeResult(m.run(ctx, args))
				return
			}

			g.storeResult(func() *result {
				if err := m.listRelative(); err != nil {
					return m.newResult(err)
				}
				if err := m.ensureDones(dones.getChan); err != nil {
					return m.newResult(err)
				}
				if err := m.waitForDeps(ctx, g.verifyResult); err != nil {
					return m.newResult(err)
				}
				return m.run(ctx, args)
			}())
		}()
	}
	wg.Wait()
}

type doneChans struct {
	mu    sync.Mutex
	chans map[RelativePath]chan struct{}
}

func newDoneChans() *doneChans {
	return &doneChans{chans: map[RelativePath]chan struct{}{}}
}

func (d *doneChans) getChan(p RelativePath) chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	dc, ok := d.chans[p]
	if !ok {
		dc = make(chan struct{})
		if !filepath.IsLocal(string(p)) {
			close(dc)
		}
		d.chans[p] = dc
	}
	return dc
}
