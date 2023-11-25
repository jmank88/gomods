package main

import (
	"context"
	"fmt"
	"io/fs"
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

	var count int
	if err := filepath.WalkDir(".", walkDirFn(ctx, func(rel RelativePath) {
		g.rels <- rel
		logf("%s/go.mod\n", rel)
		count++
	})); err != nil {
		return fmt.Errorf("failed to walk file tree: %w", err)
	}
	close(g.rels)

	logf("found %d go.mod files\n", count)

	var sorted []*result
	for r := range g.resCh {
		i := slices.IndexFunc(sorted, func(o *result) bool { return r.rel < o.rel })
		if i == -1 {
			sorted = append(sorted, r)
		} else {
			sorted = slices.Insert(sorted, i, r)
		}
	}
	for _, r := range sorted {
		logf("%s$ %s\n", r.rel, r.cmd)
		//OPT r.output.WriteTo()
		if s := r.output.String(); len(s) > 0 {
			fmt.Printf("\t")
			s = strings.ReplaceAll(s, "\n", "\n\t")
			s = strings.TrimSuffix(s, "\t")
			fmt.Print(s)
		}
		if r.err != nil {
			fmt.Printf("\terror: %s", r.err)
		}
	}
	return ctx.Err()
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
	v, ok := g.resMap.Load(rel)
	if !ok {
		return fmt.Errorf("%s is missing", rel)
	}
	res := v.(*result)
	if res.err != nil {
		return fmt.Errorf("%s error: %w", rel, res.err)
	}
	if res.mod != mod {
		return fmt.Errorf("%s: expected module %s but found %s", rel, mod, res.mod)
	}
	return nil
}

func (g *gomods) executeAll(ctx context.Context, args []string) {
	defer close(g.resCh)
	dones := make(doneMap)
	var wg sync.WaitGroup
	for rel := range g.rels {
		var m module
		m.rel = rel
		done := dones.getChan(m.rel)
		if slices.Contains(skips, string(rel)) {
			g.storeResult(m.newResult(nil))
			close(done)
			continue
		}
		if err := m.listRelative(ctx); err != nil {
			g.storeResult(m.newResult(err))
			close(done)
			continue
		}
		if verbose {
			logf("%s/go.mod: module %s\n", m.rel, m.mod)
		}

		if err := m.ensureDones(dones.getChan); err != nil {
			g.storeResult(m.newResult(err))
			close(done)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(done)

			g.storeResult(m.run(ctx, g.verifyResult, args))
		}()
	}
	wg.Wait()
}

type doneMap map[RelativePath]chan struct{}

func (m doneMap) getChan(p RelativePath) chan struct{} {
	d, ok := m[p]
	if !ok {
		d = make(chan struct{})
		m[p] = d
	}
	return d
}
