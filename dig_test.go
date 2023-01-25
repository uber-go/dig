// Copyright (c) 2019 Uber Technologies, Inc.
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

package dig_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
	"go.uber.org/dig/internal/digtest"
)

func TestEndToEndSuccess(t *testing.T) {
	t.Parallel()

	t.Run("pointer constructor", func(t *testing.T) {
		c := digtest.New(t)
		var b *bytes.Buffer
		c.RequireProvide(func() *bytes.Buffer {
			b = &bytes.Buffer{}
			return b
		})

		c.RequireInvoke(func(got *bytes.Buffer) {
			require.NotNil(t, got, "invoke got nil buffer")
			require.True(t, got == b, "invoke got wrong buffer")
		})
	})

	t.Run("nil pointer constructor", func(t *testing.T) {
		// Dig shouldn't forbid this - it's perfectly reasonable to explicitly
		// provide a typed nil, since that's often a convenient way to supply a
		// default no-op implementation.
		c := digtest.New(t)
		c.RequireProvide(func() *bytes.Buffer { return nil })
		c.RequireInvoke(func(b *bytes.Buffer) {
			require.Nil(t, b, "expected to get nil buffer")
		})
	})

	t.Run("struct constructor", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() bytes.Buffer {
			var buf bytes.Buffer
			buf.WriteString("foo")
			return buf
		})

		c.RequireInvoke(func(b bytes.Buffer) {
			// ensure we're getting back the buffer we put in
			require.Equal(t, "foo", b.String(), "invoke got new buffer")
		})
	})

	t.Run("slice constructor", func(t *testing.T) {
		c := digtest.New(t)
		b1 := &bytes.Buffer{}
		b2 := &bytes.Buffer{}
		c.RequireProvide(func() []*bytes.Buffer {
			return []*bytes.Buffer{b1, b2}
		})

		c.RequireInvoke(func(bs []*bytes.Buffer) {
			require.Equal(t, 2, len(bs), "invoke got unexpected number of buffers")
			require.True(t, b1 == bs[0], "first item did not match")
			require.True(t, b2 == bs[1], "second item did not match")
		})
	})

	t.Run("array constructor", func(t *testing.T) {
		c := digtest.New(t)
		bufs := [1]*bytes.Buffer{{}}
		c.RequireProvide(func() [1]*bytes.Buffer { return bufs })
		c.RequireInvoke(func(bs [1]*bytes.Buffer) {
			require.NotNil(t, bs[0], "invoke got new array")
		})
	})

	t.Run("map constructor", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() map[string]string {
			return map[string]string{}
		})

		c.RequireInvoke(func(m map[string]string) {
			require.NotNil(t, m, "invoke got zero value map")
		})
	})

	t.Run("channel constructor", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() chan int {
			return make(chan int)
		})

		c.RequireInvoke(func(ch chan int) {
			require.NotNil(t, ch, "invoke got nil chan")
		})
	})

	t.Run("func constructor", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() func(int) {
			return func(int) {}
		})

		c.RequireInvoke(func(f func(int)) {
			require.NotNil(t, f, "invoke got nil function pointer")
		})
	})

	t.Run("interface constructor", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() io.Writer {
			return &bytes.Buffer{}
		})

		c.RequireInvoke(func(w io.Writer) {
			require.NotNil(t, w, "invoke got nil interface")
		})
	})

	t.Run("param", func(t *testing.T) {
		c := digtest.New(t)
		type contents string
		type Args struct {
			dig.In

			Contents contents
		}

		c.RequireProvide(func(args Args) *bytes.Buffer {
			require.NotEmpty(t, args.Contents, "contents must not be empty")
			return bytes.NewBufferString(string(args.Contents))
		})

		c.RequireProvide(func() contents { return "hello world" })

		c.RequireInvoke(func(buff *bytes.Buffer) {
			out, err := io.ReadAll(buff)
			require.NoError(t, err, "read from buffer failed")
			require.Equal(t, "hello world", string(out), "contents don't match")
		})
	})

	t.Run("invoke param", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() *bytes.Buffer {
			return new(bytes.Buffer)
		})

		type Args struct {
			dig.In

			*bytes.Buffer
		}

		c.RequireInvoke(func(args Args) {
			require.NotNil(t, args.Buffer, "invoke got nil buffer")
		})
	})

	t.Run("param wrapper", func(t *testing.T) {
		var (
			buff   *bytes.Buffer
			called bool
		)

		c := digtest.New(t)
		c.RequireProvide(func() *bytes.Buffer {
			require.False(t, called, "constructor must be called exactly once")
			called = true
			buff = new(bytes.Buffer)
			return buff
		})

		type MyParam struct{ dig.In }

		type Args struct {
			MyParam

			Buffer *bytes.Buffer
		}

		c.RequireInvoke(func(args Args) {
			require.True(t, called, "constructor must be called first")
			require.NotNil(t, args.Buffer, "invoke got nil buffer")
			require.True(t, args.Buffer == buff, "buffer must match constructor's return value")
		})
	})

	t.Run("param recurse", func(t *testing.T) {
		type anotherParam struct {
			dig.In

			Buffer *bytes.Buffer
		}

		type someParam struct {
			dig.In

			Buffer  *bytes.Buffer
			Another anotherParam
		}

		var (
			buff   *bytes.Buffer
			called bool
		)

		c := digtest.New(t)
		c.RequireProvide(func() *bytes.Buffer {
			require.False(t, called, "constructor must be called exactly once")
			called = true
			buff = new(bytes.Buffer)
			return buff
		})

		c.RequireInvoke(func(p someParam) {
			require.True(t, called, "constructor must be called first")

			require.NotNil(t, p.Buffer, "someParam.Buffer must not be nil")
			require.NotNil(t, p.Another.Buffer, "anotherParam.Buffer must not be nil")

			require.True(t, p.Buffer == p.Another.Buffer, "buffers fields must match")
			require.True(t, p.Buffer == buff, "buffer must match constructor's return value")
		})
	})

	t.Run("multiple-type constructor", func(t *testing.T) {
		c := digtest.New(t)
		constructor := func() (*bytes.Buffer, []int, error) {
			return &bytes.Buffer{}, []int{42}, nil
		}
		consumer := func(b *bytes.Buffer, nums []int) {
			assert.NotNil(t, b, "invoke got nil buffer")
			assert.Equal(t, 1, len(nums), "invoke got empty slice")
		}
		c.RequireProvide(constructor)
		c.RequireInvoke(consumer)
	})

	t.Run("multiple-type constructor is called once", func(t *testing.T) {
		c := digtest.New(t)
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
		c.RequireProvide(constructor)
		c.RequireInvoke(getA)
		c.RequireInvoke(getB)
		c.RequireInvoke(func(a *A, b *B) {})
		require.Equal(t, 1, count, "Constructor must be called once")
	})

	t.Run("method invocation inside Invoke", func(t *testing.T) {
		c := digtest.New(t)
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

		c.RequireProvide(cA)
		c.RequireProvide(cB)
		c.RequireInvoke(getA)
	})

	t.Run("collections and instances of same type", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() []*bytes.Buffer {
			return []*bytes.Buffer{{}}
		})

		c.RequireProvide(func() *bytes.Buffer {
			return &bytes.Buffer{}
		})
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

		c := digtest.New(t)
		type param struct {
			dig.In

			T1 *type1 // regular 'ol type
			T2 *type2 `optional:"true" useless_tag:"false"` // optional type NOT in the graph
			T3 *type3 `unrelated:"foo=42, optional"`        // type in the graph with unrelated tag
			T4 *type4 `optional:"true"`                     // optional type present in the graph
			T5 *type5 `optional:"t"`                        // optional type NOT in the graph with "yes"
		}
		c.RequireProvide(constructor)
		c.RequireInvoke(func(p param) {
			require.NotNil(t, p.T1, "whole param struct should not be nil")
			assert.Nil(t, p.T2, "optional type not in the graph should return nil")
			assert.NotNil(t, p.T3, "required type with unrelated tag not in the graph")
			assert.NotNil(t, p.T4, "optional type in the graph should not return nil")
			assert.Nil(t, p.T5, "optional type not in the graph should return nil")
		})
	})

	t.Run("ignore unexported fields", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		constructor := func() (*type1, *type2, *type3) {
			return &type1{}, &type2{}, &type3{}
		}

		c := digtest.New(t)
		type param struct {
			dig.In `ignore-unexported:"true"`

			T1 *type1 // regular 'ol type
			T2 *type2 `optional:"true"` // optional type present in the graph
			t3 *type3
		}
		c.RequireProvide(constructor)
		c.RequireInvoke(func(p param) {
			require.NotNil(t, p.T1, "whole param struct should not be nil")
			assert.NotNil(t, p.T2, "optional type in the graph should not return nil")
			assert.Nil(t, p.t3, "unexported field should not be set")
		})
	})

	t.Run("out type inserts multiple objects into the graph", func(t *testing.T) {
		type A struct{ name string }
		type B struct{ name string }
		type Ret struct {
			dig.Out
			A  // value type A
			*B // pointer type *B
		}
		myA := A{"string A"}
		myB := &B{"string B"}

		c := digtest.New(t)
		c.RequireProvide(func() Ret {
			return Ret{A: myA, B: myB}
		})

		c.RequireInvoke(func(a A, b *B) {
			assert.Equal(t, a.name, "string A", "value type should work for dig.Out")
			assert.Equal(t, b.name, "string B", "pointer should work for dig.Out")
			assert.True(t, myA == a, "should get the same pointer for &A")
			assert.Equal(t, b, myB, "b and myB should be equal")
		})
	})

	t.Run("constructor with optional", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}

		type param struct {
			dig.In

			T1 *type1 `optional:"true"`
		}

		c := digtest.New(t)

		var gave *type2
		c.RequireProvide(func(p param) *type2 {
			require.Nil(t, p.T1, "T1 must be nil")
			gave = &type2{}
			return gave
		})

		c.RequireInvoke(func(got *type2) {
			require.True(t, got == gave, "type2 reference must be the same")
		})
	})

	t.Run("nested dependencies", func(t *testing.T) {
		c := digtest.New(t)

		type A struct{ name string }
		type B struct{ name string }
		type C struct{ name string }

		c.RequireProvide(func() A { return A{"->A"} })
		c.RequireProvide(func(A) B { return B{"A->B"} })
		c.RequireProvide(func(A, B) C { return C{"AB->C"} })
		c.RequireInvoke(func(a A, b B, c C) {
			assert.Equal(t, a, A{"->A"})
			assert.Equal(t, b, B{"A->B"})
			assert.Equal(t, c, C{"AB->C"})
		})
	})

	t.Run("primitives", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() string { return "piper" })
		c.RequireProvide(func() int { return 42 })
		c.RequireProvide(func() int64 { return 24 })
		c.RequireProvide(func() time.Duration {
			return 10 * time.Second
		})

		c.RequireInvoke(func(i64 int64, i int, s string, d time.Duration) {
			assert.Equal(t, 42, i)
			assert.Equal(t, int64(24), i64)
			assert.Equal(t, "piper", s)
			assert.Equal(t, 10*time.Second, d)
		})
	})

	t.Run("out types recurse", func(t *testing.T) {
		type A struct{}
		type B struct{}
		type C struct{}
		// Contains A
		type Ret1 struct {
			dig.Out
			*A
		}
		// Contains *A (through Ret1), *B and C
		type Ret2 struct {
			Ret1
			*B
			C
		}
		c := digtest.New(t)

		c.RequireProvide(func() Ret2 {
			return Ret2{
				Ret1: Ret1{
					A: &A{},
				},
				B: &B{},
				C: C{},
			}
		})

		c.RequireInvoke(func(a *A, b *B, c C) {
			require.NotNil(t, a, "*A should be part of the container through Ret2->Ret1")
		})
	})

	t.Run("named instances can be created with tags", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }

		// returns three named instances of A
		type ret struct {
			dig.Out

			A1 A `name:"first"`
			A2 A `name:"second"`
			A3 A `name:"third"`
		}

		// requires two specific named instances
		type param struct {
			dig.In

			A1 A `name:"first"`
			A3 A `name:"third"`
		}
		c.RequireProvide(func() ret {
			return ret{A1: A{1}, A2: A{2}, A3: A{3}}
		})

		c.RequireInvoke(func(p param) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 3, p.A3.idx)
		})
	})

	t.Run("named instances can be created with Name option", func(t *testing.T) {
		c := digtest.New(t)

		type A struct{ idx int }

		buildConstructor := func(idx int) func() A {
			return func() A { return A{idx: idx} }
		}

		c.RequireProvide(buildConstructor(1), dig.Name("first"))
		c.RequireProvide(buildConstructor(2), dig.Name("second"))
		c.RequireProvide(buildConstructor(3), dig.Name("third"))

		type param struct {
			dig.In

			A1 A `name:"first"`
			A3 A `name:"third"`
		}

		c.RequireInvoke(func(p param) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 3, p.A3.idx)
		})
	})

	t.Run("named and unnamed instances coexist", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }

		type out struct {
			dig.Out

			A `name:"foo"`
		}

		c.RequireProvide(func() out { return out{A: A{1}} })
		c.RequireProvide(func() A { return A{2} })

		type in struct {
			dig.In

			A1 A `name:"foo"`
			A2 A
		}
		c.RequireInvoke(func(i in) {
			assert.Equal(t, 1, i.A1.idx)
			assert.Equal(t, 2, i.A2.idx)
		})
	})

	t.Run("named instances recurse", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }

		type Ret1 struct {
			dig.Out

			A1 A `name:"first"`
		}
		type Ret2 struct {
			Ret1

			A2 A `name:"second"`
		}
		type param struct {
			dig.In

			A1 A `name:"first"`  // should come from ret1 through ret2
			A2 A `name:"second"` // should come from ret2
		}
		c.RequireProvide(func() Ret2 {
			return Ret2{
				Ret1: Ret1{
					A1: A{1},
				},
				A2: A{2},
			}
		})

		c.RequireInvoke(func(p param) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 2, p.A2.idx)
		})
	})

	t.Run("named instances do not cause cycles", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }
		type param struct {
			dig.In
			A `name:"uno"`
		}
		type paramBoth struct {
			dig.In

			A1 A `name:"uno"`
			A2 A `name:"dos"`
		}
		type retUno struct {
			dig.Out
			A `name:"uno"`
		}
		type retDos struct {
			dig.Out
			A `name:"dos"`
		}

		c.RequireProvide(func() retUno {
			return retUno{A: A{1}}
		})

		c.RequireProvide(func(p param) retDos {
			return retDos{A: A{2}}
		})

		c.RequireInvoke(func(p paramBoth) {
			assert.Equal(t, 1, p.A1.idx)
			assert.Equal(t, 2, p.A2.idx)
		})
	})

	t.Run("struct constructor with as interface option", func(t *testing.T) {
		c := digtest.New(t)

		c.RequireProvide(
			func() *bytes.Buffer {
				return bytes.NewBufferString("foo")
			},
			dig.As(new(fmt.Stringer), new(io.Reader)),
		)

		c.RequireInvoke(
			func(s fmt.Stringer, r io.Reader) {
				require.Equal(t, "foo", s.String(), "invoke got new buffer")
				got, err := io.ReadAll(r)
				assert.NoError(t, err, "failed to read from reader")
				require.Equal(t, "foo", string(got), "invoke got new buffer")
			})

		require.Error(t, c.Invoke(func(*bytes.Buffer) {
			t.Fatalf("must not be called")
		}), "must not have a *bytes.Buffer in the container")
	})

	t.Run("As with Name", func(t *testing.T) {
		c := digtest.New(t)

		c.RequireProvide(
			func() *bytes.Buffer {
				return bytes.NewBufferString("foo")
			},
			dig.As(new(io.Reader)),
			dig.Name("buff"))

		type in struct {
			dig.In

			Buffer *bytes.Buffer `name:"buff"`
			Reader io.Reader     `name:"buff"`
		}

		require.Error(t, c.Invoke(func(got in) {
			t.Fatal("should not be called")
		}))
	})

	t.Run("As with Group", func(t *testing.T) {
		c := digtest.New(t)
		strs := map[string]struct{}{
			"foo": {},
			"bar": {},
		}
		for s := range strs {
			s := s
			c.RequireProvide(func() *bytes.Buffer {
				return bytes.NewBufferString(s)
			}, dig.Group("readers"), dig.As(new(io.Reader)))
		}
		type in struct {
			dig.In

			Readers []io.Reader `group:"readers"`
		}

		c.RequireInvoke(func(got in) {
			require.Len(t, got.Readers, 2)
			for _, r := range got.Readers {
				buf := make([]byte, 3)
				n, err := r.Read(buf)
				require.NoError(t, err)
				require.Equal(t, 3, n)
				s := string(buf)
				_, ok := strs[s]
				require.True(t, ok, fmt.Sprintf("%s should be in the map %v", s, strs))
				delete(strs, s)
			}
		})
	})

	t.Run("multiple As with Group", func(t *testing.T) {
		c := digtest.New(t)
		strs := map[string]struct{}{
			"foo": {},
			"bar": {},
		}
		for s := range strs {
			s := s
			c.RequireProvide(func() *bytes.Buffer {
				return bytes.NewBufferString(s)
			}, dig.Group("buffs"), dig.As(new(io.Reader), new(io.Writer)))
		}
		type in struct {
			dig.In

			Readers []io.Reader `group:"buffs"`
			Writers []io.Writer `group:"buffs"`
		}

		c.RequireInvoke(func(got in) {
			require.Len(t, got.Readers, 2)
			for _, r := range got.Readers {
				buf := make([]byte, 3)
				n, err := r.Read(buf)
				require.NoError(t, err)
				require.Equal(t, 3, n)
				s := string(buf)
				_, ok := strs[s]
				require.True(t, ok, fmt.Sprintf("%s should be in the map %v", s, strs))
				delete(strs, s)
			}
			require.Len(t, got.Writers, 2)
		})
	})

	t.Run("As same interface", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() io.Reader {
			panic("this function should not be called")
		}, dig.As(new(io.Reader)))
	})

	t.Run("As different interface", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() io.ReadCloser {
			panic("this function should not be called")
		}, dig.As(new(io.Reader), new(io.Closer)))
	})

	t.Run("invoke on a type that depends on named parameters", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }
		type B struct{ sum int }
		type param struct {
			dig.In

			A1 *A `name:"foo"`
			A2 *A `name:"bar"`
			A3 *A `name:"baz" optional:"true"`
		}
		type ret struct {
			dig.Out

			A1 *A `name:"foo"`
			A2 *A `name:"bar"`
		}
		c.RequireProvide(func() (ret, error) {
			return ret{
				A1: &A{1},
				A2: &A{2},
			}, nil
		})

		c.RequireProvide(func(p param) *B {
			return &B{sum: p.A1.idx + p.A2.idx}
		})

		c.RequireInvoke(func(b *B) {
			require.Equal(t, 3, b.sum)
		})
	})

	t.Run("optional and named ordering doesn't matter", func(t *testing.T) {
		type param1 struct {
			dig.In

			Foo *struct{} `name:"foo" optional:"true"`
		}

		type param2 struct {
			dig.In

			Foo *struct{} `optional:"true" name:"foo"`
		}

		t.Run("optional", func(t *testing.T) {
			c := digtest.New(t)

			called1 := false
			c.RequireInvoke(func(p param1) {
				called1 = true
				assert.Nil(t, p.Foo)
			})

			called2 := false
			c.RequireInvoke(func(p param2) {
				called2 = true
				assert.Nil(t, p.Foo)
			})

			assert.True(t, called1)
			assert.True(t, called2)
		})

		t.Run("named", func(t *testing.T) {
			c := digtest.New(t)

			c.RequireProvide(func() *struct{} {
				return &struct{}{}
			}, dig.Name("foo"))

			called1 := false
			c.RequireInvoke(func(p param1) {
				called1 = true
				assert.NotNil(t, p.Foo)
			})

			called2 := false
			c.RequireInvoke(func(p param2) {
				called2 = true
				assert.NotNil(t, p.Foo)
			})

			assert.True(t, called1)
			assert.True(t, called2)
		})
	})

	t.Run("dynamically generated dig.In", func(t *testing.T) {
		// This test verifies that a dig.In generated using reflect.StructOf
		// works with our dig.In detection logic.
		c := digtest.New(t)

		type type1 struct{}
		type type2 struct{}

		var gave *type1
		new1 := func() *type1 {
			require.Nil(t, gave, "constructor must be called only once")
			gave = &type1{}
			return gave
		}

		c.RequireProvide(new1)

		// We generate a struct that embeds dig.In.
		//
		// Note that the fix for https://github.com/golang/go/issues/18780
		// requires that StructField.Name is always set but versions of Go
		// older than 1.9 expect Name to be empty for embedded fields.
		//
		// We use utils_for_go19_test and utils_for_pre_go19_test with build
		// tags to implement this behavior differently in the two Go versions.

		inType := reflect.StructOf([]reflect.StructField{
			{
				Name:      "In",
				Anonymous: true,
				Type:      reflect.TypeOf(dig.In{}),
			},
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

		c.RequireInvoke(fn.Interface())
	})

	t.Run("dynamically generated dig.Out", func(t *testing.T) {
		// This test verifies that a dig.Out generated using reflect.StructOf
		// works with our dig.Out detection logic.

		c := digtest.New(t)

		type A struct{ Value int }

		outType := reflect.StructOf([]reflect.StructField{
			{
				Name:      "Out",
				Anonymous: true,
				Type:      reflect.TypeOf(dig.Out{}),
			},
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
		c.RequireProvide(fn.Interface())

		type params struct {
			dig.In

			Foo *A `name:"foo"`
			Bar *A `name:"bar"`
			Baz *A `name:"baz" optional:"true"`
		}

		c.RequireInvoke(func(p params) {
			assert.Equal(t, &A{Value: 1}, p.Foo, "Foo must match")
			assert.Equal(t, &A{Value: 2}, p.Bar, "Bar must match")
			assert.Nil(t, p.Baz, "Baz must be unset")
		})
	})

	t.Run("variadic arguments invoke", func(t *testing.T) {
		c := digtest.New(t)

		type A struct{}

		var gaveA *A
		c.RequireProvide(func() *A {
			gaveA = &A{}
			return gaveA
		})

		c.RequireProvide(func() []*A {
			panic("[]*A constructor must not be called.")
		})

		c.RequireInvoke(func(a *A, as ...*A) {
			require.NotNil(t, a, "A must not be nil")
			require.True(t, a == gaveA, "A must match")
			require.Empty(t, as, "varargs must be empty")
		})
	})

	t.Run("variadic arguments dependency", func(t *testing.T) {
		c := digtest.New(t)

		type A struct{}
		type B struct{}

		var gaveA *A
		c.RequireProvide(func() *A {
			gaveA = &A{}
			return gaveA
		})

		c.RequireProvide(func() []*A {
			panic("[]*A constructor must not be called.")
		})

		var gaveB *B
		c.RequireProvide(func(a *A, as ...*A) *B {
			require.NotNil(t, a, "A must not be nil")
			require.True(t, a == gaveA, "A must match")
			require.Empty(t, as, "varargs must be empty")
			gaveB = &B{}
			return gaveB
		})

		c.RequireInvoke(func(b *B) {
			require.NotNil(t, b, "B must not be nil")
			require.True(t, b == gaveB, "B must match")
		})
	})

	t.Run("non-error return arguments from invoke are ignored", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{}
		type B struct{}

		c.RequireProvide(func() A { return A{} })
		c.RequireInvoke(func(A) B { return B{} })

		err := c.Invoke(func(B) {})
		require.Error(t, err, "invoking with B param should error out")
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".TestEndToEndSuccess.func\S+`,
			`dig_test.go:\d+`, // file:line
			"missing type:",
			"dig_test.B",
		)
	})
}

func TestGroups(t *testing.T) {
	t.Run("empty slice received without provides", func(t *testing.T) {
		c := digtest.New(t)

		type in struct {
			dig.In

			Values []int `group:"foo"`
		}

		c.RequireInvoke(func(i in) {
			require.Empty(t, i.Values)
		})
	})

	t.Run("values are provided", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Value int `group:"val"`
		}

		provide := func(i int) {
			c.RequireProvide(func() out {
				return out{Value: i}
			})
		}

		provide(1)
		provide(2)
		provide(3)

		type in struct {
			dig.In

			Values []int `group:"val"`
		}

		c.RequireInvoke(func(i in) {
			assert.Equal(t, []int{2, 3, 1}, i.Values)
		})
	})

	t.Run("groups are provided via option", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		provide := func(i int) {
			c.RequireProvide(func() int {
				return i
			}, dig.Group("val"))
		}

		provide(1)
		provide(2)
		provide(3)

		type in struct {
			dig.In

			Values []int `group:"val"`
		}

		c.RequireInvoke(func(i in) {
			assert.Equal(t, []int{2, 3, 1}, i.Values)
		})
	})

	t.Run("different types may be grouped", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		provide := func(i int, s string) {
			c.RequireProvide(func() (int, string) {
				return i, s
			}, dig.Group("val"))
		}

		provide(1, "a")
		provide(2, "b")
		provide(3, "c")

		type in struct {
			dig.In

			Ivalues []int    `group:"val"`
			Svalues []string `group:"val"`
		}

		c.RequireInvoke(func(i in) {
			assert.Equal(t, []int{2, 3, 1}, i.Ivalues)
			assert.Equal(t, []string{"a", "c", "b"}, i.Svalues)
		})
	})

	t.Run("group options may not be provided for result structs", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Value int `group:"val"`
		}

		func(i int) {
			require.Error(t, c.Provide(func() out {
				t.Fatal("This should not be called")
				return out{}
			}, dig.Group("val")), "This Provide should fail")
		}(1)
	})

	t.Run("constructor is called at most once", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Result string `group:"s"`
		}

		calls := make(map[string]int)

		provide := func(i string) {
			c.RequireProvide(func() out {
				calls[i]++
				return out{Result: i}
			})
		}

		provide("foo")
		provide("bar")
		provide("baz")

		type in struct {
			dig.In

			Results []string `group:"s"`
		}

		// Expected value of in.Results in consecutive calls.
		expected := [][]string{
			{"bar", "baz", "foo"},
			{"foo", "baz", "bar"},
			{"baz", "bar", "foo"},
			{"bar", "foo", "baz"},
		}

		for _, want := range expected {
			c.RequireInvoke(func(i in) {
				require.Equal(t, want, i.Results)
			})
		}

		for s, v := range calls {
			assert.Equal(t, 1, v, "constructor for %q called too many times", s)
		}
	})

	t.Run("consume groups in constructor", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Result []string `group:"hi"`
		}

		provideStrings := func(strings ...string) {
			c.RequireProvide(func() out {
				return out{Result: strings}
			})
		}

		provideStrings("1", "2")
		provideStrings("3", "4", "5")
		provideStrings("6")
		provideStrings("7", "8", "9", "10")

		type setParams struct {
			dig.In

			Strings [][]string `group:"hi"`
		}
		c.RequireProvide(func(p setParams) map[string]struct{} {
			m := make(map[string]struct{})
			for _, ss := range p.Strings {
				for _, s := range ss {
					m[s] = struct{}{}
				}
			}
			return m
		})

		c.RequireInvoke(func(got map[string]struct{}) {
			assert.Equal(t, map[string]struct{}{
				"1": {}, "2": {}, "3": {}, "4": {}, "5": {},
				"6": {}, "7": {}, "8": {}, "9": {}, "10": {},
			}, got)
		})
	})

	t.Run("provide multiple values", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type outInt struct {
			dig.Out
			Int int `group:"foo"`
		}

		provideInt := func(i int) {
			c.RequireProvide(func() (outInt, error) {
				return outInt{Int: i}, nil
			})
		}

		type outString struct {
			dig.Out
			String string `group:"foo"`
		}

		provideString := func(s string) {
			c.RequireProvide(func() outString {
				return outString{String: s}
			})
		}

		type outBoth struct {
			dig.Out

			Int    int    `group:"foo"`
			String string `group:"foo"`
		}

		provideBoth := func(i int, s string) {
			c.RequireProvide(func() (outBoth, error) {
				return outBoth{Int: i, String: s}, nil
			})
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
			dig.In

			Ints    []int    `group:"foo"`
			Strings []string `group:"foo"`
		}

		c.RequireInvoke(func(got in) {
			assert.Equal(t, in{
				Ints:    []int{5, 3, 4, 1, 6, 7, 2},
				Strings: []string{"foo", "bar", "baz", "quux", "qux"},
			}, got)
		})
	})

	t.Run("duplicate values are supported", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Result string `group:"s"`
		}

		provide := func(i string) {
			c.RequireProvide(func() out {
				return out{Result: i}
			})
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
			dig.In

			Strings stringSlice `group:"s"`
		}

		c.RequireInvoke(func(i in) {
			assert.Equal(t,
				stringSlice{"d", "c", "a", "a", "d", "e", "b", "a"},
				i.Strings)
		})
	})

	t.Run("failure to build a grouped value fails everything", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Result string `group:"x"`
		}

		c.RequireProvide(func() (out, error) {
			return out{Result: "foo"}, nil
		})

		var gaveErr error
		c.RequireProvide(func() (out, error) {
			gaveErr = errors.New("great sadness")
			return out{}, gaveErr
		})

		c.RequireProvide(func() out {
			return out{Result: "bar"}
		})

		type in struct {
			dig.In

			Strings []string `group:"x"`
		}

		err := c.Invoke(func(i in) {
			require.FailNow(t, "this function must not be called")
		})
		require.Error(t, err, "expected failure")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".TestGroups`,
			`could not build value group string\[group="x"\]:`,
			`received non-nil error from function "go.uber.org/dig_test".TestGroups\S+`,
			`dig_test.go:\d+`, // file:line
			"great sadness",
		)
		assert.Equal(t, gaveErr, dig.RootCause(err))
	})

	t.Run("flatten collects slices", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

		type out struct {
			dig.Out

			Value []int `group:"val,flatten"`
		}

		provide := func(i []int) {
			c.RequireProvide(func() out {
				return out{Value: i}
			})
		}

		provide([]int{1, 2})
		provide([]int{3, 4})

		type in struct {
			dig.In

			Values []int `group:"val"`
		}

		c.RequireInvoke(func(i in) {
			assert.Equal(t, []int{2, 3, 4, 1}, i.Values)
		})
	})

	t.Run("flatten via option", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))
		c.RequireProvide(func() []int {
			return []int{1, 2, 3}
		}, dig.Group("val,flatten"))

		type in struct {
			dig.In

			Values []int `group:"val"`
		}

		c.RequireInvoke(func(i in) {
			assert.Equal(t, []int{2, 3, 1}, i.Values)
		})
	})

	t.Run("flatten via option error if not a slice", func(t *testing.T) {
		c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))
		err := c.Provide(func() int { return 1 }, dig.Group("val,flatten"))
		require.Error(t, err, "failed to provide")
		assert.Contains(t, err.Error(), "flatten can be applied to slices only")
	})

	t.Run("a soft value group provider is not called when only that value group is consumed", func(t *testing.T) {
		type Param struct {
			dig.In

			Values []string `group:"foo,soft"`
		}
		type Result struct {
			dig.Out

			Value string `group:"foo"`
		}
		c := digtest.New(t)

		c.RequireProvide(func() (Result, int) {
			require.FailNow(t, "this function should not be called")
			return Result{Value: "sad times"}, 20
		})

		c.RequireInvoke(func(p Param) {
			assert.ElementsMatch(t, []string{}, p.Values)
		})
	})

	t.Run("soft value group is provided", func(t *testing.T) {
		type Param1 struct {
			dig.In

			Values []string `group:"foo,soft"`
		}
		type Result struct {
			dig.Out

			Value1 string `group:"foo"`
			Value2 int
		}

		c := digtest.New(t)
		c.RequireProvide(func() Result { return Result{Value1: "a", Value2: 2} })
		c.RequireProvide(func() string { return "b" }, dig.Group("foo"))

		// The only value that must be in the group is the one that's provided
		// because it would be provided anyways by another dependency, in
		// this case we need an int, so the first constructor is called, and
		// this provides a string, which is the one in the group
		c.RequireInvoke(func(p2 int, p1 Param1) {
			assert.ElementsMatch(t, []string{"a"}, p1.Values)
		})
	})

	t.Run("two soft group values provided by one constructor", func(t *testing.T) {
		type param struct {
			dig.In

			Value1 []string `group:"foo,soft"`
			Value2 []int    `group:"bar,soft"`
			Value3 float32
		}

		type result struct {
			dig.Out

			Result1 []string `group:"foo,flatten"`
			Result2 int      `group:"bar"`
		}
		c := digtest.New(t)

		c.RequireProvide(func() result {
			return result{
				Result1: []string{"a", "b", "c"},
				Result2: 4,
			}
		})
		c.RequireProvide(func() float32 { return 3.1416 })

		c.RequireInvoke(func(p param) {
			assert.ElementsMatch(t, []string{}, p.Value1)
			assert.ElementsMatch(t, []int{}, p.Value2)
			assert.Equal(t, float32(3.1416), p.Value3)
		})
	})
	t.Run("soft in a result value group", func(t *testing.T) {
		c := digtest.New(t)
		err := c.Provide(func() int { return 10 }, dig.Group("foo,soft"))
		require.Error(t, err, "failed to privide")
		assert.Contains(t, err.Error(), "cannot use soft with result value groups")
	})
	t.Run("value group provided after a hard dependency is provided", func(t *testing.T) {
		type Param struct {
			dig.In

			Value []string `group:"foo,soft"`
		}

		type Result struct {
			dig.Out

			Value1 string `group:"foo"`
		}

		c := digtest.New(t)
		c.Provide(func() (Result, int) { return Result{Value1: "a"}, 2 })

		c.RequireInvoke(func(param Param) {
			assert.ElementsMatch(t, []string{}, param.Value)
		})
		c.RequireInvoke(func(int) {})
		c.RequireInvoke(func(param Param) {
			assert.ElementsMatch(t, []string{"a"}, param.Value)
		})
	})
}

// --- END OF END TO END TESTS

func TestRecoverFromPanic(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*digtest.Container)
		invoke  interface{}
		wantErr []string
	}{
		{
			name: "panic in provided function",
			setup: func(c *digtest.Container) {
				c.RequireProvide(func() int {
					panic("terrible sadness")
				})
			},
			invoke: func(i int) {},
			wantErr: []string{
				`could not build arguments for function "go.uber.org/dig_test".TestRecoverFromPanic.\S+`,
				`failed to build int:`,
				`panic: "terrible sadness" in func: "go.uber.org/dig_test".TestRecoverFromPanic.\S+`,
			},
		},
		{
			name: "panic in decorator",
			setup: func(c *digtest.Container) {
				c.RequireProvide(func() string { return "" })
				c.RequireDecorate(func(s string) string {
					panic("great sadness")
				})
			},
			invoke: func(s string) {},
			wantErr: []string{
				`could not build arguments for function "go.uber.org/dig_test".TestRecoverFromPanic.\S+`,
				`failed to build string:`,
				`panic: "great sadness" in func: "go.uber.org/dig_test".TestRecoverFromPanic.\S+`,
			},
		},
		{
			name:   "panic in invoke",
			setup:  func(c *digtest.Container) {},
			invoke: func() { panic("terrible woe") },
			wantErr: []string{
				`panic: "terrible woe" in func: "go.uber.org/dig_test".TestRecoverFromPanic.\S+`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("without option", func(t *testing.T) {
				c := digtest.New(t)
				tt.setup(c)
				assert.Panics(t, func() { c.Container.Invoke(tt.invoke) },
					"expected panic without dig.RecoverFromPanics() option",
				)
			})

			t.Run("with option", func(t *testing.T) {
				c := digtest.New(t, dig.RecoverFromPanics())
				tt.setup(c)
				err := c.Container.Invoke(tt.invoke)
				require.Error(t, err)
				dig.AssertErrorMatches(t, err, tt.wantErr[0], tt.wantErr[1:]...)
				var pe dig.PanicError
				assert.True(t, errors.As(err, &pe), "expected error chain to contain a PanicError")
				_, ok := dig.RootCause(err).(dig.PanicError)
				assert.True(t, ok, "expected root cause to be a PanicError")
			})
		})
	}
}

func TestProvideConstructorErrors(t *testing.T) {
	t.Run("multiple-type constructor returns multiple objects of same type", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{}
		constructor := func() (*A, *A, error) {
			return &A{}, &A{}, nil
		}
		require.Error(t, c.Provide(constructor), "provide failed")
	})

	t.Run("constructor consumes a dig.Out", func(t *testing.T) {
		c := digtest.New(t)
		type out struct {
			dig.Out

			Reader io.Reader
		}

		type outPtr struct {
			*dig.Out

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
				msg:         `dig_test.out embeds a dig.Out`,
			},
			{
				desc:        "*dig.Out",
				constructor: func(*out) io.Writer { return nil },
				msg:         `\*dig_test.out embeds a dig.Out`,
			},
			{
				desc:        "embeds *dig.Out",
				constructor: func(outPtr) io.Writer { return nil },
				msg:         `dig_test.outPtr embeds a dig.Out`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				err := c.Provide(tt.constructor)
				require.Error(t, err, "provide should fail")
				dig.AssertErrorMatches(t, err,
					`cannot provide function "go.uber.org/dig_test".TestProvideConstructorErrors\S+`,
					`dig_test.go:\d+`, // file:line
					`bad argument 1:`,
					`cannot depend on result objects: `+tt.msg)
			})
		}
	})

	t.Run("name option cannot be provided for result structs", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{}

		type out struct {
			dig.Out

			A A
		}

		err := c.Provide(func() out {
			panic("this function must never be called")
		}, dig.Name("second"))
		require.Error(t, err)

		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".TestProvideConstructorErrors\S+`,
			`dig_test.go:\d+`, // file:line
			`bad result 1:`,
			"cannot specify a name for result objects: dig_test.out embeds dig.Out",
		)
	})

	t.Run("name tags on result structs are not allowed", func(t *testing.T) {
		c := digtest.New(t)

		type Result1 struct {
			dig.Out

			A string `name:"foo"`
		}

		type Result2 struct {
			dig.Out

			Result1 Result1 `name:"bar"`
		}

		err := c.Provide(func() Result2 {
			panic("this function should never be called")
		})
		require.Error(t, err)

		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".TestProvideConstructorErrors\S+`,
			`dig_test.go:\d+`, // file:line
			`bad field "Result1" of dig_test.Result2:`,
			"cannot specify a name for result objects: dig_test.Result1 embeds dig.Out",
		)
	})
}

func TestProvideRespectsConstructorErrors(t *testing.T) {
	t.Run("constructor succeeds", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() (*bytes.Buffer, error) {
			return &bytes.Buffer{}, nil
		})

		c.RequireInvoke(func(b *bytes.Buffer) {
			require.NotNil(t, b, "invoke got nil buffer")
		})
	})
	t.Run("constructor fails", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() (*bytes.Buffer, error) {
			return nil, errors.New("oh no")
		})

		var called bool
		err := c.Invoke(func(b *bytes.Buffer) { called = true })
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".TestProvideRespectsConstructorErrors\S+`,
			`dig_test.go:\d+`, // file:line
			`failed to build \*bytes.Buffer:`,
			`received non-nil error from function "go.uber.org/dig_test".TestProvideRespectsConstructorErrors\S+`,
			`dig_test.go:\d+`, // file:line
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
			c := digtest.New(t)
			assert.Error(t, c.Provide(tt.object))
		})
	}
}

func TestProvideWithWeirdNames(t *testing.T) {
	t.Parallel()

	t.Run("name with quotes", func(t *testing.T) {
		type type1 struct{ value int }

		c := digtest.New(t)

		c.RequireProvide(func() *type1 {
			return &type1{42}
		}, dig.Name(`foo"""bar`))

		type params struct {
			dig.In

			T *type1 `name:"foo\"\"\"bar"`
		}

		c.RequireInvoke(func(p params) {
			assert.Equal(t, &type1{value: 42}, p.T)
		})
	})

	t.Run("name with newline", func(t *testing.T) {
		type type1 struct{ value int }

		c := digtest.New(t)

		c.RequireProvide(func() *type1 {
			return &type1{42}
		}, dig.Name("foo\nbar"))

		type params struct {
			dig.In

			T *type1 `name:"foo\nbar"`
		}

		c.RequireInvoke(func(p params) {
			assert.Equal(t, &type1{value: 42}, p.T)
		})
	})
}

func TestProvideInvalidName(t *testing.T) {
	t.Parallel()

	c := digtest.New(t)
	err := c.Provide(func() io.Reader {
		panic("this function must not be called")
	}, dig.Name("foo`bar"))
	require.Error(t, err, "Provide must fail")
	assert.Contains(t, err.Error(), "invalid dig.Name(\"foo`bar\"): names cannot contain backquotes")
}

func TestProvideInvalidGroup(t *testing.T) {
	t.Parallel()

	c := digtest.New(t)
	err := c.Provide(func() io.Reader {
		panic("this function must not be called")
	}, dig.Group("foo`bar"))
	require.Error(t, err, "Provide must fail")
	assert.Contains(t, err.Error(), "invalid dig.Group(\"foo`bar\"): group names cannot contain backquotes")

	err = c.Provide(func() io.Reader {
		panic("this function must not be called")
	}, dig.Group("foo,bar"))
	require.Error(t, err, "Provide must fail")
	assert.Contains(t, err.Error(), `cannot parse group "foo,bar": invalid option "bar"`)
}

func TestProvideInvalidAs(t *testing.T) {
	ptrToStruct := &struct {
		name string
	}{
		name: "example",
	}
	type out struct {
		dig.Out

		name string
	}
	var nilInterface io.Reader

	tests := []struct {
		name        string
		param       interface{}
		expectedErr string
	}{
		{
			name:        "as param is not an type interface",
			param:       123,
			expectedErr: "invalid dig.As(int): argument must be a pointer to an interface",
		},
		{
			name:        "as param is a pointer to struct",
			param:       ptrToStruct,
			expectedErr: "invalid dig.As(*struct { name string }): argument must be a pointer to an interface",
		},
		{
			name:        "as param is a nil interface",
			param:       nilInterface,
			expectedErr: "invalid dig.As(nil): argument must be a pointer to an interface",
		},
		{
			name:        "as param is a nil",
			param:       nil,
			expectedErr: "invalid dig.As(nil): argument must be a pointer to an interface",
		},
		{
			name:        "as param is a func",
			param:       func() {},
			expectedErr: "invalid dig.As(func()): argument must be a pointer to an interface",
		},
		{
			name:        "as param is a func returning dig_test.out",
			param:       func() *out { return &out{name: "example"} },
			expectedErr: "invalid dig.As(func() *dig_test.out): argument must be a pointer to an interface",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := digtest.New(t)
			err := c.Provide(
				func() *bytes.Buffer {
					var buf bytes.Buffer
					return &buf
				},
				dig.As(tt.param),
			)

			require.Error(t, err, "provide must fail")
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestAsExpectingOriginalType(t *testing.T) {
	t.Parallel()

	t.Run("fail on expecting original type", func(t *testing.T) {
		c := digtest.New(t)

		c.RequireProvide(
			func() *bytes.Buffer {
				return bytes.NewBufferString("foo")
			},
			dig.As(new(io.Reader)),
			dig.Name("buff"))

		type in struct {
			dig.In

			Buffer *bytes.Buffer `name:"buff"`
			Reader io.Reader     `name:"buff"`
		}

		require.Error(t, c.Invoke(func(got in) {
			t.Fatal("*bytes.Buffer with name buff shouldn't be provided")
		}))
	})
}

func TestProvideIncompatibleOptions(t *testing.T) {
	t.Parallel()

	t.Run("group and name", func(t *testing.T) {
		c := digtest.New(t)
		err := c.Provide(func() io.Reader {
			panic("this function must not be called")
		}, dig.Group("foo"), dig.Name("bar"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use named values with value groups: "+
			`name:"bar" provided with group:"foo"`)
	})
}

type testStruct struct{}

func (testStruct) TestMethod(x int) float64 { return float64(x) }

func TestProvideLocation(t *testing.T) {
	t.Parallel()

	c := digtest.New(t)
	c.RequireProvide(func(x int) float64 {
		return testStruct{}.TestMethod(x)
	}, dig.LocationForPC(reflect.TypeOf(testStruct{}).Method(0).Func.Pointer()))

	err := c.Invoke(func(y float64) {})
	require.Error(t, err)
	require.Contains(t, err.Error(), `"go.uber.org/dig_test".testStruct.TestMethod`)
	require.Contains(t, err.Error(), `dig/dig_test.go`)
}

func TestCantProvideUntypedNil(t *testing.T) {
	t.Parallel()
	c := digtest.New(t)
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
			c := digtest.New(t)
			assert.Error(t, c.Provide(tt), "providing errors should fail")
		})
	}
}

func TestCantProvideParameterObjects(t *testing.T) {
	t.Parallel()

	t.Run("constructor", func(t *testing.T) {
		type Args struct{ dig.In }

		c := digtest.New(t)
		err := c.Provide(func() (Args, error) {
			panic("great sadness")
		})
		require.Error(t, err, "provide should fail")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".TestCantProvideParameterObjects\S+`,
			`dig_test.go:\d+`, // file:line
			"bad result 1:",
			"cannot provide parameter objects: dig_test.Args embeds a dig.In",
		)
	})

	t.Run("pointer from constructor", func(t *testing.T) {
		c := digtest.New(t)
		type Args struct{ dig.In }

		args := &Args{}

		err := c.Provide(func() (*Args, error) { return args, nil })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".TestCantProvideParameterObjects\S+`,
			`dig_test.go:\d+`, // file:line
			"bad result 1:",
			`cannot provide parameter objects: \*dig_test.Args embeds a dig.In`,
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
			c := digtest.New(t)
			c.RequireProvide(first)

			for _, second := range provideArgs {
				assert.Error(t, c.Provide(second), "second provide must fail")
			}
		})
	}
	t.Run("provide constructor twice", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func() *bytes.Buffer { return nil })
		assert.Error(t, c.Provide(func() *bytes.Buffer { return nil }))
	})
}

func TestDryModeSuccess(t *testing.T) {
	t.Run("does not call provides", func(t *testing.T) {
		type type1 struct{}
		provides := func() *type1 {
			t.Fatal("must not be called")
			return &type1{}
		}
		invokes := func(*type1) {}
		c := digtest.New(t, dig.DryRun(true))
		c.RequireProvide(provides)
		c.RequireInvoke(invokes)
	})
	t.Run("does not call invokes", func(t *testing.T) {
		type type1 struct{}
		provides := func() *type1 {
			t.Fatal("must not be called")
			return &type1{}
		}
		invokes := func(*type1) {
			t.Fatal("must not be called")
		}
		c := digtest.New(t, dig.DryRun(true))
		c.RequireProvide(provides)
		c.RequireInvoke(invokes)
	})
	t.Run("does not call decorators", func(t *testing.T) {
		type type1 struct{}
		provides := func() *type1 {
			t.Fatal("must not be called")
			return &type1{}
		}
		decorates := func(*type1) *type1 {
			t.Fatal("must not be called")
			return &type1{}
		}
		invokes := func(*type1) {}
		c := digtest.New(t, dig.DryRun(true))
		c.RequireProvide(provides)
		c.RequireDecorate(decorates)
		c.RequireInvoke(invokes)
	})
}

func TestProvideCycleFails(t *testing.T) {
	t.Run("not dry", func(t *testing.T) {
		testProvideCycleFails(t, false /* dry run */)
	})
	t.Run("dry", func(t *testing.T) {
		testProvideCycleFails(t, true /* dry run */)
	})
}

func testProvideCycleFails(t *testing.T, dryRun bool) {
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

		c := digtest.New(t, dig.DryRun(dryRun))
		c.RequireProvide(newA)
		c.RequireProvide(newB)
		err := c.Provide(newC)
		require.Error(t, err, "expected error when introducing cycle")
		require.True(t, dig.IsCycleDetected(err))
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideCycleFails.\S+`,
			`dig_test.go:\d+`, // file:line
			`this function introduces a cycle:`,
			`func\(\*dig_test.C\) \*dig_test.A provided by "go.uber.org/dig_test".testProvideCycleFails\S+ \(\S+\)`,
			`depends on func\(\*dig_test.B\) \*dig_test.C provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.A\) \*dig_test.B provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.C\) \*dig_test.A provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
		)
		assert.NotContains(t, err.Error(), "[scope")
		assert.Error(t, c.Invoke(func(c *C) {}), "expected invoking a function that uses a type that failed to provide to fail.")
	})

	t.Run("dig.In based cycle", func(t *testing.T) {
		// Same cycle as before but in terms of dig.Ins.

		type A struct{}
		type B struct{}
		type C struct{}

		type AParams struct {
			dig.In

			C C
		}
		newA := func(AParams) A { return A{} }

		type BParams struct {
			dig.In

			A A
		}
		newB := func(BParams) B { return B{} }

		type CParams struct {
			dig.In

			B B
			W io.Writer
		}
		newC := func(CParams) C { return C{} }

		c := digtest.New(t, dig.DryRun(dryRun))
		c.RequireProvide(newA)
		c.RequireProvide(newB)

		err := c.Provide(newC)
		require.Error(t, err, "expected error when introducing cycle")
		require.True(t, dig.IsCycleDetected(err))
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideCycleFails.\S+`,
			`dig_test.go:\d+`, // file:line
			`this function introduces a cycle:`,
			`func\(dig_test.AParams\) dig_test.A provided by "go.uber.org/dig_test".testProvideCycleFails\S+ \(\S+\)`,
			`depends on func\(dig_test.CParams\) dig_test.C provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(dig_test.BParams\) dig_test.B provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(dig_test.AParams\) dig_test.A provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
		)
		assert.Error(t, c.Invoke(func(c C) {}), "expected invoking a function that uses a type that failed to provide to fail.")
	})

	t.Run("group based cycle", func(t *testing.T) {
		type D struct{}

		type outA struct {
			dig.Out

			Foo string `group:"foo"`
			Bar int    `group:"bar"`
		}
		newA := func() outA {
			require.FailNow(t, "must not be called")
			return outA{}
		}

		type outB struct {
			dig.Out

			Foo string `group:"foo"`
		}
		newB := func(*D) outB {
			require.FailNow(t, "must not be called")
			return outB{}
		}

		type inC struct {
			dig.In

			Foos []string `group:"foo"`
		}

		type outC struct {
			dig.Out

			Bar int `group:"bar"`
		}

		newC := func(i inC) outC {
			require.FailNow(t, "must not be called")
			return outC{}
		}

		type inD struct {
			dig.In

			Bars []int `group:"bar"`
		}

		newD := func(inD) *D {
			require.FailNow(t, "must not be called")
			return nil
		}

		c := digtest.New(t)
		c.RequireProvide(newA)
		c.RequireProvide(newB)
		c.RequireProvide(newC)

		err := c.Provide(newD)
		require.Error(t, err)
		require.True(t, dig.IsCycleDetected(err))
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideCycleFails.\S+`,
			`dig_test.go:\d+`, // file:line
			`this function introduces a cycle:`,
			`func\(\*dig_test.D\) dig_test.outB provided by "go.uber.org/dig_test".testProvideCycleFails\S+ \(\S+\)`,
			`depends on func\(dig_test.inD\) \*dig_test.D provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(dig_test.inC\) dig_test.outC provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.D\) dig_test.outB provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
		)
	})

	t.Run("DeferAcyclicVerification bypasses cycle check, VerifyAcyclic catches cycle", func(t *testing.T) {
		// A <- B <- C <- D
		// |         ^
		// |_________|
		type A struct{}
		type B struct{}
		type C struct{}
		type D struct{}
		newA := func(*C) *A { return &A{} }
		newB := func(*A) *B { return &B{} }
		newC := func(*B) *C { return &C{} }
		newD := func(*C) *D { return &D{} }

		c := digtest.New(t, dig.DeferAcyclicVerification())
		c.RequireProvide(newA)
		c.RequireProvide(newB)
		c.RequireProvide(newC)
		c.RequireProvide(newD)

		err := c.Invoke(func(*A) {})
		require.Error(t, err, "expected error when introducing cycle")
		assert.True(t, dig.IsCycleDetected(err))
		dig.AssertErrorMatches(t, err,
			`cycle detected in dependency graph:`,
			`func\(\*dig_test.C\) \*dig_test.A provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.B\) \*dig_test.C provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.A\) \*dig_test.B provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.C\) \*dig_test.A provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
		)
	})

	t.Run("DeferAcyclicVerification eventually catches cycle with self-cycle", func(t *testing.T) {
		// A      <-- C <- D
		// |      |__^    ^
		// |______________|
		type A struct{}
		type C struct{}
		type D struct{}
		newA := func(*D) *A { return &A{} }
		newC := func(*C) *C { return &C{} }
		newD := func(*C) *D { return &D{} }

		c := digtest.New(t, dig.DeferAcyclicVerification())
		c.RequireProvide(newA)
		c.RequireProvide(newC)
		c.RequireProvide(newD)

		err := c.Invoke(func(*A) {})
		require.Error(t, err, "expected error when introducing cycle")
		assert.True(t, dig.IsCycleDetected(err))
		dig.AssertErrorMatches(t, err,
			`cycle detected in dependency graph:`,
			`func\(\*dig_test.C\) \*dig_test.C provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
			`depends on func\(\*dig_test.C\) \*dig_test.C provided by "go.uber.org/dig_test".testProvideCycleFails.\S+ \(\S+\)`,
		)
	})
}

func TestProvideErrNonCycle(t *testing.T) {
	c := digtest.New(t)
	type A struct{}
	type B struct{}
	newA := func() *A { return &A{} }

	c.RequireProvide(newA)
	err := c.Invoke(func(*B) {})
	require.Error(t, err)
	assert.False(t, dig.IsCycleDetected(err))
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

	c := digtest.New(t)
	c.RequireProvide(newA)
	c.RequireProvide(newC)
	c.RequireInvoke(func(*A) {})
}

func TestProvideFuncsWithoutReturnsFails(t *testing.T) {
	t.Parallel()

	c := digtest.New(t)
	assert.Error(t, c.Provide(func(*bytes.Buffer) {}))
}

func TestTypeCheckingEquality(t *testing.T) {
	type A struct{}
	type B struct {
		dig.Out
		A
	}
	type in struct {
		dig.In
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
		require.Equal(t, tt.isIn, dig.IsIn(tt.item))
		require.Equal(t, tt.isOut, dig.IsOut(tt.item))
	}
}

func TestInvokesUseCachedObjects(t *testing.T) {
	t.Parallel()

	c := digtest.New(t)

	constructorCalls := 0
	buf := &bytes.Buffer{}
	c.RequireProvide(func() *bytes.Buffer {
		assert.Equal(t, 0, constructorCalls, "constructor must not have been called before")
		constructorCalls++
		return buf
	})

	calls := 0
	for i := 0; i < 3; i++ {
		c.RequireInvoke(func(b *bytes.Buffer) {
			calls++
			require.Equal(t, 1, constructorCalls, "constructor must be called exactly once")
			require.Equal(t, buf, b, "invoke got different buffer pointer")
		})

		require.Equal(t, i+1, calls, "invoked function not called")
	}
}

func TestProvideFailures(t *testing.T) {
	t.Run("not dry", func(t *testing.T) {
		testProvideFailures(t, false /* dry run */)
	})
	t.Run("dry", func(t *testing.T) {
		testProvideFailures(t, true /* dry run */)
	})
}

func testProvideFailures(t *testing.T, dryRun bool) {
	t.Run("out returning multiple instances of the same type", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type A struct{ idx int }
		type ret struct {
			dig.Out

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
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`cannot provide dig_test.A from \[0\].A2:`,
			`already provided by \[0\].A1`,
		)
	})

	t.Run("out returning multiple instances of the same type and As option", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }
		type ret struct {
			dig.Out

			A1 A // same type A provided three times
			A2 A
			A3 A
		}

		err := c.Provide(func() ret {
			return ret{
				A1: A{idx: 1},
				A2: A{idx: 2},
				A3: A{idx: 3},
			}
		}, dig.As(new(interface{})))
		require.Error(t, err, "provide must return error")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`cannot provide interface {} from \[0\].A2:`,
			`already provided by \[0\].A1`,
		)
	})

	t.Run("provide multiple instances with the same name", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type A struct{}
		type ret1 struct {
			dig.Out
			*A `name:"foo"`
		}
		type ret2 struct {
			dig.Out
			*A `name:"foo"`
		}
		c.RequireProvide(func() ret1 {
			return ret1{A: &A{}}
		})

		err := c.Provide(func() ret2 {
			return ret2{A: &A{}}
		})
		require.Error(t, err, "expected error on the second provide")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`cannot provide \*dig_test.A\[name="foo"\] from \[0\].A:`,
			`already provided by "go.uber.org/dig_test".testProvideFailures\S+`,
		)
	})

	t.Run("out with unexported field should error", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		type A struct{ idx int }
		type out1 struct {
			dig.Out

			A1 A // should be ok
			a2 A // oops, unexported field. should generate an error
		}
		err := c.Provide(func() out1 { return out1{a2: A{77}} })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures\S+`,
			`dig_test.go:\d+`, // file:line
			"bad result 1:",
			`bad field "a2" of dig_test.out1:`,
			`unexported fields not allowed in dig.Out, did you mean to export "a2" \(dig_test.A\)\?`,
		)
	})

	t.Run("providing pointer to out should fail", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type out struct {
			dig.Out

			String string
		}
		err := c.Provide(func() *out { return &out{String: "foo"} })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures\S+`,
			`dig_test.go:\d+`, // file:line
			"bad result 1:",
			`cannot return a pointer to a result object, use a value instead: \*dig_test.out is a pointer to a struct that embeds dig.Out`,
		)
	})

	t.Run("embedding pointer to out should fail", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		type out struct {
			*dig.Out

			String string
		}

		err := c.Provide(func() out { return out{String: "foo"} })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures\S+`,
			`dig_test.go:\d+`, // file:line
			"bad result 1:",
			`cannot build a result object by embedding \*dig.Out, embed dig.Out instead: dig_test.out embeds \*dig.Out`,
		)
	})

	t.Run("provide the same implemented interface", func(t *testing.T) {
		c := digtest.New(t)
		err := c.Provide(
			func() *bytes.Buffer {
				var buf bytes.Buffer
				return &buf
			},
			dig.As(new(io.Reader)),
			dig.As(new(io.Reader)),
		)

		require.Error(t, err, "provide must fail")
		assert.Contains(t, err.Error(), "cannot provide io.Reader")
		assert.Contains(t, err.Error(), "already provided")
	})

	t.Run("provide the same implementation with as interface", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(
			func() *bytes.Buffer {
				var buf bytes.Buffer
				return &buf
			},
			dig.As(new(io.Reader)),
		)

		err := c.Provide(
			func() *bytes.Buffer {
				var buf bytes.Buffer
				return &buf
			},
			dig.As(new(io.Reader)),
		)

		require.Error(t, err, "provide must fail")
		assert.Contains(t, err.Error(), "cannot provide io.Reader")
		assert.Contains(t, err.Error(), "already provided")
	})

	t.Run("error should refer to location given by LocationForPC ProvideOption", func(t *testing.T) {
		c := digtest.New(t)
		type A struct{ idx int }
		type ret struct {
			dig.Out

			A1 A // same type A provided twice
			A2 A
		}

		locationFn := func() {}

		err := c.Provide(func() ret {
			return ret{
				A1: A{idx: 1},
				A2: A{idx: 2},
			}
		}, dig.LocationForPC(reflect.ValueOf(locationFn).Pointer()))
		require.Error(t, err, "provide must return error")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testProvideFailures.func\d+.1`,
		)
	})
}

func TestInvokeFailures(t *testing.T) {
	t.Run("not dry", func(t *testing.T) {
		testInvokeFailures(t, false /* dry run */)
	})
	t.Run("dry", func(t *testing.T) {
		testInvokeFailures(t, false /* dry run */)
	})
}

func testInvokeFailures(t *testing.T, dryRun bool) {
	t.Parallel()

	t.Run("invoke a non-function", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Invoke("foo")
		require.Error(t, err)
		dig.AssertErrorMatches(t, err, `can't invoke non-function foo \(type string\)`)
	})

	t.Run("untyped nil", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Invoke(nil)
		require.Error(t, err)
		dig.AssertErrorMatches(t, err, `can't invoke an untyped nil`)
	})

	t.Run("unmet dependency", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		err := c.Invoke(func(*bytes.Buffer) {})
		require.Error(t, err, "expected failure")
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`,
			`missing type:`,
			`\*bytes.Buffer`,
		)
	})

	t.Run("unmet required dependency", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}

		type args struct {
			dig.In

			T1 *type1 `optional:"true"`
			T2 *type2 `optional:"0"`
		}

		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Invoke(func(a args) {
			t.Fatal("function must not be called")
		})

		require.Error(t, err, "expected invoke error")
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`\*dig_test.type2`,
		)
	})

	t.Run("unmet named dependency", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type param struct {
			dig.In

			*bytes.Buffer `name:"foo"`
		}
		err := c.Invoke(func(p param) {
			t.Fatal("function should not be called")
		})
		require.Error(t, err, "invoke should fail")
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`\*bytes.Buffer\[name="foo"\]`,
		)
	})

	t.Run("unmet constructor dependency", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}

		type param struct {
			dig.In

			T1 *type1
			T2 *type2 `optional:"true"`
		}

		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func(p param) *type3 {
			panic("function must not be called")
		})

		err := c.Invoke(func(*type3) {
			t.Fatal("function must not be called")
		})
		require.Error(t, err, "invoke must fail")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`failed to build \*dig_test.type3:`,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`\*dig_test.type1`,
		)
		// We don't expect type2 to be mentioned in the list because it's
		// optional
	})

	t.Run("multiple unmet constructor dependencies", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}

		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() type2 {
			panic("function must not be called")
		})

		c.RequireProvide(func(type1, *type2) type3 {
			panic("function must not be called")
		})

		err := c.Invoke(func(type3) {
			t.Fatal("function must not be called")
		})

		require.Error(t, err, "invoke must fail")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`failed to build dig_test.type3:`,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing types:`,
			"dig_test.type1",
			`\*dig_test.type2 \(did you mean (to use )?dig_test.type2\?\)`,
		)
	})

	t.Run("invalid optional tag", func(t *testing.T) {
		type args struct {
			dig.In

			Buffer *bytes.Buffer `optional:"no"`
		}

		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Invoke(func(a args) {
			t.Fatal("function must not be called")
		})

		require.Error(t, err, "expected invoke error")
		dig.AssertErrorMatches(t, err,
			`bad field "Buffer" of dig_test.args:`,
			`invalid value "no" for "optional" tag on field Buffer:`,
		)
	})

	t.Run("constructor invalid optional tag", func(t *testing.T) {
		type type1 struct{}

		type nestedArgs struct {
			dig.In

			Buffer *bytes.Buffer `optional:"no"`
		}

		type args struct {
			dig.In

			Args nestedArgs
		}

		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Provide(func(a args) *type1 {
			panic("function must not be called")
		})

		require.Error(t, err, "expected provide error")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			"bad argument 1:",
			`bad field "Args" of dig_test.args:`,
			`bad field "Buffer" of dig_test.nestedArgs:`,
			`invalid value "no" for "optional" tag on field Buffer:`,
		)
	})

	t.Run("optional dep with unmet transitive dep", func(t *testing.T) {
		type missing struct{}
		type dep struct{}

		type params struct {
			dig.In

			Dep *dep `optional:"true"`
		}

		c := digtest.New(t, dig.DryRun(dryRun))

		// Container has a constructor for *dep, but that constructor has unmet
		// dependencies.
		c.RequireProvide(func(missing) *dep {
			panic("constructor for *dep should not be called")
		})

		// Should still be able to invoke a function that takes params, since *dep
		// is optional.
		var count int
		c.RequireInvoke(func(p params) {
			count++
			assert.Nil(t, p.Dep, "expected optional dependency to be unmet")
		})
		assert.Equal(t, 1, count, "expected invoke function to be called")
	})

	t.Run("optional dep with failed transitive dep", func(t *testing.T) {
		type failed struct{}
		type dep struct{}

		type params struct {
			dig.In

			Dep *dep `optional:"true"`
		}

		c := digtest.New(t, dig.DryRun(dryRun))

		errFailed := errors.New("failed")
		c.RequireProvide(func() (*failed, error) {
			return nil, errFailed
		})

		c.RequireProvide(func(*failed) *dep {
			panic("constructor for *dep should not be called")
		})

		// Should still be able to invoke a function that takes params, since *dep
		// is optional.
		err := c.Invoke(func(p params) {
			panic("shouldn't execute invoked function")
		})
		require.Error(t, err, "expected invoke error")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`failed to build \*dig_test.dep:`,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`failed to build \*dig_test.failed:`,
			`received non-nil error from function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`failed`,
		)
		assert.Equal(t, errFailed, dig.RootCause(err), "root cause must match")
	})

	t.Run("returned error", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Invoke(func() error { return errors.New("oh no") })
		require.Equal(t, errors.New("oh no"), err, "error must match")
	})

	t.Run("many returns", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		err := c.Invoke(func() (int, error) { return 42, errors.New("oh no") })
		require.Equal(t, errors.New("oh no"), err, "error must match")
	})

	t.Run("named instances are case sensitive", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type A struct{}
		type ret struct {
			dig.Out
			A `name:"CamelCase"`
		}
		type param1 struct {
			dig.In
			A `name:"CamelCase"`
		}
		type param2 struct {
			dig.In
			A `name:"camelcase"`
		}
		c.RequireProvide(func() ret { return ret{A: A{}} })
		c.RequireInvoke(func(param1) {})
		err := c.Invoke(func(param2) {})
		require.Error(t, err, "provide should return error since cases don't match")
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`dig_test.A\[name="camelcase"\]`)
	})

	t.Run("in unexported member gets an error", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type A struct{}
		type in struct {
			dig.In

			A1 A // all is good
			a2 A // oops, unexported type
		}

		_ = in{}.a2 // unused but needed for the test

		c.RequireProvide(func() A { return A{} })

		err := c.Invoke(func(i in) { assert.Fail(t, "should never get in here") })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			"bad argument 1:",
			`bad field "a2" of dig_test.in:`,
			`unexported fields not allowed in dig.In, did you mean to export "a2" \(dig_test.A\)\?`,
		)
	})

	t.Run("in unexported member gets an error on Provide", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type in struct {
			dig.In

			foo string
		}

		_ = in{}.foo // unused but needed for the test

		err := c.Provide(func(in) int { return 0 })
		require.Error(t, err, "Provide must fail")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".testInvokeFailures\S+`,
			`dig_test.go:\d+`, // file:line
			"bad argument 1:",
			`bad field "foo" of dig_test.in:`,
			`unexported fields not allowed in dig.In, did you mean to export "foo" \(string\)\?`,
		)
	})

	t.Run("embedded unexported member gets an error", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type A struct{}
		type Embed struct {
			dig.In

			A1 A // all is good
			a2 A // oops, unexported type
		}
		type in struct {
			Embed
		}

		_ = in{}.a2 // unused but needed for the test

		c.RequireProvide(func() A { return A{} })

		err := c.Invoke(func(i in) { assert.Fail(t, "should never get in here") })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			"bad argument 1:",
			`bad field "Embed" of dig_test.in:`,
			`bad field "a2" of dig_test.Embed:`,
			`unexported fields not allowed in dig.In, did you mean to export "a2" \(dig_test.A\)\?`,
		)
	})

	t.Run("embedded unexported member gets an error", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type param struct {
			dig.In

			string // embed an unexported std type
		}

		_ = param{}.string // unused but needed for the test

		err := c.Invoke(func(p param) { assert.Fail(t, "should never get here") })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			"bad argument 1:",
			`bad field "string" of dig_test.param:`,
			`unexported fields not allowed in dig.In, did you mean to export "string" \(string\)\?`,
		)
	})

	t.Run("pointer in dependency is not supported", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type in struct {
			dig.In

			String string
			Num    int
		}
		err := c.Invoke(func(i *in) { assert.Fail(t, "should never get here") })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			"bad argument 1:",
			`cannot depend on a pointer to a parameter object, use a value instead: \*dig_test.in is a pointer to a struct that embeds dig.In`,
		)
	})

	t.Run("embedding dig.In and dig.Out is not supported", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type in struct {
			dig.In
			dig.Out

			String string
		}

		err := c.Invoke(func(in) {
			assert.Fail(t, "should never get here")
		})
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			"bad argument 1:",
			"cannot depend on result objects: dig_test.in embeds a dig.Out",
		)
	})

	t.Run("embedding in pointer is not supported", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		type in struct {
			*dig.In

			String string
			Num    int
		}
		err := c.Invoke(func(i in) { assert.Fail(t, "should never get here") })
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			"bad argument 1:",
			`cannot build a parameter object by embedding \*dig.In, embed dig.In instead: dig_test.in embeds \*dig.In`,
		)
	})

	t.Run("requesting a value or pointer when other is present", func(t *testing.T) {
		type A struct{}
		type outA struct {
			dig.Out

			A `name:"hello"`
		}

		cases := []struct {
			name        string
			provide     interface{}
			invoke      interface{}
			errContains []string
		}{
			{
				name:    "value missing, pointer present",
				provide: func() *A { return &A{} },
				invoke:  func(A) {},
				errContains: []string{
					`missing type:`,
					`dig_test.A \(did you mean (to use )?\*dig_test.A\?\)`,
				},
			},
			{
				name:    "pointer missing, value present",
				provide: func() A { return A{} },
				invoke:  func(*A) {},
				errContains: []string{
					`missing type:`,
					`\*dig_test.A \(did you mean (to use )?dig_test.A\?\)`,
				},
			},
			{
				name:    "named pointer missing, value present",
				provide: func() outA { return outA{A: A{}} },
				invoke: func(struct {
					dig.In

					*A `name:"hello"`
				}) {
				},
				errContains: []string{
					`missing type:`,
					`\*dig_test.A\[name="hello"\] \(did you mean (to use )?dig_test.A\[name="hello"\]\?\)`,
				},
			},
		}

		for _, tc := range cases {
			c := digtest.New(t, dig.DryRun(dryRun))
			t.Run(tc.name, func(t *testing.T) {
				c.RequireProvide(tc.provide)

				err := c.Invoke(tc.invoke)
				require.Error(t, err)

				lines := append([]string{
					`dig_test.go:\d+`, // file:line
				}, tc.errContains...)
				dig.AssertErrorMatches(t, err,
					`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
					lines...)
			})
		}
	})

	t.Run("requesting an interface when an implementation is available", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))
		c.RequireProvide(bytes.NewReader)
		err := c.Invoke(func(io.Reader) {
			t.Fatalf("this function should not be called")
		})
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`io.Reader \(did you mean (to use )?\*bytes.Reader\?\)`,
		)
	})

	t.Run("requesting an interface when multiple implementations are available", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(bytes.NewReader)
		c.RequireProvide(bytes.NewBufferString)

		err := c.Invoke(func(io.Reader) {
			t.Fatalf("this function should not be called")
		})
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`io.Reader \(did you mean (to use one of )?\*bytes.Buffer, or \*bytes.Reader\?\)`,
		)
	})

	t.Run("requesting multiple interfaces when multiple implementations are available", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(bytes.NewReader)
		c.RequireProvide(bytes.NewBufferString)

		err := c.Invoke(func(io.Reader, io.Writer) {
			t.Fatalf("this function should not be called")
		})
		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing types:`,
			`io.Writer \(did you mean (to use )?\*bytes.Buffer\?\)`,
		)
	})

	t.Run("requesting a type when an interface is available", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() io.Writer { return nil })
		err := c.Invoke(func(*bytes.Buffer) {
			t.Fatalf("this function should not be called")
		})

		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`\*bytes.Buffer \(did you mean (to use )?io.Writer\?\)`,
		)
	})

	t.Run("requesting a type when multiple interfaces are available", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() io.Writer { return nil })
		c.RequireProvide(func() io.Reader { return nil })

		err := c.Invoke(func(*bytes.Buffer) {
			t.Fatalf("this function should not be called")
		})

		require.Error(t, err)
		dig.AssertErrorMatches(t, err,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`missing type:`,
			`\*bytes.Buffer \(did you mean (to use one of )?io.Reader, or io.Writer\?\)`,
		)
	})

	t.Run("direct dependency error", func(t *testing.T) {
		type A struct{}

		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() (A, error) {
			return A{}, errors.New("great sadness")
		})

		err := c.Invoke(func(A) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		dig.AssertErrorMatches(t, err,
			`received non-nil error from function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			`dig_test.go:\d+`, // file:line
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), dig.RootCause(err))
	})

	t.Run("transitive dependency error", func(t *testing.T) {
		type A struct{}
		type B struct{}

		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() (A, error) {
			return A{}, errors.New("great sadness")
		})

		c.RequireProvide(func(A) (B, error) {
			return B{}, nil
		})

		err := c.Invoke(func(B) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			"failed to build dig_test.B",
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures\S+`,
			"failed to build dig_test.A",
			`received non-nil error from function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			`dig_test.go:\d+`, // file:line
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), dig.RootCause(err))
	})

	t.Run("direct parameter object error", func(t *testing.T) {
		type A struct{}

		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() (A, error) {
			return A{}, errors.New("great sadness")
		})

		type params struct {
			dig.In

			A A
		}

		err := c.Invoke(func(params) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			"failed to build dig_test.A:",
			`received non-nil error from function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			`dig_test.go:\d+`, // file:line
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), dig.RootCause(err))
	})

	t.Run("transitive parameter object error", func(t *testing.T) {
		type A struct{}
		type B struct{}

		c := digtest.New(t, dig.DryRun(dryRun))

		c.RequireProvide(func() (A, error) {
			return A{}, errors.New("great sadness")
		})

		type params struct {
			dig.In

			A A
		}

		c.RequireProvide(func(params) (B, error) {
			return B{}, nil
		})

		err := c.Invoke(func(B) { panic("impossible") })

		require.Error(t, err, "expected Invoke error")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			`dig_test.go:\d+`, // file:line
			"failed to build dig_test.B:",
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			"failed to build dig_test.A:",
			`received non-nil error from function "go.uber.org/dig_test".testInvokeFailures.func\S+`,
			`dig_test.go:\d+`, // file:line
			"great sadness",
		)
		assert.Equal(t, errors.New("great sadness"), dig.RootCause(err))
	})

	t.Run("unmet dependency of a group value", func(t *testing.T) {
		c := digtest.New(t, dig.DryRun(dryRun))

		type A struct{}
		type B struct{}

		type out struct {
			dig.Out

			B B `group:"b"`
		}

		c.RequireProvide(func(A) out {
			require.FailNow(t, "must not be called")
			return out{}
		})

		type in struct {
			dig.In

			Bs []B `group:"b"`
		}

		err := c.Invoke(func(in) {
			require.FailNow(t, "must not be called")
		})
		require.Error(t, err, "expected failure")
		dig.AssertErrorMatches(t, err,
			`could not build arguments for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			`could not build value group dig_test.B\[group="b"\]:`,
			`missing dependencies for function "go.uber.org/dig_test".testInvokeFailures.\S+`,
			`dig_test.go:\d+`, // file:line
			"missing type:",
			"dig_test.A",
		)
	})
}

func TestFailingFunctionDoesNotCreateInvalidState(t *testing.T) {
	type type1 struct{}

	c := digtest.New(t)
	c.RequireProvide(func() (type1, error) {
		return type1{}, errors.New("great sadness")
	})

	require.Error(t, c.Invoke(func(type1) {
		require.FailNow(t, "first invoke must not call the function")
	}), "first invoke must fail")

	require.Error(t, c.Invoke(func(type1) {
		require.FailNow(t, "second invoke must not call the function")
	}), "second invoke must fail")
}

func BenchmarkProvideCycleDetection(b *testing.B) {
	// func TestBenchmarkProvideCycleDetection(b *testing.T) {
	type A struct{}

	type B struct{}
	type C struct{}
	type D struct{}

	type E struct{}
	type F struct{}
	type G struct{}

	type H struct{}
	type I struct{}
	type J struct{}

	type K struct{}
	type L struct{}
	type M struct{}

	type N struct{}
	type O struct{}
	type P struct{}

	type Q struct{}
	type R struct{}
	type S struct{}

	type T struct{}
	type U struct{}
	type V struct{}

	type W struct{}
	type X struct{}
	type Y struct{}

	type Z struct{}

	newA := func(*B, *C, *D) *A { return &A{} }

	newB := func(*E, *F, *G) *B { return &B{} }
	newC := func(*E, *F, *G) *C { return &C{} }
	newD := func(*E, *F, *G) *D { return &D{} }

	newE := func(*H, *I, *J) *E { return &E{} }
	newF := func(*H, *I, *J) *F { return &F{} }
	newG := func(*H, *I, *J) *G { return &G{} }

	newH := func(*K, *L, *M) *H { return &H{} }
	newI := func(*K, *L, *M) *I { return &I{} }
	newJ := func(*K, *L, *M) *J { return &J{} }

	newK := func(*N, *O, *P) *K { return &K{} }
	newL := func(*N, *O, *P) *L { return &L{} }
	newM := func(*N, *O, *P) *M { return &M{} }

	newN := func(*Q, *R, *S) *N { return &N{} }
	newO := func(*Q, *R, *S) *O { return &O{} }
	newP := func(*Q, *R, *S) *P { return &P{} }

	newQ := func(*T, *U, *V) *Q { return &Q{} }
	newR := func(*T, *U, *V) *R { return &R{} }
	newS := func(*T, *U, *V) *S { return &S{} }

	newT := func(*W, *X, *Y) *T { return &T{} }
	newU := func(*W, *X, *Y) *U { return &U{} }
	newV := func(*W, *X, *Y) *V { return &V{} }

	newW := func(*Z) *W { return &W{} }
	newX := func(*Z) *X { return &X{} }
	newY := func(*Z) *Y { return &Y{} }
	newZ := func() *Z { return &Z{} }

	for n := 0; n < b.N; n++ {
		c := digtest.New(b)
		c.Provide(newZ)
		c.Provide(newY)
		c.Provide(newX)
		c.Provide(newW)
		c.Provide(newV)
		c.Provide(newU)
		c.Provide(newT)
		c.Provide(newS)
		c.Provide(newR)
		c.Provide(newQ)
		c.Provide(newP)
		c.Provide(newO)
		c.Provide(newN)
		c.Provide(newM)
		c.Provide(newL)
		c.Provide(newK)
		c.Provide(newJ)
		c.Provide(newI)
		c.Provide(newH)
		c.Provide(newG)
		c.Provide(newF)
		c.Provide(newE)
		c.Provide(newD)
		c.Provide(newC)
		c.Provide(newB)
		c.Provide(newA)
	}
}

func TestUnexportedFieldsFailures(t *testing.T) {
	t.Run("empty tag value", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}

		constructor := func() (*type1, *type2) {
			return &type1{}, &type2{}
		}

		c := digtest.New(t)
		type param struct {
			dig.In `ignore-unexported:""`

			T1 *type1 // regular 'ol type
			T2 *type2 `optional:"true"` // optional type present in the graph
			t3 *type3
		}

		c.RequireProvide(constructor)
		err := c.Invoke(func(p param) {
			require.NotNil(t, p.T1, "whole param struct should not be nil")
			assert.NotNil(t, p.T2, "optional type in the graph should not return nil")
			_ = p.t3 // unused
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`bad argument 1: bad field "t3" of dig_test.param: unexported fields not allowed in dig.In, did you mean to export "t3" (*dig_test.type3)`)
	})

	t.Run("invalid tag value", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		constructor := func() (*type1, *type2) {
			return &type1{}, &type2{}
		}

		c := digtest.New(t)
		type param struct {
			dig.In `ignore-unexported:"foo"`

			T1 *type1 // regular 'ol type
			T2 *type2 `optional:"true"` // optional type present in the graph
			t3 *type3
		}

		c.RequireProvide(constructor)
		err := c.Invoke(func(p param) {
			require.NotNil(t, p.T1, "whole param struct should not be nil")
			assert.NotNil(t, p.T2, "optional type in the graph should not return nil")
			_ = p.t3
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`bad argument 1: invalid value "foo" for "ignore-unexported" tag on field In: strconv.ParseBool: parsing "foo": invalid syntax`)
	})
}

func TestProvideInfoOption(t *testing.T) {
	t.Parallel()
	t.Run("two outputs", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		ctor := func() (*type1, *type2) {
			return &type1{}, &type2{}
		}

		c := digtest.New(t)
		var info dig.ProvideInfo
		c.RequireProvide(ctor, dig.FillProvideInfo(&info))

		assert.Empty(t, info.Inputs)
		assert.Equal(t, 2, len(info.Outputs))

		assert.Equal(t, "*dig_test.type1", info.Outputs[0].String())
		assert.Equal(t, "*dig_test.type2", info.Outputs[1].String())
	})

	t.Run("two inputs and one output", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		ctor := func(*type1, *type2) *type3 {
			return &type3{}
		}
		c := digtest.New(t)
		var info dig.ProvideInfo
		c.RequireProvide(ctor, dig.Name("n"), dig.FillProvideInfo(&info))

		assert.Equal(t, 2, len(info.Inputs))
		assert.Equal(t, 1, len(info.Outputs))

		assert.Equal(t, `*dig_test.type3[name = "n"]`, info.Outputs[0].String())
		assert.Equal(t, "*dig_test.type1", info.Inputs[0].String())
		assert.Equal(t, "*dig_test.type2", info.Inputs[1].String())
	})

	t.Run("two inputs, output and error", func(t *testing.T) {
		type type1 struct{}
		type GatewayParams struct {
			dig.In

			WriteToConn  *io.Writer `name:"rw" optional:"true"`
			ReadFromConn *io.Reader `name:"ro"`
			ConnNames    []string   `group:"server"`
		}

		type type3 struct{}

		ctor := func(*type1, GatewayParams) (*type3, error) {
			return &type3{}, nil
		}
		c := digtest.New(t)
		var info dig.ProvideInfo
		c.RequireProvide(ctor, dig.FillProvideInfo(&info))

		assert.Equal(t, 4, len(info.Inputs))
		assert.Equal(t, 1, len(info.Outputs))

		assert.Equal(t, "*dig_test.type3", info.Outputs[0].String())
		assert.Equal(t, "*dig_test.type1", info.Inputs[0].String())
		assert.Equal(t, `*io.Writer[optional, name = "rw"]`, info.Inputs[1].String())
		assert.Equal(t, `*io.Reader[name = "ro"]`, info.Inputs[2].String())
		assert.Equal(t, `[]string[group = "server"]`, info.Inputs[3].String())
	})

	t.Run("two inputs, two outputs", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		type type4 struct{}
		ctor := func(*type1, *type2) (*type3, *type4) {
			return &type3{}, &type4{}
		}
		c := digtest.New(t)
		info := dig.ProvideInfo{}
		c.RequireProvide(ctor, dig.Group("g"), dig.FillProvideInfo(&info))

		assert.Equal(t, 2, len(info.Inputs))
		assert.Equal(t, 2, len(info.Outputs))

		assert.Equal(t, "*dig_test.type1", info.Inputs[0].String())
		assert.Equal(t, "*dig_test.type2", info.Inputs[1].String())

		assert.Equal(t, `*dig_test.type3[group = "g"]`, info.Outputs[0].String())
		assert.Equal(t, `*dig_test.type4[group = "g"]`, info.Outputs[1].String())
	})

	t.Run("two ctors", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		type type4 struct{}
		ctor1 := func(*type1) *type2 {
			return &type2{}
		}
		ctor2 := func(*type3) *type4 {
			return &type4{}
		}
		c := digtest.New(t)
		info1 := dig.ProvideInfo{}
		info2 := dig.ProvideInfo{}
		c.RequireProvide(ctor1, dig.FillProvideInfo(&info1))
		c.RequireProvide(ctor2, dig.FillProvideInfo(&info2))

		assert.NotEqual(t, info1.ID, info2.ID)

		assert.Equal(t, 1, len(info1.Inputs))
		assert.Equal(t, 1, len(info1.Outputs))
		assert.Equal(t, 1, len(info2.Inputs))
		assert.Equal(t, 1, len(info2.Outputs))

		assert.Equal(t, "*dig_test.type1", info1.Inputs[0].String())
		assert.Equal(t, "*dig_test.type2", info1.Outputs[0].String())

		assert.Equal(t, "*dig_test.type3", info2.Inputs[0].String())
		assert.Equal(t, "*dig_test.type4", info2.Outputs[0].String())
	})
}

func TestEndToEndSuccessWithAliases(t *testing.T) {
	t.Run("pointer constructor", func(t *testing.T) {
		type Buffer = *bytes.Buffer

		c := digtest.New(t)

		var b Buffer
		c.RequireProvide(func() *bytes.Buffer {
			b = &bytes.Buffer{}
			return b
		})

		c.RequireInvoke(func(got Buffer) {
			require.NotNil(t, got, "invoke got nil buffer")
			require.True(t, got == b, "invoke got wrong buffer")
		})
	})

	t.Run("duplicate provide", func(t *testing.T) {
		type A struct{}
		type B = A

		c := digtest.New(t)
		c.RequireProvide(func() A {
			return A{}
		})

		err := c.Provide(func() B { return B{} })
		require.Error(t, err, "B should fail to provide")
		dig.AssertErrorMatches(t, err,
			`cannot provide function "go.uber.org/dig_test".TestEndToEndSuccessWithAliases\S+`,
			`dig_test.go:\d+`, // file:line
			`cannot provide dig_test.A from \[0\]:`,
			`already provided by "go.uber.org/dig_test".TestEndToEndSuccessWithAliases\S+`,
		)
	})

	t.Run("duplicate provide with LocationForPC", func(t *testing.T) {
		c := digtest.New(t)
		c.RequireProvide(func(x int) float64 {
			return testStruct{}.TestMethod(x)
		}, dig.LocationForPC(reflect.TypeOf(testStruct{}).Method(0).Func.Pointer()))
		err := c.Provide(func(x int) float64 {
			return testStruct{}.TestMethod(x)
		}, dig.LocationForPC(reflect.TypeOf(testStruct{}).Method(0).Func.Pointer()))

		require.Error(t, err)
		require.Contains(t, err.Error(), `cannot provide function "go.uber.org/dig_test".testStruct.TestMethod`)
		require.Contains(t, err.Error(), `already provided by "go.uber.org/dig_test".testStruct.TestMethod`)
	})

	t.Run("named instances", func(t *testing.T) {
		c := digtest.New(t)
		type A1 struct{ s string }
		type A2 = A1
		type A3 = A2

		type ret struct {
			dig.Out

			A A1 `name:"a"`
			B A2 `name:"b"`
			C A3 `name:"c"`
		}

		type param struct {
			dig.In

			A1 A1 `name:"a"`
			B1 A2 `name:"b"`
			C1 A3 `name:"c"`

			A2 A3 `name:"a"`
			B2 A1 `name:"b"`
			C2 A2 `name:"c"`

			A3 A2 `name:"a"`
			B3 A3 `name:"b"`
			C3 A1 `name:"c"`
		}
		c.RequireProvide(func() ret {
			return ret{A: A2{"a"}, B: A3{"b"}, C: A1{"c"}}
		})

		c.RequireInvoke(func(p param) {
			assert.Equal(t, "a", p.A1.s, "A1 should match")
			assert.Equal(t, "b", p.B1.s, "B1 should match")
			assert.Equal(t, "c", p.C1.s, "C1 should match")

			assert.Equal(t, "a", p.A2.s, "A2 should match")
			assert.Equal(t, "b", p.B2.s, "B2 should match")
			assert.Equal(t, "c", p.C2.s, "C2 should match")

			assert.Equal(t, "a", p.A3.s, "A3 should match")
			assert.Equal(t, "b", p.B3.s, "B3 should match")
			assert.Equal(t, "c", p.C3.s, "C3 should match")
		})
	})
}
