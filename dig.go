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
)

// A Container is a directed, acyclic graph of dependencies. Dependencies are
// constructed on-demand and returned from a cache thereafter, so they're
// effectively singletons.
type Container struct {
	nodes map[nodeKey]node
	cache map[nodeKey]reflect.Value
}

// New constructs a ready-to-use Container.
func New() *Container {
	return &Container{
		nodes: make(map[nodeKey]node),
		cache: make(map[nodeKey]reflect.Value),
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
	returnTypes, err := c.getReturnTypes(ctor, ctype)
	if err != nil {
		return fmt.Errorf("unable to collect return types of a constructor: %v", err)
	}

	nodes := make([]node, 0, len(returnTypes))
	for rt := range returnTypes {
		n, err := newNode(rt, ctor, ctype)
		if err != nil {
			return err
		}
		nodes = append(nodes, n)
		c.nodes[n.key] = n
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
func (c *Container) getReturnTypes(
	ctor interface{},
	ctype reflect.Type,
) (map[reflect.Type]struct{}, error) {
	// Could pre-compute the size but it's tricky as counter is different
	// when dig.Out objects are mixed in
	returnTypes := make(map[reflect.Type]struct{})

	// Check each return object
	for i := 0; i < ctype.NumOut(); i++ {
		outt := ctype.Out(i)

		err := traverseOutTypes(outt, func(rt reflect.Type) error {
			k := key(rt)
			if rt == _errType {
				// Don't register errors into the container.
				return nil
			}

			// Tons of error checking
			if isInObject(rt) {
				return errors.New("can't provide parameter objects")
			}
			if _, ok := returnTypes[rt]; ok {
				return fmt.Errorf("returns multiple %v", rt)
			}
			if _, ok := c.nodes[k]; ok {
				return fmt.Errorf("provides type %v, which is already in the container", rt)
			}

			returnTypes[rt] = struct{}{}
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

// Do a DFS traverse over all dig.Out members (recursive) and perform an action.
// Returns the first error encountered.
func traverseOutTypes(t reflect.Type, f func(t reflect.Type) error) error {
	if !isOutObject(t) {
		// call the provided function on non-Out type
		if err := f(t); err != nil {
			return err
		}
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		ft := field.Type

		if field.PkgPath != "" {
			continue // skip private fields
		}

		// keep recursing to traverse all the embedded objects
		traverseOutTypes(ft, f)
	}
	return nil
}

func (c *Container) isAcyclic(n node) error {
	return detectCycles(n, c.nodes, nil)
}

// Retrieve a type from the container
func (c *Container) get(k nodeKey) (reflect.Value, error) {
	if v, ok := c.cache[k]; ok {
		return v, nil
	}

	if isInObject(k.t) {
		// We do not want parameter objects to be cached.
		return c.createInObject(k.t)
	}

	n, ok := c.nodes[k]
	if !ok {
		return _noValue, fmt.Errorf("type %v isn't in the container", k.t)
	}

	if err := c.contains(n.deps); err != nil {
		return _noValue, fmt.Errorf("missing dependencies for type %v: %v", k.t, err)
	}

	args, err := c.constructorArgs(n.ctype)
	if err != nil {
		return _noValue, fmt.Errorf("couldn't get arguments for constructor %v: %v", n.ctype, err)
	}
	constructed := reflect.ValueOf(n.ctor).Call(args)

	// Provide-time validation ensures that all constructors return at least
	// one value.
	if err := constructed[len(constructed)-1]; err.Type() == _errType && err.Interface() != nil {
		return _noValue, fmt.Errorf(
			"constructor %v for type %v failed: %v", n.ctype, k.t, err.Interface(),
		)
	}

	for _, con := range constructed {
		c.set(con)
	}
	return c.cache[k], nil
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

		var isOptional bool
		if tag := f.Tag.Get(_optionalTag); tag != "" {
			var err error
			isOptional, err = strconv.ParseBool(tag)
			if err != nil {
				return dest, fmt.Errorf(
					"invalid value %q for %q tag on field %v of %v: %v",
					tag, _optionalTag, f.Name, t, err)
			}
		}

		v, err := c.get(key(f.Type))
		if err != nil {
			if isOptional {
				v = reflect.Zero(f.Type)
			} else {
				return dest, fmt.Errorf(
					"could not get field %v (type %v) of %v: %v", f.Name, f.Type, t, err)
			}
		}

		dest.Field(i).Set(v)
	}
	return dest, nil
}

// Set the value in the cache after a node resolution
func (c *Container) set(v reflect.Value) {
	t := v.Type()
	if !isOutObject(t) {
		// do not cache error types
		if t != _errType {
			c.cache[key(t)] = v
		}
		return
	}

	// dig.Out objects are not acted upon directly, but rather their memebers are considered
	for i := 0; i < t.NumField(); i++ {
		// recurse into all fields, which may or may not be more dig.Out objects
		c.set(v.Field(i))
	}
}

func (c *Container) contains(keys []nodeKey) error {
	var missing []reflect.Type
	for _, k := range keys {
		if _, ok := c.nodes[k]; !ok && !k.optional {
			missing = append(missing, k.t)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("container is missing types: %v", missing)
	}
	return nil
}

func (c *Container) remove(nodes []node) {
	for _, n := range nodes {
		delete(c.nodes, n.key)
	}
}

func (c *Container) constructorArgs(ctype reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, ctype.NumIn())
	for i := 0; i < ctype.NumIn(); i++ {
		arg, err := c.get(key(ctype.In(i)))
		if err != nil {
			return nil, fmt.Errorf("couldn't get arguments for constructor %v: %v", ctype, err)
		}
		args = append(args, arg)
	}
	return args, nil
}

// Graph node represents a single return type from a constructor.
// One constructor function returning multiple types get adeed to the graph
// as multiple individual nodes. When one type is required to be initialized,
// all nodes in the graph beloning to the constructor get inserted into the
// cache.
type node struct {
	provides reflect.Type
	ctor     interface{}
	ctype    reflect.Type
	deps     []nodeKey
	key      nodeKey
}

func newNode(provides reflect.Type, ctor interface{}, ctype reflect.Type) (node, error) {
	deps, err := getConstructorDependencies(ctype)
	return node{
		provides: provides,
		ctor:     ctor,
		ctype:    ctype,
		deps:     deps,
		key:      key(provides),
	}, err
}

// Retrieves the dependencies for a constructor
func getConstructorDependencies(ctype reflect.Type) ([]nodeKey, error) {
	var deps []nodeKey
	for i := 0; i < ctype.NumIn(); i++ {
		err := traverseInTypes(ctype.In(i), func(k nodeKey) {
			deps = append(deps, k)
		})
		if err != nil {
			return nil, err
		}
	}
	return deps, nil
}

func cycleError(cycle []reflect.Type, last reflect.Type) error {
	b := &bytes.Buffer{}
	for _, t := range cycle {
		fmt.Fprintf(b, "%v ->", t)
	}
	fmt.Fprintf(b, "%v", last)
	return errors.New(b.String())
}

func detectCycles(n node, graph map[nodeKey]node, path []reflect.Type) error {
	for _, p := range path {
		if p == n.provides {
			return cycleError(path, n.provides)
		}
	}
	path = append(path, n.provides)
	for _, dep := range n.deps {
		depNode, ok := graph[dep]
		if !ok {
			continue
		}
		if err := detectCycles(depNode, graph, path); err != nil {
			return err
		}
	}
	return nil
}

// traverseInTypes traverses fields of a dig.In struct in depth-first order.
//
// If called with a non-In object, the function is called right away.
func traverseInTypes(t reflect.Type, fn func(k nodeKey)) error {
	if !isInObject(t) {
		fn(key(t))
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

		opt, err := isFieldOptional(t, f)
		if err != nil {
			return err
		}

		fn(key(f.Type, optional(opt)))
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
