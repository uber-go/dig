# dig: Dependency Injection Framework for Go

[![GoDoc][doc-img]][doc]
[![Coverage Status][cov-img]][cov]
[![Build Status][ci-img]][ci]
[![Report Card][report-card-img]][report-card]

`package dig` provides an opinionated way of resolving object dependencies.

## Status

BETA. Expect potential API changes.

## Conatiner

`package dig` exposes `type Container` as an object capable of resolving a
directional dependency graph.

To create one:
```go
import "go.uber.org/dig"

func main() {
	c := dig.New()
	// dig container `c` is ready to use!
}
```

Objects in the container are identified by their `reflect.Type` and **everything
is treated as a singleton**, meaning there can be only one object in the graph
of a given type.

There are plans to expand the API to support multiple objects of the same type
in the container, but for time being consider using a factory pattern.

## Provide

The `Provide` method adds an object, or a constructor of an object, to the container.

### Provide a constructor

Constructor can be a function returning any number of objects and, optionally,
an error.

Each argument to the constructor is registered as a **dependency** in the graph.

```go
type A struct {}
type B struct {}
type C struct {}

c := dig.New()
constructor := func (*A, *B) *C {
  // At this point, *A and *B have been resolved through the graph
  // and can be used to provide an object of type *C
  return &C{}
}
err := c.Provide(constructor)
// dig container is now able to provide *C to any constructor.
//
// However, note that in the current example *A and *B
// have not been provided, meaning that the resolution of type
// *C will result in an error, because types *A and *B can not
// be instantiated.
```

### Provide an object

As a shortcut for any object without dependencies, register it directly.

```go
type A struct {
	Name string
}

c := dig.New()
err := c.Provide(&A{Name: "Hello, dig!"})
```

## Invoke

The `Invoke` API is the flip side of `Provide` and used to retrieve types from the container.

`Invoke` looks through the graph and resolves all the constructor parameters for execution.

In order to successfully use use `Invoke`, the function must meet the following criteria:

1. Input to the `Invoke` must be a function
1. All arguments to the function must be types in the container
1. If an error is returned from an invoked function, it will be propagated to the caller

```go
// c := dig.New()
// c.Provide config.Provider, zap.Logger, etc
err := c.Invoke(func(cfg *config.Provider, l *zap.Logger) error {
	// function body
	return nil
})
```

[doc]: https://godoc.org/go.uber.org/dig
[doc-img]: https://godoc.org/go.uber.org/dig?status.svg
[cov]: https://coveralls.io/github/uber-go/dig?branch=master
[cov-img]: https://coveralls.io/repos/github/uber-go/dig/badge.svg?branch=master
[ci]: https://travis-ci.org/uber-go/dig
[ci-img]: https://travis-ci.org/uber-go/dig.svg?branch=master
[report-card]: https://goreportcard.com/report/github.com/uber-go/dig
[report-card-img]: https://goreportcard.com/badge/github.com/uber-go/dig
