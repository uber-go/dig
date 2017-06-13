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
// Status
//
// BETA. Expect potential API changes.
//
// Container
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
// **All objects in the container are treated as a singletons**, meaning there can be
// only one object in the graph of a given type.
//
//
// There are plans to expand the API to support multiple objects of the same type
// in the container, but for time being consider using a factory pattern.
//
//
// Provide
//
// The Provide method adds a constructor of an object (or objects), to the container.
// A constructor can be a function returning any number of objects and, optionally,
// an error.
//
//
// Each argument to the constructor is registered as a **dependency** in the graph.
//
//   type A struct {}
//   type B struct {}
//   type C struct {}
//
//   c := dig.New()
//   constructor := func (*A, *B) *C {
//     // At this point, *A and *B have been resolved through the graph
//     // and can be used to provide an object of type *C
//     return &C{}
//   }
//   err := c.Provide(constructor)
//   // dig container is now able to provide *C to any constructor.
//   //
//   // However, note that in the current example *A and *B
//   // have not been provided, meaning that the resolution of type
//   // *C will result in an error, because types *A and *B can not
//   // be instantiated.
//
// Advanced Provide
//
// // TODO: docs on dig.In usage
// // TODO: docs on dig.Out usage
//
//
// Invoke
//
// The Invoke API is the flip side of Provide and used to retrieve types from the container.
//
// Invoke looks through the graph and resolves all the constructor parameters for execution.
//
// In order to successfully use use Invoke, the function must meet the following criteria:
//
// • Input to the Invoke must be a function
//
// • All arguments to the function must be types in the container
//
// • If an error is returned from an invoked function, it will be propagated to the caller
//
// Here is a fully working somewhat real-world Invoke example:
//
//   package main
//
//   import (
//   	"go.uber.org/config"
//   	"go.uber.org/dig"
//   	"go.uber.org/zap"
//   )
//
//   func main() {
//   	c := dig.New()
//
//   	// Provide configuration object
//   	c.Provide(func() config.Provider {
//   		return config.NewYAMLProviderFromBytes([]byte("tag: Hello, world!"))
//   	})
//
//   	// Provide a zap logger which relies on configuration
//   	c.Provide(func(cfg config.Provider) (*zap.Logger, error) {
//   		l, err := zap.NewDevelopment()
//   		if err != nil {
//   			return nil, err
//   		}
//   		return l.With(zap.String("iconic phrase", cfg.Get("tag").AsString())), nil
//   	})
//
//   	// Invoke a function that requires a zap logger, which in turn requires config
//   	c.Invoke(func(l *zap.Logger) {
//   		l.Info("You've been invoked")
//   		// Logger output:
//   		//     INFO    You've been invoked     {"iconic phrase": "Hello, world!"}
//   		//
//   		// As we can see, Invoke caused the Logger to be created, which in turn
//   		// required the configuration to be created.
//   	})
//   }
//
//
package dig
