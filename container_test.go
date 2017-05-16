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
	"errors"
	"reflect"
	"testing"

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

func TestRegister(t *testing.T) {
	t.Parallel()
	tts := []struct {
		name  string
		param interface{}
		err   string
	}{
		{"non function", struct{}{}, "for constructor struct {}: " + errParamType.Error()},
		{"wrong return count", noReturn, "for constructor func(): " + errReturnCount.Error()},
		{"non pointer return", returnNonPointer, "for constructor func() dig.S: " + errReturnKind.Error()},
		{"wrong parameters type", nonPointerParams, errArgKind.Error()},
		{"pointer Provide", &struct{}{}, ""},
	}

	for _, tc := range tts {
		t.Run(tc.name, func(t *testing.T) {
			c := New()
			err := c.Provide(tc.param)

			if len(tc.err) > 0 {
				require.Equal(t, err.Error(), tc.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()
	tts := []struct {
		name    string
		Provide func(c *Container) error
		resolve func(c *Container) error
		es      string
	}{
		{
			"non pointer resolve",
			func(c *Container) error { return c.Provide(NewParent1) },
			func(c *Container) error { return c.Resolve(S{}) },
			"can not resolve non-pointer object",
		},
		{
			"missing dependency",
			func(c *Container) error { return nil },
			func(c *Container) error {
				var p1 *Parent1
				return c.Resolve(&p1)
			},
			"type *dig.Parent1 is not registered",
		},
	}

	for _, tc := range tts {
		t.Run(tc.name, func(t *testing.T) {
			c := New()

			err := tc.Provide(c)
			require.NoError(t, err, "Provide part of the test cant have errors")

			err = tc.resolve(c)
			if tc.es != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.es, "Unexpected error message")
			} else {
				require.NoError(t, err, "Did not expect a resolve error")
			}
		})
	}
}

func TestComplexType(t *testing.T) {
	t.Parallel()
	tts := []struct {
		name    string
		Provide func(c *Container) error
		resolve func(t *testing.T, c *Container) error
		es      string
	}{
		{
			"test map",
			func(c *Container) error { return c.Provide(testmap) },
			func(t *testing.T, c *Container) error {
				err := c.Resolve(&resolvemap)
				assert.Equal(t, resolvemap["one"], 1)
				return err
			},
			"",
		},
		{
			"test slice",
			func(c *Container) error { return c.Provide(testslice) },
			func(t *testing.T, c *Container) error {
				err := c.Resolve(&resolveslice)
				assert.Equal(t, resolveslice[0], 1)
				return err
			},
			"",
		},
		{
			"test array",
			func(c *Container) error { return c.Provide(testarray) },
			func(t *testing.T, c *Container) error {
				err := c.Resolve(&resolvearray)
				assert.Equal(t, resolvearray[0], "one")
				return err
			},
			"",
		},
		{
			"test ctor",
			func(c *Container) error { return c.Provide(testslice, testmap, testarray, ctorWithMapsAndSlices) },
			func(t *testing.T, c *Container) error {
				err := c.Resolve(&resolveTestResult)
				assert.Equal(t, resolveTestResult.testarray[0], "one")
				assert.Equal(t, resolveTestResult.testmap["one"], 1)
				assert.Equal(t, resolveTestResult.testslice[0], 1)
				return err
			},
			"",
		},
		{
			"test invoke",
			func(c *Container) error { return c.Provide(testslice, testmap, testarray) },
			func(t *testing.T, c *Container) error {
				if err := c.Invoke(ctorWithMapsAndSlices); err != nil {
					return err
				}
				assert.Equal(t, resolveTestResult.testarray[0], "one")
				assert.Equal(t, resolveTestResult.testmap["one"], 1)
				assert.Equal(t, resolveTestResult.testslice[0], 1)
				return c.Resolve(&resolveTestResult)
			},
			"",
		},
	}

	for _, tc := range tts {
		t.Run(tc.name, func(t *testing.T) {
			c := New()

			err := tc.Provide(c)
			require.NoError(t, err, "Provide part of the test cant have errors")

			err = tc.resolve(t, c)
			if tc.es != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.es, "Unexpected error message")
			} else {
				require.NoError(t, err, "Did not expect a resolve error")
			}
		})
	}
}

func TestObjectRegister(t *testing.T) {
	t.Parallel()
	c := New()

	// Provide a fake struct type into the graph
	type Fake struct {
		Name string
	}
	err := c.Provide(&Fake{Name: "I am a fake registered thing"})
	require.NoError(t, err)

	// get one pointer resolved
	var f1 *Fake
	err = c.Resolve(&f1)
	require.NoError(t, err)

	// get second pointer resolved
	var f2 *Fake
	err = c.Resolve(&f2)
	require.NoError(t, err)

	// make sure they are the same
	require.Equal(t, f1, f2)
	require.Equal(t, *f1, *f2)
}

func TestBasicRegisterResolve(t *testing.T) {
	t.Parallel()
	c := New()

	err := c.Provide(NewGrandchild1)
	require.NoError(t, err)

	var first *Grandchild1
	require.NoError(t, c.Resolve(&first), "No error expected during first Resolve")

	var second *Grandchild1
	require.NoError(t, c.Resolve(&second), "No error expected during second Resolve")

	require.NotNil(t, first, "Child1 must have been registered")
	require.NotNil(t, second, "Child1 must have been registered")
	require.True(t, *first == *second, "Must point to the same object")
}

func TestInterfaceRegisterResolve(t *testing.T) {
	t.Parallel()
	c := New()

	var gc1 GrandchildInt1 = NewGrandchild1()
	err := c.Provide(&gc1)
	require.NoError(t, err)

	var registered1 GrandchildInt1
	require.NoError(t, c.Resolve(&registered1), "No error expected during Resolve")

	require.NotNil(t, registered1, "GrandchildInt1 must have been registered")
	require.True(t, gc1 == registered1, "Must point to the same object")

	var gc2 GrandchildInt2 = &Grandchild2{}
	err = c.Provide(&gc2)
	require.NoError(t, err)

	var registered2 GrandchildInt2
	require.NoError(t, c.Resolve(&registered2), "No error expected during Resolve")

	require.NotNil(t, registered2, "GrandchildInt2 must have been registered")
	require.True(t, gc2 == registered2, "Must point to the same object")

	err = c.Provide(NewChild3)
	require.NoError(t, err)

	var c3 *Child3
	require.NoError(t, c.Resolve(&c3), "No error expected during Resolve")

	require.NotNil(t, c3, "NewChild3 must have been registered")
	require.True(t, gc1 == c3.gci1, "Child grand childeren point to the same object")
	require.True(t, gc2 == c3.gci2, "Child grand childeren point to the same object")
}

func TestConstructorErrors(t *testing.T) {
	tests := []struct {
		desc      string
		registers []interface{}
		wantErr   string
	}{
		{
			desc: "success",
			registers: []interface{}{
				NewFlakyParent,
				NewFlakyChild,
			},
		},
		{
			desc: "failure",
			registers: []interface{}{
				NewFlakyParent,
				NewFlakyChildFailure,
			},
			wantErr: "unable to resolve *dig.FlakyParent: " +
				"unable to resolve *dig.FlakyChild: " +
				"great sadness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c := New()
			require.NoError(t, c.Provide(tt.registers...))
			var p1 *FlakyParent
			err := c.Resolve(&p1)
			if tt.wantErr != "" {
				var registeredError *error
				require.Error(t, c.Resolve(&registeredError), "type *error is not registered")
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInvokeSuccess(t *testing.T) {
	t.Parallel()
	c := New()

	err := c.Provide(
		NewParent1,
		NewChild1,
		NewGrandchild1,
	)
	assert.NoError(t, err)
	var c1 *Child1

	err = c.Invoke(func(p1 *Parent1) {
		require.NotNil(t, p1)
		c1 = p1.c1
	})
	assert.NoError(t, err)
	assert.NotNil(t, c1)
}

func TestInvokeAndRegisterSuccess(t *testing.T) {
	t.Parallel()
	c := New()
	child := &Child1{}
	err := c.Provide(
		&Parent1{
			c1: child,
		},
	)
	assert.NoError(t, err)
	err = c.Invoke(func(p1 *Parent1) *Child1 {
		require.NotNil(t, p1)
		return p1.c1
	})
	assert.NoError(t, err)

	value, err := c.Graph.Read(reflect.TypeOf(child))
	assert.NoError(t, err)
	assert.NotNil(t, value)
}

func TestInvokeAndRegisterFailure(t *testing.T) {
	t.Parallel()
	c := New()
	child := &Child1{}
	err := c.Provide(
		&Parent1{
			c1: child,
		},
	)
	assert.NoError(t, err)
	err = c.Invoke(func(p1 *Parent1) Child1 {
		require.NotNil(t, p1)
		return *p1.c1
	})
	require.Contains(t, err.Error(), "constructor return type must be a pointer")
}

func TestInvokeReturnedError(t *testing.T) {
	t.Parallel()
	c := New()
	err := c.Invoke(func() error {
		return errors.New("oops")
	})
	var registeredError *error
	require.Error(t, c.Resolve(&registeredError), "type *error is not registered")
	require.Contains(t, err.Error(), "error invoking the function func() error: oops")

	err = c.Invoke(func() (*Child1, error) {
		return &Child1{}, nil
	})
	assert.NoError(t, err)
}

func TestInvokeFailureUnresolvedDependencies(t *testing.T) {
	t.Parallel()
	c := New()

	err := c.Provide(
		NewParent1,
	)
	assert.NoError(t, err)

	err = c.Invoke(func(p1 *Parent1) {})
	require.Contains(t, err.Error(), "unable to resolve *dig.Parent1")

	err = c.Invoke(func(p12 *Parent12) {})
	require.Contains(t, err.Error(), "dependency of type *dig.Parent12 is not registered")
}

func TestProvide(t *testing.T) {
	t.Parallel()
	c := New()

	err := c.Provide(
		NewParent1,
		NewChild1,
		NewGrandchild1,
	)
	require.NoError(t, err)

	var p1 *Parent1
	err = c.Resolve(&p1)

	require.NoError(t, err, "No error expected during Resolve")
	require.NotNil(t, p1.c1, "Child1 must have been registered")
	require.NotNil(t, p1.c1.gc1, "Grandchild1 must have been registered")
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	c := New()

	for i := 0; i < 10; i++ {
		go func() {
			require.NoError(t, c.Provide(NewGrandchild1))

			var gc1 *Grandchild1
			require.NoError(t, c.Resolve(&gc1))
		}()
	}
}

func TestCycles(t *testing.T) {
	t.Parallel()
	c := New()

	//    Type1
	//    /    \
	// Type2  Type 3
	//   /       \
	// Type4    Type 1
	type Type1 interface{}
	type Type2 interface{}
	type Type3 interface{}
	type Type4 interface{}
	c1 := func(t2 Type2, t3 Type3) Type1 { return nil }
	c2 := func(t4 Type4) Type2 { return nil }
	c3 := func(t3 Type1) Type3 { return nil }

	require.NoError(t, c.Provide(c1))
	require.NoError(t, c.Provide(c2))
	err := c.Provide(c3)
	require.Contains(t, err.Error(), "unable to Provide [dig.Type3]")
	require.Contains(t, err.Error(), "func(dig.Type1) dig.Type3 -> func(dig.Type2, dig.Type3) dig.Type1 -> func(dig.Type1) dig.Type3")
}

func TestCyclesWithConstructor(t *testing.T) {
	t.Parallel()
	c := New()

	//    Type1
	//      |
	//  func() Type3, Type4
	//   /       \
	// Type1    Type2
	type Type1 interface{}
	type Type2 interface{}
	type Type3 interface{}
	type Type4 interface{}
	c1 := func(t1 Type1, t2 Type2) (Type3, Type4) { return nil, nil }
	c2 := func(t4 Type4) Type1 { return nil }

	require.NoError(t, c.Provide(c1))
	err := c.Provide(c2)
	require.Contains(t, err.Error(), "unable to Provide [dig.Type1]")
	require.Contains(t, err.Error(), "unable to Provide [dig.Type1]: detected cycle func(dig.Type4) dig.Type1 -> func(dig.Type1, dig.Type2) (dig.Type3, dig.Type4) -> func(dig.Type4) dig.Type1")
}

func TestResolveAll(t *testing.T) {
	t.Parallel()
	c := New()

	err := c.Provide(
		NewGrandchild1,
		NewChild1,
		NewParent1,
	)
	require.NoError(t, err)
	var p1 *Parent1
	var p2 *Parent1
	var p3 *Parent1
	var p4 *Parent1

	err = c.Resolve(&p1, &p2, &p3, &p4)
	require.NoError(t, err, "Did not expect error on resolve all")
	require.Equal(t, p1.name, "Parent1")
	require.True(t, p1 == p2 && p2 == p3 && p3 == p4, "All pointers must be equal")
}

func TestEmptyAfterReset(t *testing.T) {
	t.Parallel()
	c := New()

	require.NoError(t, c.Provide(NewGrandchild1))

	var first *Grandchild1
	require.NoError(t, c.Resolve(&first), "No error expected during first Resolve")
	c.Reset()
	require.Contains(t, c.Resolve(&first).Error(), "not registered")
}

func TestPanicConstructor(t *testing.T) {
	t.Parallel()
	c := New()

	type Type1 struct{}
	ty := func() *Type1 {
		panic("RUH ROH")
	}

	require.NoError(t, c.Provide(ty))

	var v *Type1
	err := c.Resolve(&v)
	require.Contains(t, err.Error(), "panic during Resolve")
	require.Contains(t, err.Error(), "RUH ROH")
}

func TestMultiObjectRegisterResolve(t *testing.T) {
	t.Parallel()
	c := New()

	err := c.Provide(threeObjects)
	require.NoError(t, err)

	var first *Child1
	require.NoError(t, c.Resolve(&first), "No error expected during first Resolve")

	var second *Child2
	require.NoError(t, c.Resolve(&second), "No error expected during first Resolve")

	var third *Child3
	require.NoError(t, c.Resolve(&third), "No error expected during first Resolve")

	var errRegistered *error
	require.Error(t, c.Resolve(&errRegistered), "type *error shouldn't be registered")
	require.Nil(t, errRegistered)

	require.NotNil(t, first, "Child1 must have been registered")
	require.NotNil(t, second, "Child2 must have been registered")
	require.NotNil(t, third, "Child3 must have been registered")
}
