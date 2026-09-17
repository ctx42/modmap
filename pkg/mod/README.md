# mod

Package `mod` discovers Go modules on disk and resolves what they require.

It walks directory trees collecting every `go.mod` file, reads the direct
requirements from each one, and expands the rest of the closure from the Go
module cache, reaching the network only when the cache does not hold a
`go.mod` yet. Module path globs decide what takes part, and they are applied
during discovery, so an excluded module is never fetched.

```shell
go get github.com/ctx42/modmap/pkg/mod
```

## Usage

```go
flt := mod.NewFilter([]string{"github.com/ctx42/*"}, nil)

mods, err := mod.NewScanner(flt, nil).Scan([]string{"/home/user/src"})
if err != nil {
    return err
}

rsv, err := mod.NewResolver(flt, os.Environ(), nil)
if err != nil {
    return err
}
defer func() { _ = rsv.Close() }()

if err = rsv.Resolve(context.Background(), mods); err != nil {
    return err
}

for path, module := range mods {
    fmt.Println(path, module.Deps())
}
```

Pass a `func(format string, args ...any)` as the last argument to either
constructor to follow the progress; a nil one discards the messages.
