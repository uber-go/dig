// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package dig is the dig: Dependency Injection Framework for Go.
//
// package dig provides an opinionated way of resolving object dependencies.
//
// For runnable examples, see the examples directory (examples/).
//
// Status
//
// BETA. Expect potential API changes.
//
// Conatiner
//
// package dig exposes type Container as an object capable of resolving a
// directional dependency graph.
//
//
// To create one:
//
//   import "go.uber.org/dig"
//
//   func main() {
//   	c := dig.New()
//   	// dig container `c` is ready to use!
//   }
//
// Objects in the container are identified by their reflect.Type and **everything
// is treated as a singleton
// **, meaning there can be only one object in the graph
// of a given type.
//
//
// For more advanced use cases, consider using a factory pattern. That is,
// have one object shared as a singleton, capable of creating many instances
// of the same type on demand.
//
//
// Provide
//
// The Provide method adds an object, or a constructor of an object, to the container.
//
// There are several ways to Provide an object:
//
// • Provide a constructor function that returns one pointer (or interface)
//
// • Provide a pointer to an existing object
//
// • Provide a slice, map, or array
//
// Provide a Constructor
//
// A constructor is defined as a function that returns one pointer (or
// interface), returns an optional error, and takes 0-N number of arguments.
//
//
// Each one of the arguments is automatically registered as a **dependency**and must also be an interface or a pointer.
//
//
//   type Type1 struct {}
//   type Type2 struct {}
//   type Type3 struct {}
//
//   c := dig.New()
//   err := c.Provide(func(*Type1, *Type2) *Type3 {
//   	// operate on Type1 and Type2 to make Type3 and return
//   })
//   // dig container is now able to provide *Type3 to any constructor.
//   // However, note that in the current examples *Type1 and *Type2
//   // have not been provided. Constructors (or instances) first
//   // have to be provided to the dig container before it is able
//   // to create a shared singleton instance of *Type3
//
// Provide an object
//
// As a shortcut for objects without dependencies, register it directly.
//
//   type Type1 struct {
//   	Name string
//   }
//
//   c := dig.New()
//   err := c.Provide(&Type1{Name: "I am a thing"})
//   // dig container is now able to provide *Type1 as a dependency
//   // to other constructors that require it.
//
// Provide slices, maps, and arrays
//
// With dig, you can also use slices, maps, and arrays as objects
// to resolve, or you can
// Provide them as dependencies to the constructor.
//
//   c := dig.New()
//   var (
//   	typemap = map[string]int{}
//   	typeslice = []int{1, 2, 3}
//   	typearray = [2]string{"one", "two"}
//   )
//   err := c.Provide(typemap, typeslice, typearray)
//
//   var resolveslice []int
//   err := c.Resolve(&resolveslice)
//
//   c.Invoke(func(map map[string]int, slice []int, array [2]string) {
//   	// do something
//   })
//
// Resolve
//
// Resolve retrieves objects from the container by building the object graph.
//
// Object is resolution is based on the type of the variable passed into Resolvefunction.
//
//
// For example, in the current scenario:
//
//   // c := dig.New()
//   // c.Provide...
//   var cfg config.Provider
//   c.Resolve(&cfg) // note pointer to interface
//
// dig looks through the dependency graph and identifies whether a constructor was
// registered that is able to return a type of
// config.Provider. It then checks
// if said constructor requires any dependencies (by analyzing the function parameters).
// If it does,
// dig recursively completes resolutions of the parameters in the graph
// until the constructor for
// config.Provider can be fully satisfied.
//
// If resolution is not possible (for instance, if one of the required dependencies
// lacks a constructor and doesn't appear in the graph),
// dig returns an error.
//
// Invoke
//
// Invoke API executes the provided function and stores return results in the dig container.
//
// Invoke looks through the dig graph and resolves all the constructor parameters for execution.
// The return results, except the
// error object is inserted into the graph for further use.
// To utilize
// Invoke, function needs following form -
//
// • function arguments are pointers, maps, slice or arrays
//
// • function arguments must be provided to dig
//
// • function return parameters are optional
//
// • provided function can be anonymous
//
// For example, Invoke scenarios:
//
// Invoke anonymous function:
//
//    // c := dig.New()
//   // c.Provide... config.Provider, zap.Logger, etc
//   err := c.Invoke(func(cfg *config.Provider, l *zap.Logger) error {
//     // function body
//   	return nil
//   })
//
// Invoke recursively resolve dependencies:
//
//   type Foo struct{}
//
//   func invokeMe(cfg *config.Provider, l *zap.Logger) (*Foo, error) {
//     // function body
//   	return &Foo{}, nil
//   }
//
//   // c := dig.New()
//   // c.Provide... config.Provider, zap.Logger, etc
//   err := c.Invoke(invokeMe)
//   err := c.Invoke(func(foo *Foo) error {
//   	// foo is resolved from previous invokeMe call
//   	return nil
//   })
//   //
//
// Benchmarks
//
// Benchmark_CtorInvoke-8                               1000000          2137 ns/op         304 B/op         10 allocs/op
// Benchmark_CtorInvokeWithObjects-8                    1000000          1497 ns/op         200 B/op          6 allocs/op
// Benchmark_InvokeCtorWithMapsSlicesAndArrays-8         500000          2571 ns/op         440 B/op          9 allocs/op
// Benchmark_CtorProvideAndResolve-8                    1000000          2183 ns/op         320 B/op         11 allocs/op
// Benchmark_CtorResolve-8                              1000000          1903 ns/op         216 B/op          7 allocs/op
// Benchmark_ResolveCtors-8                              500000          2252 ns/op         344 B/op         14 allocs/op
//
//
//
package dig
