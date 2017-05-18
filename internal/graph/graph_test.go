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

package graph

import (
	"testing"

	"reflect"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Type1 struct {
	t int
}

type Type2 struct {
	s string
}

type Type3 struct {
	f float32
}

type S struct{}

func noReturn() {}

func returnNonPointer() S {
	return S{}
}

func nonPointerParams(one, two string) *S {
	return &S{}
}

func TestInsertObject(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	p1 := &Parent1{
		c1: &Child1{},
	}
	err := g.InsertObject(reflect.ValueOf(p1))
	require.NoError(t, err)

	var first *Parent1
	v, err := g.Read(reflect.TypeOf(first))
	require.NoError(t, err, "No error expected during first Resolve")
	require.NotNil(t, v)
}

func constructor(p1 *Parent1, p12 *Parent12) error {
	return nil
}

func TestResolvedArguments(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	p1 := &Parent1{
		c1: &Child1{},
	}
	p12 := &Parent12{
		c1: &Child1{},
	}
	err := g.InsertObject(reflect.ValueOf(p1))
	require.NoError(t, err)

	err = g.InsertObject(reflect.ValueOf(p12))
	require.NoError(t, err)

	values, err := g.ConstructorArguments(reflect.TypeOf(constructor))
	require.NoError(t, err)

	assert.Equal(t, reflect.TypeOf(values[0].Interface()).String(), "*graph.Parent1")
	assert.Equal(t, reflect.TypeOf(values[1].Interface()).String(), "*graph.Parent12")
}

func TestGraphString(t *testing.T) {
	g := NewGraph()
	p1 := &Parent1{
		c1: &Child1{},
	}
	p12 := &Parent12{
		c1: &Child1{},
	}
	err := g.InsertObject(reflect.ValueOf(p1))
	require.NoError(t, err)
	err = g.InsertObject(reflect.ValueOf(p12))
	require.NoError(t, err)

	require.Contains(t, g.String(), "*graph.Parent1 -> (object) obj: *graph.Parent1")
	require.Contains(t, g.String(), "*graph.Parent12 -> (object) obj: *graph.Parent12")
}

func TestCtorConflicts(t *testing.T) {
	t.Parallel()
	g := NewGraph()

	err := g.InsertConstructor(threeObjects)
	require.NoError(t, err)

	err = g.InsertConstructor(oneObject)
	require.Contains(t, err.Error(), "ctor: func() (*graph.Child1, error), object type: *graph.Child1: node already exist for the constructor")

	g.Reset()
	err = g.InsertConstructor(func() (*Child1, *Child1, error) {
		return &Child1{}, &Child1{}, nil
	})
	require.Contains(t, err.Error(), "ctor: func() (*graph.Child1, *graph.Child1, error), object type: *graph.Child1: node already exist for the constructor")
}

func TestCtorOverrideReturnsError(t *testing.T) {
	t.Parallel()
	g := NewGraph()

	err := g.InsertConstructor(threeObjects)
	require.NoError(t, err)
	err = g.validateCtorReturnTypes(reflect.TypeOf(oneObject))
	require.Contains(t, err.Error(), "ctor: func() (*graph.Child1, error), object type: *graph.Child1: node already exist for the constructor")
}

func TestInvokeOverrideReturnsError(t *testing.T) {
	t.Parallel()
	g := NewGraph()

	g.InsertObject(reflect.ValueOf(&Child1{}))
	err := g.ValidateInvokeReturnTypes(reflect.TypeOf(oneObject))
	require.Contains(t, err.Error(), "ctor: func() (*graph.Child1, error), object type: *graph.Child1: node already exist for the constructor")
}

func TestMultiObjectRegisterResolve(t *testing.T) {
	t.Parallel()
	g := NewGraph()

	err := g.InsertConstructor(threeObjects)
	require.NoError(t, err)

	var first *Child1
	v, err := g.Read(reflect.TypeOf(first))
	require.NoError(t, err, "No error expected during first Resolve")
	require.NotNil(t, v)

	var second *Child2
	v, err = g.Read(reflect.TypeOf(second))
	require.NoError(t, err, "No error expected during first Resolve")
	require.NotNil(t, v)

	var third *Child3
	v, err = g.Read(reflect.TypeOf(third))
	require.NoError(t, err, "No error expected during first Resolve")
	require.NotNil(t, v)

	var errRegistered *error
	v, err = g.Read(reflect.TypeOf(errRegistered))
	require.Error(t, err, "type *error shouldn't be registered")
	require.Nil(t, v.Interface())
}
