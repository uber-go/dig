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
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/dig/internal/graph"
)

var (
	errParamType   = errors.New("registration must be done through a pointer or a function")
	errReturnCount = errors.New("constructor function must one or two values")
	errReturnKind  = errors.New("constructor return type must be a pointer")
	errArgKind     = errors.New("constructor arguments must be pointers")

	_typeOfError = reflect.TypeOf((*error)(nil)).Elem()
)

// New returns a new DI Container
func New() *Container {
	return &Container{graph.NewGraph()}
}

// Container facilitates automated dependency resolution
type Container struct {
	graph.Graph
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
		args, err := c.Graph.ConstructorArguments(ctype)
		if err != nil {
			return err
		}
		cv := reflect.ValueOf(t)

		// execute the provided func
		values := cv.Call(args)

		if len(values) > 0 {
			if err, _ := values[len(values)-1].Interface().(error); err != nil {
				return errors.Wrapf(err, "Error executing the function %v", ctype)
			}
			for _, v := range values {
				switch v.Type().Kind() {
				case reflect.Slice, reflect.Array, reflect.Map, reflect.Ptr, reflect.Interface:
					c.Graph.InsertObject(v)
				default:
					return errors.Wrapf(errReturnKind, "%v", ctype)
				}
			}
		}
	default:
		return errParamType
	}
	return nil
}

// Provide an object in the Container
//
// The provided argument must be a function that accepts its dependencies as
// arguments and returns one or more results, which must be a pointer type, map, slice or an array.
// The function may optionally return an error as the last argument.
func (c *Container) Provide(t interface{}) error {
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
		return c.Graph.InsertConstructor(t)
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Ptr, reflect.Interface:
		v := reflect.ValueOf(t)
		if ctype.Elem().Kind() == reflect.Interface {
			ctype = ctype.Elem()
			v = v.Elem()
		}
		return c.Graph.InsertObject(v)
	default:
		return errParamType
	}
}

// Resolve all of the dependencies of the provided class
//
// Provided object must be a pointer, map, slice or an array
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

	v, err := c.Graph.Read(objElemType)
	if err != nil {
		return err
	}

	// set the pointer value of the provided object to the instance pointer
	objVal.Elem().Set(v)

	return nil
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

// ProvideAll registers all the provided args in the Container
func (c *Container) ProvideAll(types ...interface{}) error {
	for _, t := range types {
		if err := c.Provide(t); err != nil {
			return err
		}
	}
	return nil
}
