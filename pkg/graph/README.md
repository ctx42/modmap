# graph

Package `graph` turns discovered Go modules into a layered dependency graph.

Every module lands one level above the modules it depends on, so level zero
holds the modules without dependencies and the levels read as the order in
which a change has to be propagated. Each node also carries the modules
reaching it, directly or through others: the set which has to be updated when
that module changes. A dependency circle is an error, reported with the path
around it.

```shell
go get github.com/ctx42/modmap/pkg/graph
```

## Usage

```go
grp, err := graph.New(mods) // map[string]*mod.Module
if err != nil {
    return err // graph.CycleError when the modules depend on each other.
}

for level, nodes := range grp.Levels {
    for _, nod := range nodes {
        fmt.Println(level, nod.Path, nod.Dependents)
    }
}
```
