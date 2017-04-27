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
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
)

var (
	errParamType     = errors.New("registration must be done through a pointer or a function")
	errReturnCount   = errors.New("constructor function must one or two values")
	errReturnKind    = errors.New("constructor return type must be a pointer")
	errReturnErrKind = errors.New("second return value of constructor must be error")
	errArgKind       = errors.New("constructor arguments must be pointers")

	_typeOfError = reflect.TypeOf((*error)(nil)).Elem()
)

// New returns a new DI Container
func New() *Container {
	return &Container{
		nodes: make(map[interface{}]graphNode),
	}
}

// Container facilitates automated dependency resolution
type Container struct {
	sync.RWMutex

	nodes map[interface{}]graphNode
}

// Invoke the function and resolve the dependencies immidiately without providing the
// constructor to the graph. The Invoke function returns error object which can be
// occurred during the execution
// The return arguments from Invoked function are registered in the graph for later use
// The last parameter, if it is an error, is returned to the Invoke caller
func (c *Container) Invoke(t interface{}) error {
	ctype := reflect.TypeOf(t)
	switch ctype.Kind() {
	case reflect.Func:
		// find dependencies from the graph and place them in the args
		args := make([]reflect.Value, ctype.NumIn(), ctype.NumIn())
		for idx := range args {
			arg := ctype.In(idx)
			c.RLock()
			node, ok := c.nodes[arg]
			c.RUnlock()
			if ok {
				c.RLock()
				v, err := node.value(c, arg)
				c.RUnlock()
				if err != nil {
					return errors.Wrapf(err, "unable to resolve %v", arg)
				}
				args[idx] = v
			} else {
				return fmt.Errorf("%v dependency of type %v is not registered", ctype, arg)
			}
		}
		cv := reflect.ValueOf(t)

		// execute the provided func
		values := cv.Call(args)
		if len(values) > 0 {
			c.Lock()
			defer c.Unlock()
			err, _ := values[len(values)-1].Interface().(error)
			for _, v := range values {
				switch v.Type().Kind() {
				case reflect.Ptr:
					c.insertObjectToGraph(v, v.Type())
				default:
					return errors.Wrapf(errReturnKind, "%v", ctype)
				}
			}
			return err
		}
	default:
		return errParamType
	}
	return nil
}

// Provide an object in the Container
//
// The provided argument must be a function that accepts its dependencies as
// arguments and returns a single result, which must be a pointer type.
// The function may optionally return an error as a second result.
func (c *Container) Provide(t interface{}) error {
	c.Lock()
	defer c.Unlock()

	ctype := reflect.TypeOf(t)
	switch ctype.Kind() {
	case reflect.Func:
		switch ctype.NumOut() {
		case 0:
			return errReturnCount
		case 1:
			objType := ctype.Out(0)
			if objType.Kind() != reflect.Ptr && objType.Kind() != reflect.Interface {
				return errReturnKind
			}
		}
		return c.provideConstructor(t)
	case reflect.Ptr:
		return c.provideObject(t, ctype)
	default:
		return errParamType
	}
}

// MustRegister will attempt to Provide the object and panic if error is encountered
func (c *Container) MustRegister(t interface{}) {
	if err := c.Provide(t); err != nil {
		panic(err)
	}
}

// Resolve all of the dependencies of the provided class
//
// Provided object must be a pointer
// Any dependencies of the object will receive constructor calls, or be initialized (once)
// Constructor with return value *object will be called
func (c *Container) Resolve(obj interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during Resolve %v", r)
		}
	}()

	objType := reflect.TypeOf(obj)
	if objType.Kind() != reflect.Ptr {
		return fmt.Errorf("can not resolve non-pointer object of type %v", objType)
	}

	objElemType := reflect.TypeOf(obj).Elem()
	objVal := reflect.ValueOf(obj)

	c.Lock()
	defer c.Unlock()
	// check if the type is a registered objNode
	n, ok := c.nodes[objElemType]
	if !ok {
		return fmt.Errorf("type %v is not registered", objType)
	}

	v, err := n.value(c, objElemType)
	if err != nil {
		return errors.Wrapf(err, "unable to resolve %v", objType)
	}

	// set the pointer value of the provided object to the instance pointer
	objVal.Elem().Set(v)

	return nil
}

// MustResolve calls Resolve and panics if an error is encountered
func (c *Container) MustResolve(obj interface{}) {
	if err := c.Resolve(obj); err != nil {
		panic(err)
	}
}

// ResolveAll the dependencies of each provided object
// Returns the first error encountered
func (c *Container) ResolveAll(objs ...interface{}) error {
	for _, o := range objs {
		if err := c.Resolve(o); err != nil {
			return err
		}
	}
	return nil
}

// MustResolveAll calls ResolveAll and panics if an error is encountered
func (c *Container) MustResolveAll(objs ...interface{}) {
	if err := c.ResolveAll(objs...); err != nil {
		panic(err)
	}
}

// ProvideAll registers all the provided args in the Container
func (c *Container) ProvideAll(types ...interface{}) error {
	for _, t := range types {
		if err := c.Provide(t); err != nil {
			return err
		}
	}
	return nil
}

// MustProvideAll calls ProvideAll and panics is an error is encountered
func (c *Container) MustProvideAll(types ...interface{}) {
	if err := c.ProvideAll(types...); err != nil {
		panic(err)
	}
}

// Reset the graph by removing all the registered nodes
func (c *Container) Reset() {
	c.Lock()
	defer c.Unlock()

	c.nodes = make(map[interface{}]graphNode)
}

// String representation of the entire Container
func (c *Container) String() string {
	b := &bytes.Buffer{}
	fmt.Fprintln(b, "{nodes:")
	for key, reg := range c.nodes {
		fmt.Fprintln(b, key, "->", reg)
	}
	fmt.Fprintln(b, "}")
	return b.String()
}

func (c *Container) provideObject(o interface{}, otype reflect.Type) error {
	v := reflect.ValueOf(o)
	if otype.Elem().Kind() == reflect.Interface {
		otype = otype.Elem()
		v = v.Elem()
	}
	c.insertObjectToGraph(v, otype)
	return nil
}

func (c *Container) insertObjectToGraph(v reflect.Value, vtype reflect.Type) {
	n := objNode{
		node: node{
			objType:     vtype,
			cached:      true,
			cachedValue: v,
		},
	}
	c.nodes[vtype] = &n
}

// constr must be a function that returns the result type and an error
func (c *Container) provideConstructor(constr interface{}) error {
	ctype := reflect.TypeOf(constr)
	// count of number of objects to be registered from the list of return parameters
	count := ctype.NumOut()
	objTypes := make([]reflect.Type, count, count)
	for i := 0; i < count; i++ {
		objTypes[i] = ctype.Out(i)
	}

	nodes := make([]node, count, count)
	for i := 0; i < count; i++ {
		nodes[i] = node{
			objType: objTypes[i],
		}
	}
	argc := ctype.NumIn()
	n := funcNode{
		deps:        make([]interface{}, argc),
		constructor: constr,
		nodes:       nodes,
	}
	for i := 0; i < argc; i++ {
		arg := ctype.In(i)
		if arg.Kind() != reflect.Ptr && arg.Kind() != reflect.Interface {
			return errArgKind
		}
		n.deps[i] = arg
	}

	for i := 0; i < count; i++ {
		c.nodes[objTypes[i]] = &n
	}

	// object needs to be part of the container to properly detect cycles
	if cycleErr := c.detectCycles(&n); cycleErr != nil {
		// if the cycle was detected delete from the container
		for objType := range objTypes {
			delete(c.nodes, objType)
		}
		return errors.Wrapf(cycleErr, "unable to Provide %v", objTypes)
	}

	return nil
}

// When a new constructor is being inserted, detect any present cycles
func (c *Container) detectCycles(n *funcNode) error {
	l := []string{}
	return c.recursiveDetectCycles(n, l)
}

// DFS and tracking if same node is visited twice
func (c *Container) recursiveDetectCycles(n graphNode, l []string) error {
	for _, el := range l {
		if n.id() == el {
			b := &bytes.Buffer{}
			for _, curr := range l {
				fmt.Fprint(b, curr, " -> ")
			}
			fmt.Fprint(b, n.id())
			return fmt.Errorf("detected cycle %s", b.String())
		}
	}

	l = append(l, n.id())

	for _, dep := range n.dependencies() {
		if node, ok := c.nodes[dep]; ok {
			if err := c.recursiveDetectCycles(node, l); err != nil {
				return err
			}
		}
	}

	return nil
}
