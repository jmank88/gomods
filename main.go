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
	cmdSh   bool
	force   bool
	skip    string
	skips   []string
	verbose bool
	without bool
)

func initFlags() {
	flag.BoolVar(&cmdSh, "c", false, "command: command string execution with 'sh -c' prefix")
	flag.BoolVar(&force, "f", false, "force: continue execution even if dependencies failed")
	//TODO -q (quiet)
	flag.StringVar(&skip, "s", "", "skip: comma separated list of paths to skip")
	flag.BoolVar(&verbose, "v", false, "verbose: detailed logs")
	flag.BoolVar(&without, "w", false, "without: without 'go mod' prefix")
	flag.Parse()
	skips = strings.Split(skip, ",")
	//TODO c & w are mutually exclusive
}

func main() { os.Exit(Main()) }

func Main() (exitCode int) {
	initFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	//defer stop()
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
