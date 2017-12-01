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
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndSuccess(t *testing.T) {
	t.Parallel()

	t.Run("pointer constructor", func(t *testing.T) {
		c := New()
		var b *bytes.Buffer
		require.NoError(t, c.Provide(func() *bytes.Buffer {
			b = &bytes.Buffer{}
			return b
		}), "provide failed")
		require.NoError(t, c.Invoke(func(got *bytes.Buffer) {
			require.NotNil(t, got, "invoke got nil buffer")
			require.True(t, got == b, "invoke got wrong buffer")
		}), "invoke failed")
	})

	t.Run("nil pointer constructor", func(t *testing.T) {
		// Dig shouldn't forbid this - it's perfectly reasonable to explicitly
		// provide a typed nil, since that's often a convenient way to supply a
		// default no-op implementation.
		c := New()
		require.NoError(t, c.Provide(func() *bytes.Buffer { return nil }), "provide failed")
		require.NoError(t, c.Invoke(func(b *bytes.Buffer) {
			require.Nil(t, b, "expected to get nil buffer")
		}), "invoke failed")
	})

	t.Run("struct constructor", func(t *testing.T) {
		c := New()
		var buf bytes.Buffer
		buf.WriteString("foo")
		require.NoError(t, c.Provide(func() bytes.Buffer { return buf }), "provide failed")
		require.NoError(t, c.Invoke(func(b bytes.Buffer) {
			// ensure we're getting back the buffer we put in
			require.Equal(t, "foo", buf.String(), "invoke got new buffer")
		}), "invoke failed")
	})

	t.Run("slice constructor", func(t *testing.T) {
		c := New()
		b1 := &bytes.Buffer{}
		b2 := &bytes.Buffer{}
		require.NoError(t, c.Provide(func() []*bytes.Buffer {
			return []*bytes.Buffer{b1, b2}
		}), "provide failed")
		require.NoError(t, c.Invoke(func(bs []*bytes.Buffer) {
			require.Equal(t, 2, len(bs), "invoke got unexpected number of buffers")
			require.True(t, b1 == bs[0], "first item did not match")
			require.True(t, b2 == bs[1], "second item did not match")
		}), "invoke failed")
	})

	t.Run("array constructor", func(t *testing.T) {
		c := New()
		bufs := [1]*bytes.Buffer{{}}
		require.NoError(t, c.Provide(func() [1]*bytes.Buffer { return bufs }), "provide failed")
		require.NoError(t, c.Invoke(func(bs [1]*bytes.Buffer) {
			require.NotNil(t, bs[0], "invoke got new array")
		}), "invoke failed")
	})

	t.Run("map constructor", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() map[string]string {
			return map[string]string{}
		}), "provide failed")
		require.NoError(t, c.Invoke(func(m map[string]string) {
			require.NotNil(t, m, "invoke got zero value map")
		}), "invoke failed")
	})

	t.Run("channel constructor", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() chan int {
			return make(chan int)
		}), "provide failed")
		require.NoError(t, c.Invoke(func(ch chan int) {
			require.NotNil(t, ch, "invoke got nil chan")
		}), "invoke failed")
	})

	t.Run("func constructor", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() func(int) {
			return func(int) {}
		}), "provide failed")
		require.NoError(t, c.Invoke(func(f func(int)) {
			require.NotNil(t, f, "invoke got nil function pointer")
		}), "invoke failed")
	})

	t.Run("interface constructor", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() io.Writer {
			return &bytes.Buffer{}
		}), "provide failed")
		require.NoError(t, c.Invoke(func(w io.Writer) {
			require.NotNil(t, w, "invoke got nil interface")
		}), "invoke failed")
	})

	t.Run("param", func(t *testing.T) {
		c := New()
		type contents string
		type Args struct {
			In

			Contents contents
		}

		require.NoError(t,
			c.Provide(func(args Args) *bytes.Buffer {
				require.NotEmpty(t, args.Contents, "contents must not be empty")
				return bytes.NewBufferString(string(args.Contents))
			}), "provide constructor failed")

		require.NoError(t,
			c.Provide(func() contents { return "hello world" }),
			"provide value failed")

		require.NoError(t, c.Invoke(func(buff *bytes.Buffer) {
			out, err := ioutil.ReadAll(buff)
			require.NoError(t, err, "read from buffer failed")
			require.Equal(t, "hello world", string(out), "contents don't match")
		}))
	})

	t.Run("invoke param", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() *bytes.Buffer {
			return new(bytes.Buffer)
		}), "provide failed")

		type Args struct {
			In

			*bytes.Buffer
		}

		require.NoError(t, c.Invoke(func(args Args) {
			require.NotNil(t, args.Buffer, "invoke got nil buffer")
		}))
	})

	t.Run("param wrapper", func(t *testing.T) {
		var (
			buff   *bytes.Buffer
			called bool
		)

		c := New()
		require.NoError(t, c.Provide(func() *bytes.Buffer {
			require.False(t, called, "constructor must be called exactly once")
			called = true
			buff = new(bytes.Buffer)
			return buff
		}), "provide failed")

		type MyParam struct{ In }

		type Args struct {
			MyParam

			Buffer *bytes.Buffer
		}

		require.NoError(t, c.Invoke(func(args Args) {
			require.True(t, called, "constructor must be called first")
			require.NotNil(t, args.Buffer, "invoke got nil buffer")
			require.True(t, args.Buffer == buff, "buffer must match constructor's return value")
		}))
	})

	t.Run("param recurse", func(t *testing.T) {
		type anotherParam struct {
			In

			Buffer *bytes.Buffer
		}

		type someParam struct {
			In

			Buffer  *bytes.Buffer
			Another anotherParam
		}

		var (
			buff   *bytes.Buffer
			called bool
		)

		c := New()
		require.NoError(t, c.Provide(func() *bytes.Buffer {
			require.False(t, called, "constructor must be called exactly once")
			called = true
			buff = new(bytes.Buffer)
			return buff
		}), "provide must not fail")

		require.NoError(t, c.Invoke(func(p someParam) {
			require.True(t, called, "constructor must be called first")

			require.NotNil(t, p.Buffer, "someParam.Buffer must not be nil")
			require.NotNil(t, p.Another.Buffer, "anotherParam.Buffer must not be nil")

			require.True(t, p.Buffer == p.Another.Buffer, "buffers fields must match")
			require.True(t, p.Buffer == buff, "buffer must match constructor's return value")
		}), "invoke must not fail")
	})

	t.Run("multiple-type constructor", func(t *testing.T) {
		c := New()
		constructor := func() (*bytes.Buffer, []int, error) {
			return &bytes.Buffer{}, []int{42}, nil
		}
		consumer := func(b *bytes.Buffer, nums []int) {
			assert.NotNil(t, b, "invoke got nil buffer")
			assert.Equal(t, 1, len(nums), "invoke got empty slice")
		}
		require.NoError(t, c.Provide(constructor), "provide failed")
		require.NoError(t, c.Invoke(consumer), "invoke failed")
	})

	t.Run("multiple-type constructor is called once", func(t *testing.T) {
		c := New()
		type A struct{}
		type B struct{}
		count := 0
		constructor := func() (*A, *B, error) {
			count++
			return &A{}, &B{}, nil
		}
		getA := func(a *A) {
			assert.NotNil(t, a, "got nil A")
		}
		getB := func(b *B) {
			assert.NotNil(t, b, "got nil B")
		}
		require.NoError(t, c.Provide(constructor), "provide failed")
		require.NoError(t, c.Invoke(getA), "A invoke failed")
		require.NoError(t, c.Invoke(getB), "B invoke failed")
		require.NoError(t, c.Invoke(func(a *A, b *B) {}), "AB invoke failed")
		require.Equal(t, 1, count, "Constructor must be called once")
	})

	t.Run("method invocation inside Invoke", func(t *testing.T) {
		c := New()
		type A struct{}
		type B struct{}
		cA := func() (*A, error) {
			return &A{}, nil
		}
		cB := func() (*B, error) {
			return &B{}, nil
		}
		getA := func(a *A) {
			c.Invoke(func(b *B) {
				assert.NotNil(t, b, "got nil B")
			})
			assert.NotNil(t, a, "got nil A")
		}

		require.NoError(t, c.Provide(cA), "provide failed")
		require.NoError(t, c.Provide(cB), "provide failed")
		require.NoError(t, c.Invoke(getA), "A invoke failed")
	})

	t.Run("collections and instances of same type", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() []*bytes.Buffer {
			return []*bytes.Buffer{{}}
		}), "providing collection failed")
		require.NoError(t, c.Provide(func() *bytes.Buffer {
			return &bytes.Buffer{}
		}), "providing pointer failed")
	})

	t.Run("optional param field", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		type type4 struct{}
		type type5 struct{}
		constructor := func() (*type1, *type3, *type4) {
			return &type1{}, &type3{}, &type4{}
		}

		c := New()
		type param struct {
			In

			T1 *type1 // regular 'ol type
			T2 *type2 `optional:"true" useless_tag:"false"` // optional type NOT in the graph
			T3 *type3 `unrelated:"foo=42, optional"`        // type in the graph with unrelated tag
			T4 *type4 `optional:"true"`                     // optional type present in the graph
			T5 *type5 `optional:"t"`                        // optional type NOT in the graph with "yes"
		}
		require.NoError(t, c.Provide(constructor))
		require.NoError(t, c.Invoke(func(p param) {
			require.NotNil(t, p.T1, "whole param struct should not be nil")
			assert.Nil(t, p.T2, "optional type not in the graph should return nil")
			assert.NotNil(t, p.T3, "required type with unrelated tag not in the graph")
			assert.NotNil(t, p.T4, "optional type in the graph should not return nil")
			assert.Nil(t, p.T5, "optional type not in the graph should return nil")
		}))
	})

	t.Run("out type inserts multiple objects into the graph", func(t *testing.T) {
		type A struct{ name string }
		type B struct{ name string }
		type Ret struct {
			Out
			A  // value type A
			*B // pointer type *B
		}
		myA := A{"string A"}
		myB := &B{"string B"}

		c := New()
		require.NoError(t, c.Provide(func() Ret {
			return Ret{A: myA, B: myB}
		}), "provide for the Ret struct should succeed")
		require.NoError(t, c.Invoke(func(a A, b *B) {
			assert.Equal(t, a.name, "string A", "value type should work for dig.Out")
			assert.Equal(t, b.name, "string B", "pointer should work for dig.Out")
			assert.True(t, myA == a, "should get the same pointer for &A")
			assert.Equal(t, b, myB, "b and myB should be uqual")
		}))
	})

	t.Run("constructor with optional", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}

		type param struct {
			In

			T1 *type1 `optional:"true"`
		}

		c := New()

		var gave *type2
		require.NoError(t, c.Provide(func(p param) *type2 {
			require.Nil(t, p.T1, "T1 must be nil")
			gave = &type2{}
			return gave
		}), "provide failed")

		require.NoError(t, c.Invoke(func(got *type2) {
			require.True(t, got == gave, "type2 reference must be the same")
		}), "invoke failed")
	})

	t.Run("nested dependencies", func(t *testing.T) {
		c := New()

		type A struct{ name string }
		type B struct{ name string }
		type C struct{ name string }

		require.NoError(t, c.Provide(func() A { return A{"->A"} }))
		require.NoError(t, c.Provide(func(A) B { return B{"A->B"} }))
		require.NoError(t, c.Provide(func(A, B) C { return C{"AB->C"} }))
		require.NoError(t, c.Invoke(func(a A, b B, c C) {
			assert.Equal(t, a, A{"->A"})
			assert.Equal(t, b, B{"A->B"})
			assert.Equal(t, c, C{"AB->C"})
		}), "invoking should succeed")
	})

	t.Run("primitives", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() string { return "piper" }), "string provide failed")
		require.NoError(t, c.Provide(func() int { return 42 }), "int provide failed")
		require.NoError(t, c.Provide(func() int64 { return 24 }), "int provide failed")
		require.NoError(t, c.Provide(func() time.Duration {
			return 10 * time.Second
		}), "time.Duration provide failed")
		require.NoError(t, c.Invoke(func(i64 int64, i int, s string, d time.Duration) {
			assert.Equal(t, 42, i)
			assert.Equal(t, int64(24), i64)
			assert.Equal(t, "piper", s)
			assert.Equal(t, 10*time.Second, d)
		}))
	})

	t.Run("out types recurse", func(t *testing.T) {
		type A struct{}
		type B struct{}
		type C struct{}
		// Contains A
		type Ret1 struct {
			Out
			*A
		}
		// Contains *A (through Ret1), *B and C
		type Ret2 struct {
			Ret1
			*B
			C
		}
		c := New()

		require.NoError(t, c.Provide(func() Ret2 {
			return Ret2{
				Ret1: Ret1{
					A: &A{},
				},
				B: &B{},
				C: C{},
			}
		}), "provide for the Ret struct should succeed")
		require.NoError(t, c.Invoke(func(a *A, b *B, c C) {
			require.NotNil(t, a, "*A should be part of the container through Ret2->Ret1")
		}))
	})

	t.Run("named instances", func(t *testing.T) {
		c := New()
		type A struct{ idx int }

		// returns three named instances of A
		type ret struct {
			Out

			A1 A `name:"first"`
			A2 A `name:"second"`
			A3 A `name:"third"`
		}

		// requires two specific named instances
		type param struct {
			In

			A1 A `name:"first"`
			A3 A `name:"third"`
		}
		require.NoError(t, c.Provide(func() ret {
			return ret{A1: A{1}, A2: A{2}, A3: A{3}}
		}), "provide for three named instances should succeed")
		require.NoError(t, c.Invoke(func(p param) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 3, p.A3.idx)
		}), "invoke should succeed, pulling out two named instances")
	})

	t.Run("named and unnamed instances coexist", func(t *testing.T) {
		c := New()
		type A struct{ idx int }

		type out struct {
			Out

			A `name:"foo"`
		}

		require.NoError(t, c.Provide(func() out { return out{A: A{1}} }))
		require.NoError(t, c.Provide(func() A { return A{2} }))

		type in struct {
			In

			A1 A `name:"foo"`
			A2 A
		}
		require.NoError(t, c.Invoke(func(i in) {
			assert.Equal(t, 1, i.A1.idx)
			assert.Equal(t, 2, i.A2.idx)
		}))
	})

	t.Run("named instances recurse", func(t *testing.T) {
		c := New()
		type A struct{ idx int }

		type Ret1 struct {
			Out

			A1 A `name:"first"`
		}
		type Ret2 struct {
			Ret1

			A2 A `name:"second"`
		}
		type param struct {
			In

			A1 A `name:"first"`  // should come from ret1 through ret2
			A2 A `name:"second"` // should come from ret2
		}
		require.NoError(t, c.Provide(func() Ret2 {
			return Ret2{
				Ret1: Ret1{
					A1: A{1},
				},
				A2: A{2},
			}
		}))
		require.NoError(t, c.Invoke(func(p param) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 2, p.A2.idx)
		}), "invoke should succeed, pulling out two named instances")
	})

	t.Run("named instances do not cause cycles", func(t *testing.T) {
		c := New()
		type A struct{ idx int }
		type param struct {
			In
			A `name:"uno"`
		}
		type paramBoth struct {
			In

			A1 A `name:"uno"`
			A2 A `name:"dos"`
		}
		type retUno struct {
			Out
			A `name:"uno"`
		}
		type retDos struct {
			Out
			A `name:"dos"`
		}

		require.NoError(t, c.Provide(func() retUno {
			return retUno{A: A{1}}
		}), `should be able to provide A[name="uno"]`)
		require.NoError(t, c.Provide(func(p param) retDos {
			return retDos{A: A{2}}
		}), `A[name="dos"] should be able to rely on A[name="uno"]`)
		require.NoError(t, c.Invoke(func(p paramBoth) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 2, p.A2.idx)
		}), "both objects should be successfully resolved on Invoke")
	})

	t.Run("invoke on a type that depends on named parameters", func(t *testing.T) {
		c := New()
		type A struct{ idx int }
		type B struct{ sum int }
		type param struct {
			In

			A1 *A `name:"foo"`
			A2 *A `name:"bar"`
			A3 *A `name:"baz" optional:"true"`
		}
		type ret struct {
			Out

			A1 *A `name:"foo"`
			A2 *A `name:"bar"`
		}
		require.NoError(t, c.Provide(func() (ret, error) {
			return ret{
				A1: &A{1},
				A2: &A{2},
			}, nil
		}), "should be able to provide A1 and A2 into the graph")
		require.NoError(t, c.Provide(func(p param) *B {
			return &B{sum: p.A1.idx + p.A2.idx}
		}), "should be able to provide *B that relies on two named types")
		require.NoError(t, c.Invoke(func(b *B) {
			require.Equal(t, 3, b.sum)
		}))
	})

	t.Run("dynamically generated dig.In", func(t *testing.T) {
		// This test verifies that a dig.In generated using reflect.StructOf
		// works with our dig.In detection logic.
		c := New()

		type type1 struct{}
		type type2 struct{}

		var gave *type1
		new1 := func() *type1 {
			require.Nil(t, gave, "constructor must be called only once")
			gave = &type1{}
			return gave
		}

		require.NoError(t, c.Provide(new1), "failed to provide constructor")

		// We generate a struct that embeds dig.In.
		//
		// Note that the fix for https://github.com/golang/go/issues/18780
		// requires that StructField.Name is always set but versions of Go
		// older than 1.9 expect Name to be empty for embedded fields.
		//
		// We use utils_for_go19_test and utils_for_pre_go19_test with build
		// tags to implement this behavior differently in the two Go versions.

		inType := reflect.StructOf([]reflect.StructField{
			anonymousField(reflect.TypeOf(In{})),
			{
				Name: "Foo",
				Type: reflect.TypeOf(&type1{}),
			},
			{
				Name: "Bar",
				Type: reflect.TypeOf(&type2{}),
				Tag:  `optional:"true"`,
			},
		})

		// We generate a function that relies on that struct and validates the
		// result.
		fn := reflect.MakeFunc(
			reflect.FuncOf([]reflect.Type{inType}, nil /* returns */, false /* variadic */),
			func(args []reflect.Value) []reflect.Value {
				require.Len(t, args, 1, "expected only one argument")
				require.Equal(t, reflect.Struct, args[0].Kind(), "argument must be a struct")
				require.Equal(t, 3, args[0].NumField(), "struct must have two fields")

				t1, ok := args[0].Field(1).Interface().(*type1)
				require.True(t, ok, "field must be a type1")
				require.NotNil(t, t1, "value must not be nil")
				require.True(t, t1 == gave, "value must match constructor's return value")

				require.True(t, args[0].Field(2).IsNil(), "type2 must be nil")
				return nil
			},
		)

		require.NoError(t, c.Invoke(fn.Interface()), "invoke failed")
	})

	t.Run("dynamically generated dig.Out", func(t *testing.T) {
		// This test verifies that a dig.Out generated using reflect.StructOf
		// works with our dig.Out detection logic.

		c := New()

		type A struct{ Value int }

		outType := reflect.StructOf([]reflect.StructField{
			anonymousField(reflect.TypeOf(Out{})),
			{
				Name: "Foo",
				Type: reflect.TypeOf(&A{}),
				Tag:  `name:"foo"`,
			},
			{
				Name: "Bar",
				Type: reflect.TypeOf(&A{}),
				Tag:  `name:"bar"`,
			},
		})

		fn := reflect.MakeFunc(
			reflect.FuncOf(nil /* params */, []reflect.Type{outType}, false /* variadic */),
			func([]reflect.Value) []reflect.Value {
				result := reflect.New(outType).Elem()
				result.Field(1).Set(reflect.ValueOf(&A{Value: 1}))
				result.Field(2).Set(reflect.ValueOf(&A{Value: 2}))
				return []reflect.Value{result}
			},
		)
		require.NoError(t, c.Provide(fn.Interface()), "provide failed")

		type params struct {
			In

			Foo *A `name:"foo"`
			Bar *A `name:"bar"`
			Baz *A `name:"baz" optional:"true"`
		}

		require.NoError(t, c.Invoke(func(p params) {
			assert.Equal(t, &A{Value: 1}, p.Foo, "Foo must match")
			assert.Equal(t, &A{Value: 2}, p.Bar, "Bar must match")
			assert.Nil(t, p.Baz, "Baz must be unset")
		}), "invoke failed")
	})

	t.Run("variadic arguments invoke", func(t *testing.T) {
		c := New()

		type A struct{}

		var gaveA *A
		require.NoError(t, c.Provide(func() *A {
			gaveA = &A{}
			return gaveA
		}), "failed to provide A")

		require.NoError(t, c.Provide(func() []*A {
			panic("[]*A constructor must not be called.")
		}), "failed to provide A slice")

		require.NoError(t, c.Invoke(func(a *A, as ...*A) {
			require.NotNil(t, a, "A must not be nil")
			require.True(t, a == gaveA, "A must match")
			require.Empty(t, as, "varargs must be empty")
		}), "failed to invoke")
	})

	t.Run("variadic arguments dependency", func(t *testing.T) {
		c := New()

		type A struct{}
		type B struct{}

		var gaveA *A
		require.NoError(t, c.Provide(func() *A {
			gaveA = &A{}
			return gaveA
		}), "failed to provide A")

		require.NoError(t, c.Provide(func() []*A {
			panic("[]*A constructor must not be called.")
		}), "failed to provide A slice")

		var gaveB *B
		require.NoError(t, c.Provide(func(a *A, as ...*A) *B {
			require.NotNil(t, a, "A must not be nil")
			require.True(t, a == gaveA, "A must match")
			require.Empty(t, as, "varargs must be empty")
			gaveB = &B{}
			return gaveB
		}), "failed to provide B")

		require.NoError(t, c.Invoke(func(b *B) {
			require.NotNil(t, b, "B must not be nil")
			require.True(t, b == gaveB, "B must match")
		}), "failed to invoke")
	})

	t.Run("non-error return arguments from invoke are ignored", func(t *testing.T) {
		c := New()
		type A struct{}
		type B struct{}

		require.NoError(t, c.Provide(func() A { return A{} }))
		require.NoError(t, c.Invoke(func(A) B { return B{} }))

		err := c.Invoke(func(B) {})
		require.Error(t, err, "invoking with B param should error out")
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestEndToEndSuccess.func\S+ \(\S+/src/go.uber.org/dig/dig_test.go:\d+\):`,
			"type dig.B is not in the container,",
			"did you mean to Provide it?",
		)
	})
}

func TestGroups(t *testing.T) {
	t.Run("empty slice received without provides", func(t *testing.T) {
		c := New()

		type in struct {
			In

			Values []int `group:"foo"`
		}

		require.NoError(t, c.Invoke(func(i in) {
			require.Empty(t, i.Values)
		}), "failed to invoke")
	})

	t.Run("values are provided", func(t *testing.T) {
		c := New(setRand(rand.New(rand.NewSource(0))))

		type out struct {
			Out

			Value int `group:"val"`
		}

		provide := func(i int) {
			require.NoError(t, c.Provide(func() out {
				return out{Value: i}
			}), "failed to provide ")
		}

		provide(1)
		provide(2)
		provide(3)

		type in struct {
			In

			Values []int `group:"val"`
		}

		require.NoError(t, c.Invoke(func(i in) {
			assert.Equal(t, []int{2, 3, 1}, i.Values)
		}), "invoke failed")
	})

	t.Run("constructor is called at most once", func(t *testing.T) {
		c := New(setRand(rand.New(rand.NewSource(0))))

		type out struct {
			Out

			Result string `group:"s"`
		}

		calls := make(map[string]int)

		provide := func(i string) {
			require.NoError(t, c.Provide(func() out {
				calls[i]++
				return out{Result: i}
			}), "failed to provide")
		}

		provide("foo")
		provide("bar")
		provide("baz")

		type in struct {
			In

			Results []string `group:"s"`
		}

		// Expected value of in.Results in consecutive calls.
		expected := [][]string{
			{"bar", "baz", "foo"},
			{"foo", "baz", "bar"},
			{"baz", "bar", "foo"},
			{"bar", "foo", "baz"},
		}

		for i, want := range expected {
			require.NoError(t, c.Invoke(func(i in) {
				require.Equal(t, want, i.Results)
			}), "invoke %d failed", i)
		}

		for s, v := range calls {
			assert.Equal(t, 1, v, "constructor for %q called too many times", s)
		}
	})

	t.Run("consume groups in constructor", func(t *testing.T) {
		c := New(setRand(rand.New(rand.NewSource(0))))

		type out struct {
			Out

			Result []string `group:"hi"`
		}

		provideStrings := func(strings ...string) {
			require.NoError(t, c.Provide(func() out {
				return out{Result: strings}
			}), "failed to provide")
		}

		provideStrings("1", "2")
		provideStrings("3", "4", "5")
		provideStrings("6")
		provideStrings("7", "8", "9", "10")

		type setParams struct {
			In

			Strings [][]string `group:"hi"`
		}
		require.NoError(t, c.Provide(func(p setParams) map[string]struct{} {
			m := make(map[string]struct{})
			for _, ss := range p.Strings {
				for _, s := range ss {
					m[s] = struct{}{}
				}
			}
			return m
		}), "failed to provide set constructor")

		require.NoError(t, c.Invoke(func(got map[string]struct{}) {
			assert.Equal(t, map[string]struct{}{
				"1": {}, "2": {}, "3": {}, "4": {}, "5": {},
				"6": {}, "7": {}, "8": {}, "9": {}, "10": {},
			}, got)
		}), "failed to invoke")
	})

	t.Run("provide multiple values", func(t *testing.T) {
		c := New(setRand(rand.New(rand.NewSource(0))))

		type outInt struct {
			Out
			Int int `group:"foo"`
		}

		provideInt := func(i int) {
			require.NoError(t, c.Provide(func() (outInt, error) {
				return outInt{Int: i}, nil
			}), "failed to provide int")
		}

		type outString struct {
			Out
			String string `group:"foo"`
		}

		provideString := func(s string) {
			require.NoError(t, c.Provide(func() outString {
				return outString{String: s}
			}), "failed to provide string")
		}

		type outBoth struct {
			Out

			Int    int    `group:"foo"`
			String string `group:"foo"`
		}

		provideBoth := func(i int, s string) {
			require.NoError(t, c.Provide(func() (outBoth, error) {
				return outBoth{Int: i, String: s}, nil
			}), "failed to provide both")
		}

		provideInt(1)
		provideString("foo")
		provideBoth(2, "bar")
		provideString("baz")
		provideInt(3)
		provideBoth(4, "qux")
		provideBoth(5, "quux")
		provideInt(6)
		provideInt(7)

		type in struct {
			In

			Ints    []int    `group:"foo"`
			Strings []string `group:"foo"`
		}

		require.NoError(t, c.Invoke(func(got in) {
			assert.Equal(t, in{
				Ints:    []int{5, 3, 4, 1, 6, 7, 2},
				Strings: []string{"foo", "bar", "baz", "quux", "qux"},
			}, got)
		}), "failed to invoke")
	})

	t.Run("duplicate values are supported", func(t *testing.T) {
		c := New(setRand(rand.New(rand.NewSource(0))))

		type out struct {
			Out

			Result string `group:"s"`
		}

		provide := func(i string) {
			require.NoError(t, c.Provide(func() out {
				return out{Result: i}
			}), "failed to provide")
		}

		provide("a")
		provide("b")
		provide("c")
		provide("a")
		provide("d")
		provide("d")
		provide("a")
		provide("e")

		type stringSlice []string

		type in struct {
			In

			Strings stringSlice `group:"s"`
		}

		require.NoError(t, c.Invoke(func(i in) {
			assert.Equal(t,
				stringSlice{"d", "c", "a", "a", "d", "e", "b", "a"},
				i.Strings)
		}), "failed to invoke")
	})

	t.Run("failure to build a grouped value fails everything", func(t *testing.T) {
		c := New(setRand(rand.New(rand.NewSource(0))))

		type out struct {
			Out

			Result string `group:"x"`
		}

		require.NoError(t, c.Provide(func() (out, error) {
			return out{Result: "foo"}, nil
		}), "failed to provide")

		var gaveErr error
		require.NoError(t, c.Provide(func() (out, error) {
			gaveErr = errors.New("great sadness")
			return out{}, gaveErr
		}), "failed to provide")

		require.NoError(t, c.Provide(func() out {
			return out{Result: "bar"}
		}), "failed to provide")

		type in struct {
			In

			Strings []string `group:"x"`
		}

		err := c.Invoke(func(i in) {
			require.FailNow(t, "this function must not be called")
		})
		require.Error(t, err, "expected failure")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestGroups`,
			`could not build value group string\[group="x"\]:`,
			`function "go.uber.org/dig".TestGroups\S+ \(\S+:\d+\) returned a non-nil error:`,
			"great sadness",
		)
		assert.Equal(t, gaveErr, RootCause(err))
	})
}

// --- END OF END TO END TESTS

func TestProvideConstructorErrors(t *testing.T) {
	t.Run("multiple-type constructor returns multiple objects of same type", func(t *testing.T) {
		c := New()
		type A struct{}
		constructor := func() (*A, *A, error) {
			return &A{}, &A{}, nil
		}
		require.Error(t, c.Provide(constructor), "provide failed")
	})

	t.Run("constructor consumes a dig.Out", func(t *testing.T) {
		c := New()
		type out struct {
			Out

			Reader io.Reader
		}

		type outPtr struct {
			*Out

			Reader io.Reader
		}

		tests := []struct {
			desc        string
			constructor interface{}
			msg         string
		}{
			{
				desc:        "dig.Out",
				constructor: func(out) io.Writer { return nil },
				msg:         "dig.out embeds a dig.Out",
			},
			{
				desc:        "*dig.Out",
				constructor: func(*out) io.Writer { return nil },
				msg:         `\*dig.out embeds a dig.Out`,
			},
			{
				desc:        "embeds *dig.Out",
				constructor: func(outPtr) io.Writer { return nil },
				msg:         `dig.outPtr embeds a dig.Out`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				err := c.Provide(tt.constructor)
				require.Error(t, err, "provide should fail")
				assertErrorMatches(t, err,
					`function "go.uber.org/dig".TestProvideConstructorErrors\S+ \(\S+:\d+\) cannot be provided:`,
					`bad argument 1:`,
					`cannot depend on result objects:`,
					tt.msg)
			})
		}
	})
}

func TestProvideRespectsConstructorErrors(t *testing.T) {
	t.Run("constructor succeeds", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() (*bytes.Buffer, error) {
			return &bytes.Buffer{}, nil
		}), "provide failed")
		require.NoError(t, c.Invoke(func(b *bytes.Buffer) {
			require.NotNil(t, b, "invoke got nil buffer")
		}), "invoke failed")
	})
	t.Run("constructor fails", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(func() (*bytes.Buffer, error) {
			return nil, errors.New("oh no")
		}), "provide failed")

		var called bool
		err := c.Invoke(func(b *bytes.Buffer) { called = true })
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestProvideRespectsConstructorErrors\S+ \(\S+:\d+\):`,
			`failed to build \*bytes.Buffer:`,
			`function "go.uber.org/dig".TestProvideRespectsConstructorErrors\S+ \(\S+:\d+\) returned a non-nil error:`,
			`oh no`)
		assert.False(t, called, "shouldn't call invoked function when deps aren't available")
	})
}

func TestCantProvideObjects(t *testing.T) {
	t.Parallel()

	var writer io.Writer = &bytes.Buffer{}
	tests := []struct {
		object   interface{}
		typeDesc string
	}{
		{&bytes.Buffer{}, "pointer"},
		{bytes.Buffer{}, "struct"},
		{writer, "interface"},
		{map[string]string{}, "map"},
		{[]string{}, "slice"},
		{[1]string{}, "array"},
		{make(chan struct{}), "channel"},
	}

	for _, tt := range tests {
		t.Run(tt.typeDesc, func(t *testing.T) {
			c := New()
			assert.Error(t, c.Provide(tt.object))
		})
	}
}

func TestCantProvideUntypedNil(t *testing.T) {
	t.Parallel()
	c := New()
	assert.Error(t, c.Provide(nil))
}

func TestCantProvideErrorLikeType(t *testing.T) {
	t.Parallel()

	tests := []interface{}{
		func() *os.PathError { return &os.PathError{} },
		func() error { return &os.PathError{} },
		func() (*os.PathError, error) { return &os.PathError{}, nil },
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt), func(t *testing.T) {
			c := New()
			assert.Error(t, c.Provide(tt), "providing errors should fail")
		})
	}
}

func TestCantProvideParameterObjects(t *testing.T) {
	t.Parallel()

	t.Run("constructor", func(t *testing.T) {
		type Args struct{ In }

		c := New()
		err := c.Provide(func() (Args, error) {
			panic("great sadness")
		})
		require.Error(t, err, "provide should fail")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestCantProvideParameterObjects\S+ \(\S+:\d+\) cannot be provided:`,
			"bad result 1:",
			`cannot provide parameter objects:`,
			`dig.Args embeds a dig.In`,
		)
	})

	t.Run("pointer from constructor", func(t *testing.T) {
		c := New()
		type Args struct{ In }

		args := &Args{}

		err := c.Provide(func() (*Args, error) { return args, nil })
		require.Error(t, err)
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestCantProvideParameterObjects\S+ \(\S+:\d+\) cannot be provided:`,
			"bad result 1:",
			`cannot provide parameter objects:`,
			`\*dig.Args embeds a dig.In`,
		)
	})
}

func TestProvideKnownTypesFails(t *testing.T) {
	t.Parallel()

	provideArgs := []interface{}{
		func() *bytes.Buffer { return nil },
		func() (*bytes.Buffer, error) { return nil, nil },
	}

	for _, first := range provideArgs {
		t.Run(fmt.Sprintf("%T", first), func(t *testing.T) {
			c := New()
			require.NoError(t, c.Provide(first), "first provide must not fail")

			for _, second := range provideArgs {
				assert.Error(t, c.Provide(second), "second provide must fail")
			}
		})
	}
	t.Run("provide constructor twice", func(t *testing.T) {
		c := New()
		assert.NoError(t, c.Provide(func() *bytes.Buffer { return nil }))
		assert.Error(t, c.Provide(func() *bytes.Buffer { return nil }))
	})
}

func TestProvideCycleFails(t *testing.T) {
	t.Parallel()

	t.Run("parameters only", func(t *testing.T) {
		// A <- B <- C
		// |         ^
		// |_________|
		type A struct{}
		type B struct{}
		type C struct{}
		newA := func(*C) *A { return &A{} }
		newB := func(*A) *B { return &B{} }
		newC := func(*B) *C { return &C{} }

		c := New()
		assert.NoError(t, c.Provide(newA))
		assert.NoError(t, c.Provide(newB))
		err := c.Provide(newC)
		require.Error(t, err, "expected error when introducing cycle")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+:\d+\) cannot be provided:`,
			`this function introduces a cycle:`,
			`\*dig.C provided by "go.uber.org/dig".TestProvideCycleFails\S+ \(\S+\)`,
			`depends on \*dig.B provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
			`depends on \*dig.A provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
			`depends on \*dig.C provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
		)
	})

	t.Run("dig.In based cycle", func(t *testing.T) {
		// Same cycle as before but in terms of dig.Ins.

		type A struct{}
		type B struct{}
		type C struct{}

		type AParams struct {
			In

			C C
		}
		newA := func(AParams) A { return A{} }

		type BParams struct {
			In

			A A
		}
		newB := func(BParams) B { return B{} }

		type CParams struct {
			In

			B B
			W io.Writer
		}
		newC := func(CParams) C { return C{} }

		c := New()
		require.NoError(t, c.Provide(newA))
		require.NoError(t, c.Provide(newB))

		err := c.Provide(newC)
		require.Error(t, err, "expected error when introducing cycle")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+:\d+\) cannot be provided:`,
			`this function introduces a cycle:`,
			`dig.C provided by "go.uber.org/dig".TestProvideCycleFails\S+ \(\S+\)`,
			`depends on dig.B provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
			`depends on dig.A provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
			`depends on dig.C provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
		)
	})

	t.Run("group based cycle", func(t *testing.T) {
		type D struct{}

		type outA struct {
			Out

			Foo string `group:"foo"`
			Bar int    `group:"bar"`
		}
		newA := func() outA {
			require.FailNow(t, "must not be called")
			return outA{}
		}

		type outB struct {
			Out

			Foo string `group:"foo"`
		}
		newB := func(*D) outB {
			require.FailNow(t, "must not be called")
			return outB{}
		}

		type inC struct {
			In

			Foos []string `group:"foo"`
		}

		type outC struct {
			Out

			Bar int `group:"bar"`
		}

		newC := func(i inC) outC {
			require.FailNow(t, "must not be called")
			return outC{}
		}

		type inD struct {
			In

			Bars []int `group:"bar"`
		}

		newD := func(inD) *D {
			require.FailNow(t, "must not be called")
			return nil
		}

		c := New()
		require.NoError(t, c.Provide(newA))
		require.NoError(t, c.Provide(newB))
		require.NoError(t, c.Provide(newC))

		err := c.Provide(newD)
		require.Error(t, err)
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+:\d+\) cannot be provided:`,
			`this function introduces a cycle:`,
			`\*dig.D provided by "go.uber.org/dig".TestProvideCycleFails\S+ \(\S+\)`,
			`depends on int\[group="bar"\] provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
			`depends on string\[group="foo"\] provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
			`depends on \*dig.D provided by "go.uber.org/dig".TestProvideCycleFails.\S+ \(\S+\)`,
		)
	})
}

func TestIncompleteGraphIsOkay(t *testing.T) {
	t.Parallel()

	// A <- B <- C
	// Even if we don't provide B, we should be able to resolve A.
	type A struct{}
	type B struct{}
	type C struct{}
	newA := func() *A { return &A{} }
	newC := func(*B) *C { return &C{} }

	c := New()
	assert.NoError(t, c.Provide(newA), "provide failed")
	assert.NoError(t, c.Provide(newC), "provide failed")
	assert.NoError(t, c.Invoke(func(*A) {}), "invoke failed")
}

func TestProvideFuncsWithoutReturnsFails(t *testing.T) {
	t.Parallel()

	c := New()
	assert.Error(t, c.Provide(func(*bytes.Buffer) {}))
}

func TestTypeCheckingEquality(t *testing.T) {
	type A struct{}
	type B struct {
		Out
		A
	}
	type in struct {
		In
		A
	}
	type out struct {
		B
	}
	tests := []struct {
		item  interface{}
		isIn  bool
		isOut bool
	}{
		{in{}, true, false},
		{out{}, false, true},
		{A{}, false, false},
		{B{}, false, true},
		{nil, false, false},
	}
	for _, tt := range tests {
		require.Equal(t, tt.isIn, IsIn(tt.item))
		require.Equal(t, tt.isOut, IsOut(tt.item))
	}
}

func TestInvokesUseCachedObjects(t *testing.T) {
	t.Parallel()

	c := New()

	constructorCalls := 0
	buf := &bytes.Buffer{}
	require.NoError(t, c.Provide(func() *bytes.Buffer {
		assert.Equal(t, 0, constructorCalls, "constructor must not have been called before")
		constructorCalls++
		return buf
	}))

	calls := 0
	for i := 0; i < 3; i++ {
		assert.NoError(t, c.Invoke(func(b *bytes.Buffer) {
			calls++
			require.Equal(t, 1, constructorCalls, "constructor must be called exactly once")
			require.Equal(t, buf, b, "invoke got different buffer pointer")
		}), "invoke %d failed", i)
		require.Equal(t, i+1, calls, "invoked function not called")
	}
}

func TestProvideFailures(t *testing.T) {
	t.Run("out returning multiple instances of the same type", func(t *testing.T) {
		c := New()
		type A struct{ idx int }
		type ret struct {
			Out

			A1 A // sampe type A provided three times
			A2 A
			A3 A
		}

		err := c.Provide(func() ret {
			return ret{
				A1: A{idx: 1},
				A2: A{idx: 2},
				A3: A{idx: 3},
			}
		})
		require.Error(t, err, "provide must return error")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideFailures\S+ \(\S+:\d+\) cannot be provided:`,
			`cannot provide dig.A from \[0\].A2:`,
			`already provided by \[0\].A1`,
		)
	})

	t.Run("provide multiple instances with the same name", func(t *testing.T) {
		c := New()
		type A struct{}
		type ret1 struct {
			Out
			*A `name:"foo"`
		}
		type ret2 struct {
			Out
			*A `name:"foo"`
		}
		require.NoError(t, c.Provide(func() ret1 {
			return ret1{A: &A{}}
		}))
		err := c.Provide(func() ret2 {
			return ret2{A: &A{}}
		})
		require.Error(t, err, "expected error on the second provide")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideFailures\S+ \(\S+:\d+\) cannot be provided:`,
			`cannot provide \*dig.A\[name="foo"\] from \[0\].A:`,
			`already provided by "go.uber.org/dig".TestProvideFailures\S+`,
		)
	})

	t.Run("out with unexported field should error", func(t *testing.T) {
		c := New()

		type A struct{ idx int }
		type out1 struct {
			Out

			A1 A // should be ok
			a2 A // oops, unexported field. should generate an error
		}
		err := c.Provide(func() out1 { return out1{a2: A{77}} })
		require.Error(t, err)
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideFailures\S+ \(\S+:\d+\) cannot be provided:`,
			"bad result 1:",
			`bad field "a2" of dig.out1:`,
			`unexported fields not allowed in dig.Out, did you mean to export "a2" \(dig.A\)\?`,
		)
	})

	t.Run("providing pointer to out should fail", func(t *testing.T) {
		c := New()
		type out struct {
			Out

			String string
		}
		err := c.Provide(func() *out { return &out{String: "foo"} })
		require.Error(t, err)
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideFailures\S+ \(\S+:\d+\) cannot be provided:`,
			"bad result 1:",
			`cannot return a pointer to a result object, use a value instead:`,
			`\*dig.out is a pointer to a struct that embeds dig.Out`,
		)
	})

	t.Run("embedding pointer to out should fail", func(t *testing.T) {
		c := New()

		type out struct {
			*Out

			String string
		}

		err := c.Provide(func() out { return out{String: "foo"} })
		require.Error(t, err)
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestProvideFailures\S+ \(\S+:\d+\) cannot be provided:`,
			"bad result 1:",
			`cannot build a result object by embedding \*dig.Out, embed dig.Out instead:`,
			`dig.out embeds \*dig.Out`,
		)
	})
}

func TestInvokeFailures(t *testing.T) {
	t.Parallel()

	t.Run("invoke a non-function", func(t *testing.T) {
		c := New()
		err := c.Invoke("foo")
		require.Error(t, err)
		assertErrorMatches(t, err, `can't invoke non-function foo \(type string\)`)
	})

	t.Run("untyped nil", func(t *testing.T) {
		c := New()
		err := c.Invoke(nil)
		require.Error(t, err)
		assertErrorMatches(t, err, `can't invoke an untyped nil`)
	})

	t.Run("unmet dependency", func(t *testing.T) {
		c := New()

		err := c.Invoke(func(*bytes.Buffer) {})
		require.Error(t, err, "expected failure")
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+\):`,
			`type \*bytes.Buffer is not in the container, did you mean to Provide it\?`,
		)
	})

	t.Run("unmet required dependency", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}

		type args struct {
			In

			T1 *type1 `optional:"true"`
			T2 *type2 `optional:"0"`
		}

		c := New()
		err := c.Invoke(func(a args) {
			t.Fatal("function must not be called")
		})

		require.Error(t, err, "expected invoke error")
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+\):`,
			`type \*dig.type2 is not in the container, did you mean to Provide it\?`,
		)
	})

	t.Run("unmet named dependency", func(t *testing.T) {
		c := New()
		type param struct {
			In

			*bytes.Buffer `name:"foo"`
		}
		err := c.Invoke(func(p param) {
			t.Fatal("function should not be called")
		})
		require.Error(t, err, "invoke should fail")
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`type \*bytes.Buffer\[name="foo"\] is not in the container, did you mean to Provide it\?`,
		)
	})

	t.Run("unmet constructor dependency", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}

		type param struct {
			In

			T1 *type1
			T2 *type2 `optional:"true"`
		}

		c := New()

		require.NoError(t, c.Provide(func(p param) *type3 {
			panic("function must not be called")
		}), "provide failed")

		err := c.Invoke(func(*type3) {
			t.Fatal("function must not be called")
		})
		require.Error(t, err, "invoke must fail")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+:\d+\):`,
			`failed to build \*dig.type3:`,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`type \*dig.type1 is not in the container, did you mean to Provide it\?`,
		)
		// We don't expect type2 to be mentioned in the list because it's
		// optional
	})

	t.Run("multiple unmet constructor dependencies", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}

		c := New()

		require.NoError(t, c.Provide(func() type2 {
			panic("function must not be called")
		}), "provide should not fail")

		require.NoError(t, c.Provide(func(type1, *type2) type3 {
			panic("function must not be called")
		}), "provide should not fail")

		err := c.Invoke(func(type3) {
			t.Fatal("function must not be called")
		})

		require.Error(t, err, "invoke must fail")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+:\d+\):`,
			`failed to build dig.type3:`,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`the following types are not in the container:`,
			"dig.type1;",
			`\*dig.type2 \(did you mean dig.type2\?\)`,
		)
	})

	t.Run("invalid optional tag", func(t *testing.T) {
		type args struct {
			In

			Buffer *bytes.Buffer `optional:"no"`
		}

		c := New()
		err := c.Invoke(func(a args) {
			t.Fatal("function must not be called")
		})

		require.Error(t, err, "expected invoke error")
		assertErrorMatches(t, err,
			`bad field "Buffer" of dig.args:`,
			`invalid value "no" for "optional" tag on field Buffer:`,
		)
	})

	t.Run("constructor invalid optional tag", func(t *testing.T) {
		type type1 struct{}

		type nestedArgs struct {
			In

			Buffer *bytes.Buffer `optional:"no"`
		}

		type args struct {
			In

			Args nestedArgs
		}

		c := New()
		err := c.Provide(func(a args) *type1 {
			panic("function must not be called")
		})

		require.Error(t, err, "expected provide error")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+:\d+\) cannot be provided:`,
			"bad argument 1:",
			`bad field "Args" of dig.args:`,
			`bad field "Buffer" of dig.nestedArgs:`,
			`invalid value "no" for "optional" tag on field Buffer:`,
		)
	})

	t.Run("optional dep with unmet transitive dep", func(t *testing.T) {
		type missing struct{}
		type dep struct{}

		type params struct {
			In

			Dep *dep `optional:"true"`
		}

		c := New()

		// Container has a constructor for *dep, but that constructor has unmet
		// dependencies.
		err := c.Provide(func(missing) *dep {
			panic("constructor for *dep should not be called")
		})
		require.NoError(t, err, "unexpected provide error")

		// Should still be able to invoke a function that takes params, since *dep
		// is optional.
		var count int
		err = c.Invoke(func(p params) {
			count++
			assert.Nil(t, p.Dep, "expected optional dependency to be unmet")
		})
		assert.NoError(t, err, "unexpected invoke error")
		assert.Equal(t, 1, count, "expected invoke function to be called")
	})

	t.Run("optional dep with failed transitive dep", func(t *testing.T) {
		type failed struct{}
		type dep struct{}

		type params struct {
			In

			Dep *dep `optional:"true"`
		}

		c := New()

		errFailed := errors.New("failed")
		err := c.Provide(func() (*failed, error) {
			return nil, errFailed
		})
		require.NoError(t, err, "unexpected provide error")

		err = c.Provide(func(*failed) *dep {
			panic("constructor for *dep should not be called")
		})
		require.NoError(t, err, "unexpected provide error")

		// Should still be able to invoke a function that takes params, since *dep
		// is optional.
		err = c.Invoke(func(p params) {
			panic("shouldn't execute invoked function")
		})
		require.Error(t, err, "expected invoke error")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+:\d+\):`,
			`failed to build \*dig.dep:`,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+:\d+\):`,
			`failed to build \*dig.failed:`,
			`function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+:\d+\) returned a non-nil error:`,
			`failed`,
		)
		assert.Equal(t, errFailed, RootCause(err), "root cause must match")
	})

	t.Run("returned error", func(t *testing.T) {
		c := New()
		err := c.Invoke(func() error { return errors.New("oh no") })
		require.Equal(t, errors.New("oh no"), err, "error must match")
	})

	t.Run("many returns", func(t *testing.T) {
		c := New()
		err := c.Invoke(func() (int, error) { return 42, errors.New("oh no") })
		require.Equal(t, errors.New("oh no"), err, "error must match")
	})

	t.Run("named instances are case sensitive", func(t *testing.T) {
		c := New()
		type A struct{}
		type ret struct {
			Out
			A `name:"CamelCase"`
		}
		type param1 struct {
			In
			A `name:"CamelCase"`
		}
		type param2 struct {
			In
			A `name:"camelcase"`
		}
		require.NoError(t, c.Provide(func() ret { return ret{A: A{}} }))
		require.NoError(t, c.Invoke(func(param1) {}))
		err := c.Invoke(func(param2) {})
		require.Error(t, err, "provide should return error since cases don't match")
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+:\d+\):`,
			`type dig.A\[name="camelcase"\] is not in the container, did you mean to Provide it`)
	})

	t.Run("in unexported member gets an error", func(t *testing.T) {
		c := New()
		type A struct{}
		type in struct {
			In

			A1 A // all is good
			a2 A // oops, unexported type
		}
		require.NoError(t, c.Provide(func() A { return A{} }))

		err := c.Invoke(func(i in) { assert.Fail(t, "should never get in here") })
		require.Error(t, err)
		assertErrorMatches(t, err,
			"bad argument 1:",
			`bad field "a2" of dig.in:`,
			`unexported fields not allowed in dig.In, did you mean to export "a2" \(dig.A\)\?`,
		)
	})

	t.Run("in unexported member gets an error on Provide", func(t *testing.T) {
		c := New()
		type in struct {
			In

			foo string
		}

		err := c.Provide(func(in) int { return 0 })
		require.Error(t, err, "Provide must fail")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestInvokeFailures\S+ \(\S+:\d+\) cannot be provided:`,
			"bad argument 1:",
			`bad field "foo" of dig.in:`,
			`unexported fields not allowed in dig.In, did you mean to export "foo" \(string\)\?`,
		)
	})

	t.Run("embedded unexported member gets an error", func(t *testing.T) {
		c := New()
		type A struct{}
		type Embed struct {
			In

			A1 A // all is good
			a2 A // oops, unexported type
		}
		type in struct {
			Embed
		}
		require.NoError(t, c.Provide(func() A { return A{} }))

		err := c.Invoke(func(i in) { assert.Fail(t, "should never get in here") })
		require.Error(t, err)
		assertErrorMatches(t, err,
			"bad argument 1:",
			`bad field "Embed" of dig.in:`,
			`bad field "a2" of dig.Embed:`,
			`unexported fields not allowed in dig.In, did you mean to export "a2" \(dig.A\)\?`,
		)
	})

	t.Run("embedded unexported member gets an error", func(t *testing.T) {
		c := New()
		type param struct {
			In

			string // embed an unexported std type
		}
		err := c.Invoke(func(p param) { assert.Fail(t, "should never get here") })
		require.Error(t, err)
		assertErrorMatches(t, err,
			"bad argument 1:",
			`bad field "string" of dig.param:`,
			`unexported fields not allowed in dig.In, did you mean to export "string" \(string\)\?`,
		)
	})

	t.Run("pointer in dependency is not supported", func(t *testing.T) {
		c := New()
		type in struct {
			In

			String string
			Num    int
		}
		err := c.Invoke(func(i *in) { assert.Fail(t, "should never get here") })
		require.Error(t, err)
		assertErrorMatches(t, err,
			"bad argument 1:",
			"cannot depend on a pointer to a parameter object, use a value instead:",
			`\*dig.in is a pointer to a struct that embeds dig.In`,
		)
	})

	t.Run("embedding dig.In and dig.Out is not supported", func(t *testing.T) {
		c := New()
		type in struct {
			In
			Out

			String string
		}

		err := c.Invoke(func(in) {
			assert.Fail(t, "should never get here")
		})
		require.Error(t, err)
		assertErrorMatches(t, err,
			"bad argument 1:",
			"cannot depend on result objects:",
			"dig.in embeds a dig.Out",
		)
	})

	t.Run("embedding in pointer is not supported", func(t *testing.T) {
		c := New()
		type in struct {
			*In

			String string
			Num    int
		}
		err := c.Invoke(func(i in) { assert.Fail(t, "should never get here") })
		require.Error(t, err)
		assertErrorMatches(t, err,
			"bad argument 1:",
			`cannot build a parameter object by embedding \*dig.In, embed dig.In instead:`,
			`dig.in embeds \*dig.In`,
		)
	})

	t.Run("requesting a value or pointer when other is present", func(t *testing.T) {
		type A struct{}
		type outA struct {
			Out

			A `name:"hello"`
		}
		type B struct{}

		cases := []struct {
			name        string
			provide     interface{}
			invoke      interface{}
			errContains string
		}{
			{
				name:        "value missing, pointer present",
				provide:     func() *A { return &A{} },
				invoke:      func(A) {},
				errContains: `type dig.A is not in the container, did you mean to use \*dig.A\?`,
			},
			{
				name:        "pointer missing, value present",
				provide:     func() A { return A{} },
				invoke:      func(*A) {},
				errContains: `type \*dig.A is not in the container, did you mean to use dig.A?`,
			},
			{
				name:    "named pointer missing, value present",
				provide: func() outA { return outA{A: A{}} },
				invoke: func(struct {
					In

					*A `name:"hello"`
				}) {
				},
				errContains: `type \*dig.A\[name="hello"\] is not in the container, did you mean to use dig.A\[name="hello"\]?`,
			},
		}

		for _, tc := range cases {
			c := New()
			t.Run(tc.name, func(t *testing.T) {
				require.NoError(t, c.Provide(tc.provide))

				err := c.Invoke(tc.invoke)
				require.Error(t, err)
				assertErrorMatches(t, err,
					`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
					tc.errContains,
				)
			})
		}
	})

	t.Run("requesting an interface when an implementation is available", func(t *testing.T) {
		c := New()
		require.NoError(t, c.Provide(bytes.NewReader))
		err := c.Invoke(func(io.Reader) {
			t.Fatalf("this function should not be called")
		})
		require.Error(t, err)
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`type io.Reader is not in the container, did you mean to use \*bytes.Reader\?`,
		)
	})

	t.Run("requesting an interface when multiple implementations are available", func(t *testing.T) {
		c := New()

		require.NoError(t, c.Provide(bytes.NewReader))
		require.NoError(t, c.Provide(bytes.NewBufferString))

		err := c.Invoke(func(io.Reader) {
			t.Fatalf("this function should not be called")
		})
		require.Error(t, err)
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`type io.Reader is not in the container, did you mean to use one of \*bytes.Buffer, or \*bytes.Reader\?`,
		)
	})

	t.Run("requesting multiple interfaces when multiple implementations are available", func(t *testing.T) {
		c := New()

		require.NoError(t, c.Provide(bytes.NewReader))
		require.NoError(t, c.Provide(bytes.NewBufferString))

		err := c.Invoke(func(io.Reader, io.Writer) {
			t.Fatalf("this function should not be called")
		})
		require.Error(t, err)
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`the following types are not in the container:`,
			`io.Reader \(did you mean \*bytes.Buffer, or \*bytes.Reader\?\);`,
			`io.Writer \(did you mean \*bytes.Buffer\?\)`,
		)
	})

	t.Run("requesting a type when an interface is available", func(t *testing.T) {
		c := New()

		require.NoError(t, c.Provide(func() io.Writer { return nil }))
		err := c.Invoke(func(*bytes.Buffer) {
			t.Fatalf("this function should not be called")
		})

		require.Error(t, err)
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`type \*bytes.Buffer is not in the container, did you mean to use io.Writer\?`,
		)
	})

	t.Run("requesting a type when multiple interfaces are available", func(t *testing.T) {
		c := New()

		require.NoError(t, c.Provide(func() io.Writer { return nil }))
		require.NoError(t, c.Provide(func() io.Reader { return nil }))

		err := c.Invoke(func(*bytes.Buffer) {
			t.Fatalf("this function should not be called")
		})

		require.Error(t, err)
		assertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+\):`,
			`type \*bytes.Buffer is not in the container, did you mean to use one of io.Reader, or io.Writer\?`,
		)
	})

	t.Run("direct dependency error", func(t *testing.T) {
		type A struct{}

		c := New()

		require.NoError(t, c.Provide(func() (A, error) {
			return A{}, errors.New("great sadness")
		}), "Provide failed")

		err := c.Invoke(func(A) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		assertErrorMatches(t, err,
			`function "go.uber.org/dig".TestInvokeFailures.func\S+ \(\S+\) returned a non-nil error:`,
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), RootCause(err))
	})

	t.Run("transitive dependency error", func(t *testing.T) {
		type A struct{}
		type B struct{}

		c := New()

		require.NoError(t, c.Provide(func() (A, error) {
			return A{}, errors.New("great sadness")
		}), "Provide failed")

		require.NoError(t, c.Provide(func(A) (B, error) {
			return B{}, nil
		}), "Provide failed")

		err := c.Invoke(func(B) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures\S+`,
			"failed to build dig.B",
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures\S+`,
			"failed to build dig.A",
			`function "go.uber.org/dig".TestInvokeFailures.func\S+ \(\S+\) returned a non-nil error:`,
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), RootCause(err))
	})

	t.Run("direct parameter object error", func(t *testing.T) {
		type A struct{}

		c := New()

		require.NoError(t, c.Provide(func() (A, error) {
			return A{}, errors.New("great sadness")
		}), "Provide failed")

		type params struct {
			In

			A A
		}

		err := c.Invoke(func(params) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures.func\S+`,
			"failed to build dig.A:",
			`function "go.uber.org/dig".TestInvokeFailures.func\S+ \(\S+\) returned a non-nil error:`,
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), RootCause(err))
	})

	t.Run("transitive parameter object error", func(t *testing.T) {
		type A struct{}
		type B struct{}

		c := New()

		require.NoError(t, c.Provide(func() (A, error) {
			return A{}, errors.New("great sadness")
		}), "Provide failed")

		type params struct {
			In

			A A
		}

		require.NoError(t, c.Provide(func(params) (B, error) {
			return B{}, nil
		}), "Provide failed")

		err := c.Invoke(func(B) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures.func\S+ \(\S+:\d+\):`,
			"failed to build dig.B:",
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures.func\S+`,
			"failed to build dig.A:",
			`function "go.uber.org/dig".TestInvokeFailures.func\S+ \(\S+\) returned a non-nil error:`,
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), RootCause(err))
	})

	t.Run("unmet dependency of a group value", func(t *testing.T) {
		c := New()

		type A struct{}
		type B struct{}

		type out struct {
			Out

			B B `group:"b"`
		}

		require.NoError(t, c.Provide(func(A) out {
			require.FailNow(t, "must not be called")
			return out{}
		}))

		type in struct {
			In

			Bs []B `group:"b"`
		}

		err := c.Invoke(func(in) {
			require.FailNow(t, "must not be called")
		})
		require.Error(t, err, "expected failure")
		assertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+:\d+\):`,
			`could not build value group dig.B\[group="b"\]:`,
			`missing dependencies for function "go.uber.org/dig".TestInvokeFailures.\S+ \(\S+:\d+\):`,
			"type dig.A is not in the container, did you mean to Provide it?",
		)
	})
}

func TestNodeAlreadyCalled(t *testing.T) {
	type type1 struct{}
	f := func() type1 { return type1{} }

	n, err := newNode(f)
	require.NoError(t, err, "failed to build node")
	require.False(t, n.called, "node must not have been called")

	c := New()
	require.NoError(t, n.Call(c), "invoke failed")
	require.True(t, n.called, "node must be called")
	require.NoError(t, n.Call(c), "calling again should be okay")
}

func TestFailingFunctionDoesNotCreateInvalidState(t *testing.T) {
	type type1 struct{}

	c := New()
	require.NoError(t, c.Provide(func() (type1, error) {
		return type1{}, errors.New("great sadness")
	}), "provide failed")

	require.Error(t, c.Invoke(func(type1) {
		require.FailNow(t, "first invoke must not call the function")
	}), "first invoke must fail")

	require.Error(t, c.Invoke(func(type1) {
		require.FailNow(t, "second invoke must not call the function")
	}), "second invoke must fail")
}
