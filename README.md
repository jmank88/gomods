# gomods

CLI tool to execute a command on every go module in the tree

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
