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
	args, err := c.constructorArgs(ftype)
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
	keys, err := c.getReturnKeys(ctor, ctype)
	if err != nil {
		return errWrapf(err, "unable to collect return types of a constructor")
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
			return errWrapf(err, "introduces a cycle")
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
			if isError(k.t) {
				// Don't register errors into the container.
				return nil
			}

			// Tons of error checking
			if IsIn(k.t) {
				return errors.New("can't provide parameter objects")
			}
			if embedsType(k.t, _outPtrType) {
				return errors.New("can't embed *dig.Out pointers")
			}
			if k.t.Kind() == reflect.Ptr {
				if IsIn(k.t.Elem()) {
					return errors.New("can't provide pointers to parameter objects")
				}
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
	if !IsOut(k.t) {
		if k.t.Kind() == reflect.Ptr {
			if IsOut(k.t.Elem()) {
				return fmt.Errorf("%v is a pointer to dig.Out, use value type instead", k.t)
			}
		}

		// call the provided function on non-Out type
		if err := f(k); err != nil {
			return err
		}
		return nil
	}

	for i := 0; i < k.t.NumField(); i++ {
		field := k.t.Field(i)
		ft := field.Type

		if field.Type == _outType {
			// do not recurse into dig.Out itself, it will contain digSentinel only
			continue
		}

		if field.PkgPath != "" {
			return fmt.Errorf(
				"private fields not allowed in dig.Out, did you mean to export %q (%v) from %v",
				field.Name, field.Type, k.t)
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

	if IsIn(e.t) {
		// We do not want parameter objects to be cached.
		return c.createInObject(e.t)
	}
	if embedsType(e.t, _inPtrType) {
		return _noValue, fmt.Errorf(
			"%v embeds *dig.In which is not supported, embed dig.In value instead", e.t,
		)
	}

	if e.t.Kind() == reflect.Ptr {
		if IsIn(e.t.Elem()) {
			return _noValue, fmt.Errorf(
				"dependency %v is a pointer to dig.In, use value type instead", e.t,
			)
		}
	}

	n, ok := c.nodes[e.key]
	if !ok {
		// Unlike in the fallback case below, if a user makes an error requesting
		// a mixed type for an optional parameter, a good error message "did you mean X?"
		// will not be used and dig will return zero value.
		if e.optional {
			return reflect.Zero(e.t), nil
		}

		// If the type being asked for is the pointer that is not found,
		// check if the graph contains the value type element - perhaps the user
		// accidentally included a splat and vice versa.
		var typo reflect.Type
		if e.t.Kind() == reflect.Ptr {
			typo = e.t.Elem()
		} else {
			typo = reflect.PtrTo(e.t)
		}

		tk := key{t: typo, name: e.name}
		if _, ok := c.nodes[tk]; ok {
			return _noValue, fmt.Errorf(
				"type %v is not in the container, did you mean to use %v?", e.key, tk)
		}

		return _noValue, fmt.Errorf("type %v isn't in the container", e.key)
	}

	if err := c.contains(n.deps); err != nil {
		if e.optional {
			return reflect.Zero(e.t), nil
		}
		return _noValue, errWrapf(err, "missing dependencies for %v", e.key)
	}

	args, err := c.constructorArgs(n.ctype)
	if err != nil {
		return _noValue, errWrapf(err, "couldn't get arguments for constructor %v", n.ctype)
	}
	constructed := reflect.ValueOf(n.ctor).Call(args)

	// Provide-time validation ensures that all constructors return at least
	// one value.
	if errV := constructed[len(constructed)-1]; isError(errV.Type()) {
		if err, _ := errV.Interface().(error); err != nil {
			return _noValue, errWrapf(err, "constructor %v for type %v failed", n.ctype, e.t)
		}
	}

	for _, con := range constructed {
		// Set the resolved object into the cache.
		// This might look confusing at first like we're ignoring named types,
		// but `con` in this case will be the dig.Out object, which will
		// cause a recursion into the .set for each of it's memebers.
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

		if f.Type == _inType {
			// skip over the dig.In embed itself
			continue
		}

		if f.PkgPath != "" {
			return dest, fmt.Errorf(
				"private fields not allowed in dig.In, did you mean to export %q (%v) from %v?",
				f.Name, f.Type, t)
		}

		isOptional, err := isFieldOptional(t, f)
		if err != nil {
			return dest, err
		}

		e := edge{key: key{t: f.Type, name: f.Tag.Get(_nameTag)}, optional: isOptional}
		v, err := c.get(e)
		if err != nil {
			return dest, errWrapf(err, "could not get field %v (edge %v) of %v", f.Name, e, t)
		}
		dest.Field(i).Set(v)
	}
	return dest, nil
}

// Set the value in the cache after a node resolution
func (c *Container) set(k key, v reflect.Value) {
	if !IsOut(k.t) {
		// do not cache error types
		if k.t != _errType {
			c.cache[k] = v
		}
		return
	}

	// dig.Out objects are not acted upon directly, but rather their members are considered
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
	argTypes := getConstructorArgTypes(ctype)
	args := make([]reflect.Value, 0, len(argTypes))
	for _, t := range argTypes {
		arg, err := c.get(edge{key: key{t: t}})
		if err != nil {
			return nil, errWrapf(err, "couldn't get arguments for constructor %v", ctype)
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
	for _, t := range getConstructorArgTypes(ctype) {
		err := traverseInTypes(t, func(e edge) {
			deps = append(deps, e)
		})
		if err != nil {
			return nil, err
		}
	}
	return deps, nil
}

// Retrieves the types of the arguments of a constructor in-order.
//
// If the constructor is a variadic function, the returned list does NOT
// include the implicit slice argument because dig does not support passing
// those values in yet.
func getConstructorArgTypes(ctype reflect.Type) []reflect.Type {
	numArgs := ctype.NumIn()
	if ctype.IsVariadic() {
		// NOTE: If the function is variadic, we skip the last argument
		// because we're not filling variadic arguments yet. See #120.
		numArgs--
	}

	args := make([]reflect.Type, numArgs)
	for i := 0; i < numArgs; i++ {
		args[i] = ctype.In(i)
	}
	return args
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
	if !IsIn(t) {
		fn(edge{key: key{t: t}})
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // skip private fields
		}

		if IsIn(f.Type) {
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
		err = errWrapf(err,
			"invalid value %q for %q tag on field %v of %v",
			tag, _optionalTag, f.Name, parent)
	}

	return optional, err
}
