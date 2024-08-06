package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{"gomods": Main}))
}

func TestScripts(t *testing.T) {
	out, err := exec.Command("go", "env", "-json").Output()
	if err != nil {
		t.Fatalf("failed to determine environment from go command: %v\n%v", err, out)
	}
	var goEnv map[string]string
	if err := json.Unmarshal(out, &goEnv); err != nil {
		t.Fatalf("failed to unmarshal GOROOT and GOCACHE tags from go command out: %v\n%v", err, out)
	}
	testscript.Run(t, testscript.Params{
		Dir:             "testdata",
		ContinueOnError: true,
		Setup: func(env *testscript.Env) error {
			env.Setenv("GOPATH", goEnv["GOPATH"])
			env.Setenv("GOCACHE", goEnv["GOCACHE"])
			env.Setenv("GOMODCACHE", goEnv["GOMODCACHE"])
			return nil
		},
		//UpdateScripts:   true,
	})
}
