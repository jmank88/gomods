# gomods

CLI tool to execute a command on every go module in the tree

```
$ gomods --help
Usage of gomods:
  -c    command: command string execution with 'sh -c' prefix
  -f    force: continue execution even if dependencies failed
  -s string
        skip: comma separated list of paths to skip
  -u    unordered: execute without waiting for dependencies
  -v    verbose: detailed logs
  -w    without: without 'go mod' prefix
```

## w/ go mod
```shell
$ gomods download
$ gomods tidy
$ gomods list
$ gomods graph
```

## w/o go mod
```shell
$ gomods -w go generate ./...
$ gomods -w go test ./...
$ gomods -w cat go.mod
```
