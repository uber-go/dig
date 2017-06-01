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
)

var (
	_noValue             reflect.Value
	_errType             = reflect.TypeOf((*error)(nil)).Elem()
	_parameterObjectType = reflect.TypeOf((*parameterObject)(nil)).Elem()
)

const _optionalTag = "optional"

// A Container is a directed, acyclic graph of dependencies. Dependencies are
// constructed on-demand and returned from a cache thereafter, so they're
// effectively singletons.
type Container struct {
	nodes map[reflect.Type]node
	cache map[reflect.Type]reflect.Value
}

// New constructs a ready-to-use Container.
func New() *Container {
	return &Container{
		nodes: make(map[reflect.Type]node),
		cache: make(map[reflect.Type]reflect.Value),
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
	// Since we want to wrap any errors, don't return early.
	var err error
	if ctype.Kind() != reflect.Func {
		err = c.provideInstance(constructor)
	} else {
		err = c.provideConstructor(constructor, ctype)
	}

	if err == nil {
		return nil
	}
	return fmt.Errorf("can't provide %v: %v", ctype, err)
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
	if last := returned[len(returned)-1]; last.Type() == _errType && last.Interface() != nil {
		return fmt.Errorf("failed to execute %v (type %v): %v", function, ftype, last.Interface())
	}
	return nil
}

func (c *Container) provideInstance(val interface{}) error {
	vtype := reflect.TypeOf(val)
	if vtype == _errType {
		return errors.New("can't provide errors")
	}
	if vtype.Implements(_parameterObjectType) {
		return errors.New("can't provide parameter objects")
	}
	if _, ok := c.nodes[vtype]; ok {
		return errors.New("already in container")
	}
	c.nodes[vtype] = node{provides: vtype}
	c.cache[vtype] = reflect.ValueOf(val)
	return nil
}

func (c *Container) provideConstructor(ctor interface{}, ctype reflect.Type) error {
	returnTypes := make(map[reflect.Type]struct{}, ctype.NumOut())
	for i := 0; i < ctype.NumOut(); i++ {
		rt := ctype.Out(i)
		if rt == _errType {
			// Don't register errors into the container.
			continue
		}
		if rt.Implements(_parameterObjectType) {
			return errors.New("can't provide parameter objects")
		}
		if _, ok := returnTypes[rt]; ok {
			return fmt.Errorf("returns multiple %v", rt)
		}
		if _, ok := c.nodes[rt]; ok {
			return fmt.Errorf("provides type %v, which is already in the container", rt)
		}
		returnTypes[rt] = struct{}{}
	}
	if len(returnTypes) == 0 {
		return errors.New("must provide at least one non-error type")
	}

	nodes := make([]node, 0, len(returnTypes))
	for rt := range returnTypes {
		n, err := newNode(rt, ctor, ctype)
		if err != nil {
			return err
		}
		nodes = append(nodes, n)
		c.nodes[rt] = n
	}

	for _, n := range nodes {
		if err := c.isAcyclic(n); err != nil {
			c.remove(nodes)
			return fmt.Errorf("introduces a cycle: %v", err)
		}
	}

	return nil
}

func (c *Container) isAcyclic(n node) error {
	return detectCycles(n, c.nodes, nil, make(map[reflect.Type]struct{}))
}

// Retrieve a type from the container
func (c *Container) get(t reflect.Type) (reflect.Value, error) {
	if v, ok := c.cache[t]; ok {
		return v, nil
	}

	if t.Implements(_parameterObjectType) {
		// We do not want parameter objects to be cached.
		return c.createParamObject(t)
	}

	n, ok := c.nodes[t]
	if !ok {
		return _noValue, fmt.Errorf("type %v isn't in the container", t)
	}

	if err := c.contains(n.deps); err != nil {
		return _noValue, fmt.Errorf("missing dependencies for type %v: %v", t, err)
	}

	args, err := c.constructorArgs(n.ctype)
	if err != nil {
		return _noValue, fmt.Errorf("couldn't get arguments for constructor %v: %v", n.ctype, err)
	}
	constructed := reflect.ValueOf(n.ctor).Call(args)

	// Provide-time validation ensures that all constructors return at least
	// one value.
	if err := constructed[len(constructed)-1]; err.Type() == _errType && err.Interface() != nil {
		return _noValue, fmt.Errorf("constructor %v for type %v failed: %v", n.ctype, t, err.Interface())
	}

	for _, con := range constructed {
		ct := con.Type()
		if ct == _errType {
			continue
		}
		c.cache[ct] = con
	}
	return c.cache[t], nil
}

func (c *Container) contains(types []reflect.Type) error {
	var missing []reflect.Type
	for _, t := range types {
		if _, ok := c.nodes[t]; !ok {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("container is missing types: %v", missing)
	}
	return nil
}

func (c *Container) remove(nodes []node) {
	for _, n := range nodes {
		delete(c.nodes, n.provides)
	}
}

func (c *Container) constructorArgs(ctype reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, ctype.NumIn())
	for i := 0; i < ctype.NumIn(); i++ {
		arg, err := c.get(ctype.In(i))
		if err != nil {
			return nil, fmt.Errorf("couldn't get arguments for constructor %v: %v", ctype, err)
		}
		args = append(args, arg)
	}
	return args, nil
}

type node struct {
	provides reflect.Type
	ctor     interface{}
	ctype    reflect.Type
	deps     []reflect.Type
}

func newNode(provides reflect.Type, ctor interface{}, ctype reflect.Type) (node, error) {
	deps := make([]reflect.Type, 0, ctype.NumIn())
	for i := 0; i < ctype.NumIn(); i++ {
		deps = append(deps, getCtorParamDependencies(ctype.In(i))...)
	}

	return node{
		provides: provides,
		ctor:     ctor,
		ctype:    ctype,
		deps:     deps,
	}, nil
}

// Retrives the dependencies for the parameter of a constructor.
func getCtorParamDependencies(t reflect.Type) (deps []reflect.Type) {
	if !t.Implements(_parameterObjectType) {
		deps = append(deps, t)
		return
	}

	deps = make([]reflect.Type, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // skip private fields
		}

		deps = append(deps, getCtorParamDependencies(f.Type)...)
	}

	return
}

func cycleError(cycle []reflect.Type, last reflect.Type) error {
	b := &bytes.Buffer{}
	for _, t := range cycle {
		fmt.Fprintf(b, "%v ->", t)
	}
	fmt.Fprintf(b, "%v", last)
	return errors.New(b.String())
}

func detectCycles(n node, graph map[reflect.Type]node, path []reflect.Type, seen map[reflect.Type]struct{}) error {
	if _, ok := seen[n.provides]; ok {
		return cycleError(path, n.provides)
	}
	path = append(path, n.provides)
	seen[n.provides] = struct{}{}
	for _, depType := range n.deps {
		depNode, ok := graph[depType]
		if !ok {
			continue
		}
		if err := detectCycles(depNode, graph, path, seen); err != nil {
			return err
		}
	}
	return nil
}

// Param is embedded inside structs to opt those structs in as Dig parameter
// objects.
type Param struct{}

// TODO usage docs for param

var _ parameterObject = Param{}

// Param is the only instance of parameterObject.
func (Param) parameterObject() {}

// Users embed the Param struct to opt a struct in as a parameter object.
// Param implements this interface so the struct into which Param is embedded
// also implements this interface. This provides us an easy way to check if
// something embeds Param without iterating through all its fields.
type parameterObject interface {
	parameterObject()
}

// Returns a new Param parent object with all the dependency fields
// populated from the dig container.
func (c *Container) createParamObject(t reflect.Type) (reflect.Value, error) {
	dest := reflect.New(t).Elem()
	result := dest
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		dest.Set(reflect.New(t))
		dest = dest.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // skip private fields
		}

		v, err := c.get(f.Type)
		if err != nil {
			switch f.Tag.Get(_optionalTag) {
			case "true", "yes":
				v = reflect.Zero(f.Type)
			default:
				return result, fmt.Errorf(
					"could not get field %v (type %v) of %v: %v", f.Name, f.Type, t, err)
			}
		}

		dest.Field(i).Set(v)
	}
	return result, nil
}
