# dig: Dependency Injection Framework for Go

[![GoDoc][doc-img]][doc]
[![Coverage Status][cov-img]][cov]
[![Build Status][ci-img]][ci]
[![Report Card][report-card-img]][report-card]

`package dig` provides an opinionated way of resolving object dependencies.

For runnable examples, see the [examples directory](examples/).

## Status

BETA. Expect potential API changes.

## Provide

`Provide` adds an object, or a constructor of an object to the container.

There are two ways to Provide an object:

1. Provide a pointer to an existing object
1. Provide a slice, map, array
1. Provide a "constructor function" that returns one pointer (or interface)

### Provide a Constructor

Constructor is defined as a function that returns one pointer (or
interface), an optional error, and takes 0-N number of arguments.

Each one of the arguments is automatically registered as a **dependency**
and must also be an interface or a pointer.

```go
type Type1 struct {}
type Type2 struct {}
type Type3 struct {}

c := dig.New()
err := c.Provide(func(*Type1, *Type2) *Type3 {
	// operate on Type1 and Type2 to make Type3 and return
})
// dig container is now able to provide *Type3 to any constructor.
// However, note that in the current examples *Type1 and *Type2
// have not been provided. Constructors (or instances) first
// have to be provided to the dig container before it is able
// to create a shared singleton instance of *Type3
```

### Provide an Object

Registering an object directly is a shortcut to register something that
has no dependencies.

```go
type Type1 struct {
	Name string
}

c := dig.New()
err := c.Provide(&Type1{Name: "I am a thing"})
// dig container is now able to provide *Type1 as a dependency
// to other constructors that require it.
```


### Providing Maps and Slices

Dig also support maps, slices and arrays as objects to
resolve, or provided as a dependency to the constructor.

```go
c := dig.New()
var (
	typemap = map[string]int{}
	typeslice = []int{1, 2, 3}
	typearray = [2]string{"one", "two"}
)
err := c.Provide(typemap, typeslice, typearray)

var resolveslice []int
err := c.Resolve(&resolveslice)

c.Invoke(func(map map[string]int, slice []int, array [2]string) {
	// do something
})
```

## Resolve

`Resolve` retrieves objects from the container by building the object graph.

Object is resolution is based on the type of the variable passed into `Resolve`
function.

For example, in the current scenario:

```go
// c := dig.New()
// c.Provide...
var cfg config.Provider
c.Resolve(&cfg) // note pointer to interface
```

dig will look through the dependency graph and identify if there was a constructor
registered that is able to return a type of `config.Provider`. It will then check
if said constructor requires any dependencies (by analyzing the function parameters).
If it does, it will recursively complete resolutions of the parameters in the graph
until the constructor for `config.Provider` can be fully satisfied.

If resolution is not possible, for instance one of the required dependencies has
does not have a constructor and doesn't appear in the graph, an error will be returned.

[doc]: https://godoc.org/go.uber.org/dig
[doc-img]: https://godoc.org/go.uber.org/dig?status.svg
[cov]: https://coveralls.io/github/uber-go/dig?branch=master
[cov-img]: https://coveralls.io/repos/github/uber-go/dig/badge.svg?branch=master
[ci]: https://travis-ci.org/uber-go/dig
[ci-img]: https://travis-ci.org/uber-go/dig.svg?branch=master
[report-card]: https://goreportcard.com/report/github.com/uber-go/dig
[report-card-img]: https://goreportcard.com/badge/github.com/uber-go/dig
