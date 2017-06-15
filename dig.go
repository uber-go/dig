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

// A Container is a directed, acyclic graph of dependencies. Dependencies are
// constructed on-demand and returned from a cache thereafter, so they're
// effectively singletons.
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

// New constructs a ready-to-use Container.
func New() *Container {
	return &Container{
		nodes: make(map[key]*node),
		cache: make(map[key]reflect.Value),
	}
}

// Provide teaches the Container how to construct one or more new types. It
// accepts either a constructor function or an already-constructed object.
//
// Any function passed to Provide is assumed to be a constructor. Constructors
// can take any number of parameters, which will be supplied by the Container
// on demand. They must return at least one non-error value, all of which are
// then available in the Container. If the last returned value is an error, the
// Container inspects it to determine whether the constructor succeeded or
// failed. Regardless of position, returned errors are never put into the
// Container's dependency graph.
//
// All non-functions (including structs, pointers, Go's built-in collections,
// and primitive types like ints) are inserted into the Container as-is.
func (c *Container) Provide(constructor interface{}) error {
	ctype := reflect.TypeOf(constructor)
	if ctype == nil {
		return errors.New("can't provide an untyped nil")
	}
	if ctype.Kind() != reflect.Func {
		return fmt.Errorf("must provide constructor function, got %v (type %v)", constructor, ctype)
	}
	if err := c.provide(constructor, ctype); err != nil {
		return fmt.Errorf("can't provide %v: %v", ctype, err)
	}
	return nil
}

// Invoke runs a function, supplying its arguments from the Container. If the
// function's last return value is an error, that error is propagated to the
// caller. All other returned values (if any) are ignored.
//
// Passing anything other than a function to Invoke returns an error
// immediately.
func (c *Container) Invoke(function interface{}) error {
	ftype := reflect.TypeOf(function)
	if ftype == nil {
		return errors.New("can't invoke an untyped nil")
	}
	if ftype.Kind() != reflect.Func {
		return fmt.Errorf("can't invoke non-function %v (type %v)", function, ftype)
	}
	args, err := c.constructorArgs(ftype)
	if err != nil {
		return fmt.Errorf("failed to get arguments for %v (type %v): %v", function, ftype, err)
	}
	returned := reflect.ValueOf(function).Call(args)
	if len(returned) == 0 {
		return nil
	}
	if last := returned[len(returned)-1]; last.Type() == _errType {
		if err, _ := last.Interface().(error); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) provide(ctor interface{}, ctype reflect.Type) error {
	keys, err := c.getReturnKeys(ctor, ctype)
	if err != nil {
		return fmt.Errorf("unable to collect return types of a constructor: %v", err)
	}

	nodes := make([]*node, 0, len(keys))
	for k := range keys {
		n, err := newNode(k, ctor, ctype)
		if err != nil {
			return err
		}
		nodes = append(nodes, n)
		c.nodes[k] = n
	}

	for _, n := range nodes {
		if err := c.isAcyclic(n); err != nil {
			c.remove(nodes)
			return fmt.Errorf("introduces a cycle: %v", err)
		}
	}

	return nil
}

// Get the return types of a constructor with all the dig.Out returns get expanded.
func (c *Container) getReturnKeys(
	ctor interface{},
	ctype reflect.Type,
) (map[key]struct{}, error) {
	// Could pre-compute the size but it's tricky as counter is different
	// when dig.Out objects are mixed in
	returnTypes := make(map[key]struct{})

	// Check each return object
	for i := 0; i < ctype.NumOut(); i++ {
		outt := ctype.Out(i)

		err := traverseOutTypes(key{t: outt}, func(k key) error {
			if k.t == _errType {
				// Don't register errors into the container.
				return nil
			}

			// Tons of error checking
			if isInObject(k.t) {
				return errors.New("can't provide parameter objects")
			}
			if _, ok := returnTypes[k]; ok {
				return fmt.Errorf("returns multiple %v", k)
			}
			if _, ok := c.nodes[k]; ok {
				return fmt.Errorf("provides %v, which is already in the container", k)
			}

			returnTypes[k] = struct{}{}
			return nil
		})
		if err != nil {
			return returnTypes, err
		}
	}
	if len(returnTypes) == 0 {
		return nil, errors.New("must provide at least one non-error type")
	}

	return returnTypes, nil
}

// DFS traverse over all the types and execute the provided function.
// Types that embed dig.Out get recursed on. Returns the first error encountered.
func traverseOutTypes(k key, f func(key) error) error {
	if !isOutObject(k.t) {
		// call the provided function on non-Out type
		if err := f(k); err != nil {
			return err
		}
		return nil
	}

	for i := 0; i < k.t.NumField(); i++ {
		field := k.t.Field(i)
		ft := field.Type

		if field.PkgPath != "" {
			continue // skip private fields
		}

		// keep recursing to traverse all the embedded objects
		if err := traverseOutTypes(key{t: ft, name: field.Tag.Get(_nameTag)}, f); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) isAcyclic(n *node) error {
	return detectCycles(n, c.nodes, nil)
}

// Retrieve a type from the container
func (c *Container) get(e edge) (reflect.Value, error) {
	if v, ok := c.cache[e.key]; ok {
		return v, nil
	}

	if isInObject(e.t) {
		// We do not want parameter objects to be cached.
		return c.createInObject(e.t)
	}

	n, ok := c.nodes[e.key]
	if !ok {
		if e.optional {
			return reflect.Zero(e.t), nil
		}
		return _noValue, fmt.Errorf("type %v isn't in the container", e.key)
	}

	if err := c.contains(n.deps); err != nil {
		return _noValue, fmt.Errorf("missing dependencies for %v: %v", e.key, err)
	}

	args, err := c.constructorArgs(n.ctype)
	if err != nil {
		return _noValue, fmt.Errorf("couldn't get arguments for constructor %v: %v", n.ctype, err)
	}
	constructed := reflect.ValueOf(n.ctor).Call(args)

	// Provide-time validation ensures that all constructors return at least
	// one value.
	if err := constructed[len(constructed)-1]; err.Type() == _errType && err.Interface() != nil {
		return _noValue, fmt.Errorf("constructor %v for type %v failed: %v", n.ctype, e.t, err.Interface())
	}

	for _, con := range constructed {
		c.set(key{t: con.Type()}, con)
	}
	return c.cache[e.key], nil
}

// Returns a new In parent object with all the dependency fields
// populated from the dig container.
func (c *Container) createInObject(t reflect.Type) (reflect.Value, error) {
	dest := reflect.New(t).Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // skip private fields
		}

		isOptional, err := isFieldOptional(t, f)
		if err != nil {
			return dest, err
		}

		e := edge{key: key{t: f.Type, name: f.Tag.Get(_nameTag)}, optional: isOptional}
		v, err := c.get(e)
		if err != nil {
			return dest, fmt.Errorf(
				"could not get field %v (edge %v) of %v: %v", f.Name, e, t, err)
		}

		dest.Field(i).Set(v)
	}
	return dest, nil
}

// Set the value in the cache after a node resolution
func (c *Container) set(k key, v reflect.Value) {
	if !isOutObject(k.t) {
		// do not cache error types
		if k.t != _errType {
			c.cache[k] = v
		}
		return
	}

	// dig.Out objects are not acted upon directly, but rather their memebers are considered
	for i := 0; i < k.t.NumField(); i++ {
		f := k.t.Field(i)

		// recurse into all fields, which may or may not be more dig.Out objects
		fk := key{t: f.Type, name: f.Tag.Get(_nameTag)}
		c.set(fk, v.Field(i))
	}
}

func (c *Container) contains(deps []edge) error {
	var missing []key
	for _, d := range deps {
		if _, ok := c.nodes[d.key]; !ok && !d.optional {
			missing = append(missing, d.key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("container is missing: %v", missing)
	}
	return nil
}

func (c *Container) remove(nodes []*node) {
	for _, n := range nodes {
		delete(c.nodes, n.key)
	}
}

func (c *Container) constructorArgs(ctype reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, ctype.NumIn())
	for i := 0; i < ctype.NumIn(); i++ {
		arg, err := c.get(edge{key: key{t: ctype.In(i)}})
		if err != nil {
			return nil, fmt.Errorf("couldn't get arguments for constructor %v: %v", ctype, err)
		}
		args = append(args, arg)
	}
	return args, nil
}

type node struct {
	key

	ctor  interface{}
	ctype reflect.Type
	deps  []edge
}

type edge struct {
	key

	optional bool
}

func newNode(k key, ctor interface{}, ctype reflect.Type) (*node, error) {
	deps, err := getConstructorDependencies(ctype)
	return &node{
		key:   k,
		ctor:  ctor,
		ctype: ctype,
		deps:  deps,
	}, err
}

// Retrieves the dependencies for a constructor
func getConstructorDependencies(ctype reflect.Type) ([]edge, error) {
	var deps []edge
	for i := 0; i < ctype.NumIn(); i++ {
		err := traverseInTypes(ctype.In(i), func(e edge) {
			deps = append(deps, e)
		})
		if err != nil {
			return nil, err
		}
	}
	return deps, nil
}

func cycleError(cycle []key, last key) error {
	b := &bytes.Buffer{}
	for _, k := range cycle {
		fmt.Fprintf(b, "%v ->", k.t)
	}
	fmt.Fprintf(b, "%v", last.t)
	return errors.New(b.String())
}

func detectCycles(n *node, graph map[key]*node, path []key) error {
	for _, p := range path {
		if p == n.key {
			return cycleError(path, n.key)
		}
	}
	path = append(path, n.key)
	for _, dep := range n.deps {
		depNode, ok := graph[dep.key]
		if !ok {
			continue
		}
		if err := detectCycles(depNode, graph, path); err != nil {
			return err
		}
	}
	return nil
}

// Traverse all fields starting with the given type.
// Types that dig.In get recursed on. Returns the first error encountered.
func traverseInTypes(t reflect.Type, fn func(edge)) error {
	if !isInObject(t) {
		fn(edge{key: key{t: t}})
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // skip private fields
		}

		if isInObject(f.Type) {
			if err := traverseInTypes(f.Type, fn); err != nil {
				return err
			}
			continue
		}

		optional, err := isFieldOptional(t, f)
		if err != nil {
			return err
		}

		fn(edge{key: key{t: f.Type, name: f.Tag.Get(_nameTag)}, optional: optional})
	}

	return nil
}

// Checks if a field of an In struct is optional.
func isFieldOptional(parent reflect.Type, f reflect.StructField) (bool, error) {
	tag := f.Tag.Get(_optionalTag)
	if tag == "" {
		return false, nil
	}

	optional, err := strconv.ParseBool(tag)
	if err != nil {
		err = fmt.Errorf(
			"invalid value %q for %q tag on field %v of %v: %v",
			tag, _optionalTag, f.Name, parent, err)
	}

	return optional, err
}
