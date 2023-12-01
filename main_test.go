package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{"gomods": Main}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:             "testdata",
		ContinueOnError: true,
		//UpdateScripts:   true,
	})
}
