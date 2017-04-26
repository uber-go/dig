# dig: Dependency Injection Framework for Go

`package dig` provides an opinionated way of resolving object dependencies.

For runnable examples, see the [examples directory](examples/).

## Status

BETA. Expect potential API changes.

## Provide

`Provide` adds an object, or a constructor of an object to the container.

There are two ways to Provide an object:

1. Provide a pointer to an existing object
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
err := c.Provide(&Type1{Name: "I am an thing"})
// dig container is now able to provide *Type1 as a dependency
// to other constructors that require it.
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

There are future plans to do named retrievals to support multiple
objects of the same type in the container.

```go
type Type1 struct {
	Name string
}

c := dig.New()
err := c.Provide(&Type1{Name: "I am an thing"})
// dig container is now able to provide *Type1 as a dependency
// to other constructors that require it.
```

## Error Handling and Alternatives

`Provide` and `Resolve` (and their `ProvideAll` and `ResolveAll` counterparts)
return errors, and usages of `dig` should fully utilize the error checking, since
container creation is done through reflection meaning a lot of the errors surface
at runtime.

There are, however, `Must*` alternatives of all the methods available. They
are drawn from some of the patterns in the Go standard library and are there
to simplify usage in critical scenarios: where not being able to resolve an
object is not an option and panic is preferred.
