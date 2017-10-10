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

package dig

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

const (
	_optionalTag = "optional"
	_nameTag     = "name"
)

// Unique identification of an object in the graph.
type key struct {
	t    reflect.Type
	name string
}

// Option configures a Container. It's included for future functionality;
// currently, there are no concrete implementations.
type Option interface {
	unimplemented()
}

// A ProvideOption modifies the default behavior of Provide. It's included for
// future functionality; currently, there are no concrete implementations.
type ProvideOption interface {
	unimplemented()
}

// An InvokeOption modifies the default behavior of Invoke. It's included for
// future functionality; currently, there are no concrete implementations.
type InvokeOption interface {
	unimplemented()
}

// Container is a directed acyclic graph of types and their dependencies.
type Container struct {
	nodes map[key]*node
	cache map[key]reflect.Value

	// TODO: for advanced use-case, add an index
	// This will allow retrieval of a single type, without specifying the exact
	// tag, provided there is only one object of that given type
	//
	// It will also allow library owners to create a "default" tag for their
	// object, in case users want to provide another type with a different name
	//
	// index map[reflect.Type]key
}

// New constructs a Container.
func New(opts ...Option) *Container {
	return &Container{
		nodes: make(map[key]*node),
		cache: make(map[key]reflect.Value),
	}
}

// Provide teaches the container how to build values of one or more types and
// expresses their dependencies.
//
// The first argument of Provide is a function that accepts zero or more
// parameters and returns one or more results. The function may optionally
// return an error to indicate that it failed to build the value. This
// function will be treated as the constructor for all the types it returns.
// This function will be called AT MOST ONCE when a type produced by it, or a
// type that consumes this function's output, is requested via Invoke. If the
// same types are requested multiple times, the previously produced value will
// be reused.
//
// In addition to accepting constructors that accept dependencies as separate
// arguments and produce results as separate return values, Provide also
// accepts constructors that specify dependencies as dig.In structs and/or
// specify results as dig.Out structs.
func (c *Container) Provide(constructor interface{}, opts ...ProvideOption) error {
	ctype := reflect.TypeOf(constructor)
	if ctype == nil {
		return errors.New("can't provide an untyped nil")
	}
	if ctype.Kind() != reflect.Func {
		return fmt.Errorf("must provide constructor function, got %v (type %v)", constructor, ctype)
	}
	if err := c.provide(constructor, ctype); err != nil {
		return errWrapf(err, "can't provide %v", ctype)
	}
	return nil
}

// Invoke runs the given function after instantiating its dependencies.
//
// Any arguments that the function has are treated as its dependencies. The
// dependencies are instantiated in an unspecified order along with any
// dependencies that they might have.
//
// The function may return an error to indicate failure. The error will be
// returned to the caller as-is.
func (c *Container) Invoke(function interface{}, opts ...InvokeOption) error {
	ftype := reflect.TypeOf(function)
	if ftype == nil {
		return errors.New("can't invoke an untyped nil")
	}
	if ftype.Kind() != reflect.Func {
		return fmt.Errorf("can't invoke non-function %v (type %v)", function, ftype)
	}

	pl, err := newParamList(ftype)
	if err != nil {
		return err
	}

	args, err := pl.BuildList(c)
	if err != nil {
		return errWrapf(err, "failed to get arguments for %v (type %v)", function, ftype)
	}

	returned := reflect.ValueOf(function).Call(args)
	if len(returned) == 0 {
		return nil
	}
	if last := returned[len(returned)-1]; isError(last.Type()) {
		if err, _ := last.Interface().(error); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) provide(ctor interface{}, ctype reflect.Type) error {
	n, err := newNode(ctor, ctype)
	if err != nil {
		return err
	}

	for k := range n.Results.Produces() {
		if _, ok := c.nodes[k]; ok {
			return fmt.Errorf("%v (%v) provides %v, which is already in the container", ctor, ctype, k)
		}
		c.nodes[k] = n

		if err := c.isAcyclic(n.Params, k); err != nil {
			delete(c.nodes, k)
			return errWrapf(err, "%v (%v) introduces a cycle", ctor, ctype)
		}
	}

	return nil
}

func (c *Container) isAcyclic(p param, k key) error {
	return detectCycles(p, c.nodes, []key{k})
}

type node struct {
	ctor  interface{}
	ctype reflect.Type

	// Type information about constructor parameters.
	Params paramList

	// Type information about constructor results.
	Results resultList
}

func newNode(ctor interface{}, ctype reflect.Type) (*node, error) {
	params, err := newParamList(ctype)
	if err != nil {
		return nil, err
	}

	results, err := newResultList(ctype)
	if err != nil {
		return nil, err
	}

	return &node{
		ctor:    ctor,
		ctype:   ctype,
		Params:  params,
		Results: results,
	}, err
}

func (n *node) Call(c *Container) error {
	args, err := n.Params.BuildList(c)
	if err != nil {
		return errWrapf(err, "couldn't get arguments for constructor %v", n.ctype)
	}

	results := reflect.ValueOf(n.ctor).Call(args)
	if err := n.Results.ExtractList(c, results); err != nil {
		return errWrapf(err, "constructor %v failed", n.ctype)
	}

	return nil
}

type errCycleDetected struct {
	Path []key
	Key  key
}

func (e errCycleDetected) Error() string {
	b := new(bytes.Buffer)
	for _, k := range e.Path {
		fmt.Fprintf(b, "%v ->", k.t)
	}
	fmt.Fprintf(b, "%v", e.Key.t)
	return b.String()
}

func detectCycles(par param, graph map[key]*node, path []key) error {
	var err error
	walkParam(par, paramVisitorFunc(func(param param) bool {
		if err != nil {
			return false
		}

		p, ok := param.(paramSingle)
		if !ok {
			return true
		}

		k := key{name: p.Name, t: p.Type}
		for _, p := range path {
			if p == k {
				err = errCycleDetected{Path: path, Key: k}
				return false
			}
		}

		n, ok := graph[k]
		if !ok {
			return true
		}

		if e := detectCycles(n.Params, graph, append(path, k)); e != nil {
			err = e
		}

		return true
	}))

	return err
}

// Checks if a field of an In struct is optional.
func isFieldOptional(parent reflect.Type, f reflect.StructField) (bool, error) {
	tag := f.Tag.Get(_optionalTag)
	if tag == "" {
		return false, nil
	}

	optional, err := strconv.ParseBool(tag)
	if err != nil {
		err = errWrapf(err,
			"invalid value %q for %q tag on field %v of %v",
			tag, _optionalTag, f.Name, parent)
	}

	return optional, err
}

// Checks that all direct dependencies of the provided param are present in
// the container. Returns an error if not.
func shallowCheckDependencies(c *Container, p param) error {
	var missing []key
	walkParam(p, paramVisitorFunc(func(p param) bool {
		ps, ok := p.(paramSingle)
		if !ok {
			return true
		}

		k := key{name: ps.Name, t: ps.Type}
		if _, ok := c.nodes[k]; !ok && !ps.Optional {
			missing = append(missing, k)
		}

		return true
	}))

	if len(missing) > 0 {
		return fmt.Errorf("container is missing: %v", missing)
	}
	return nil
}
