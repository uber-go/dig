# dig: Dependency Injection Framework for Go

[![GoDoc][doc-img]][doc]
[![Coverage Status][cov-img]][cov]
[![Build Status][ci-img]][ci]
[![Report Card][report-card-img]][report-card]

`package dig` provides an opinionated way of resolving object dependencies.

## Status

BETA. Expect potential API changes.

## Container

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

**All objects in the container are treated as a singletons**, meaning there can be
only one object in the graph of a given type.

There are plans to expand the API to support multiple objects of the same type
in the container, but for time being consider using a factory pattern.

## Provide

The `Provide` method adds an object, or a constructor of an object, to the container.

### Provide a constructor

A constructor can be a function returning any number of objects and, optionally,
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

Here is a fully working somewhat real-world `Invoke` example:

```go
package main

import (
	"go.uber.org/config"
	"go.uber.org/dig"
	"go.uber.org/zap"
)

func main() {
	c := dig.New()

	// Provide configuration object
	c.Provide(func() config.Provider {
		return config.NewYAMLProviderFromBytes([]byte("tag: Hello, world!"))
	})

	// Provide a zap logger which relies on configuration
	c.Provide(func(cfg config.Provider) (*zap.Logger, error) {
		l, err := zap.NewDevelopment()
		if err != nil {
			return nil, err
		}
		return l.With(zap.String("iconic phrase", cfg.Get("tag").AsString())), nil
	})

	// Invoke a function that requires a zap logger, which in turn requires config
	c.Invoke(func(l *zap.Logger) {
		l.Info("You've been invoked")
		// Logger output:
		//     INFO    You've been invoked     {"iconic phrase": "Hello, world!"}
		//
		// As we can see, Invoke caused the Logger to be created, which in turn
		// required the configuration to be created.
	})
}
```

[doc]: https://godoc.org/go.uber.org/dig
[doc-img]: https://godoc.org/go.uber.org/dig?status.svg
[cov]: https://coveralls.io/github/uber-go/dig?branch=master
[cov-img]: https://coveralls.io/repos/github/uber-go/dig/badge.svg?branch=master
[ci]: https://travis-ci.org/uber-go/dig
[ci-img]: https://travis-ci.org/uber-go/dig.svg?branch=master
[report-card]: https://goreportcard.com/report/github.com/uber-go/dig
[report-card-img]: https://goreportcard.com/badge/github.com/uber-go/dig
