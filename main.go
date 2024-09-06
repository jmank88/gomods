package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
)

var (
	cmdSh     bool
	force     bool
	goCmd     bool
	liveLogs  bool
	skips     []string
	unordered bool
	verbose   bool
	without   bool
)

func initFlags() {
	flag.BoolVar(&cmdSh, "c", false, "command: command string execution with 'sh -c' prefix")
	flag.BoolVar(&force, "f", false, "force: continue execution even if dependencies failed")
	flag.BoolVar(&goCmd, "go", false, "go: execute with 'go' prefix")
	flag.BoolVar(&liveLogs, "live", false, "live: enable live logging")
	//TODO -q (quiet)
	skip := flag.String("s", "", "skip: comma separated list of paths to skip")
	//TODO -p to limit parallelism?
	flag.BoolVar(&unordered, "u", false, "unordered: execute without waiting for dependencies")
	flag.BoolVar(&verbose, "v", false, "verbose: detailed logs")
	flag.BoolVar(&without, "w", false, "without: without 'go mod' prefix")
	flag.Parse()
	skips = strings.Split(*skip, ",")
	if countBools(cmdSh, goCmd, without) > 1 {
		logln("Invalid flags: only one of -c, -go, or -w may be used at a time")
		os.Exit(1)
	}
}

func countBools(flags ...bool) (count int) {
	for _, f := range flags {
		if f {
			count++
		}
	}
	return count
}

func main() { os.Exit(Main()) }

func Main() (exitCode int) {
	initFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		logln("\nCancelling... interrupt again to exit")
		stop() // restore default exit behavior
	}()

	if err := newMods().run(ctx, flag.Args()); err != nil {
		logln("error:", err)
		return 1
	}
	return 0
}

func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func logln(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
}
