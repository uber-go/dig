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
	"os"
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

			privateContents contents
			Contents        contents
		}

		require.NoError(t,
			c.Provide(func(args Args) *bytes.Buffer {
				// testify's Empty doesn't work on string aliases for some
				// reason
				require.Len(t, args.privateContents, 0, "private contents must be empty")

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

			privateBuffer *bytes.Buffer

			*bytes.Buffer
		}

		require.NoError(t, c.Invoke(func(args Args) {
			require.Nil(t, args.privateBuffer, "private buffer must be nil")
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

			foo string // private field to be ignored
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
}

func TestProvideConstructorErrors(t *testing.T) {
	t.Run("multiple-type constructor returns multiple objects of same type", func(t *testing.T) {
		c := New()
		type A struct{}
		constructor := func() (*A, *A, error) {
			return &A{}, &A{}, nil
		}
		require.Error(t, c.Provide(constructor), "provide failed")
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
		assert.Contains(t, err.Error(), "oh no", "expected to bubble up constructor error")
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

func TestCanProvideErrorLikeType(t *testing.T) {
	t.Parallel()

	tests := []interface{}{
		func() *os.PathError { return &os.PathError{} },
		func() (*os.PathError, error) { return &os.PathError{}, nil },
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt), func(t *testing.T) {
			c := New()
			require.NoError(t, c.Provide(tt), "provide must not fail")

			require.NoError(t, c.Invoke(func(err *os.PathError) {
				assert.NotNil(t, err, "invoke received nil")
			}), "invoke must not fail")
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
		require.Contains(t, err.Error(), "can't provide parameter objects")
	})

	t.Run("pointer from constructor", func(t *testing.T) {
		type Args struct{ In }

		args := &Args{}

		c := New()
		require.NoError(t, c.Provide(func() (*Args, error) {
			return args, nil
		}), "provide failed")
		require.NoError(t, c.Invoke(func(a *Args) {
			require.True(t, args == a, "args must match")
		}), "invoke failed")
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
	require.Contains(t, err.Error(), "cycle")
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
		require.Contains(t, err.Error(), "returns multiple dig.A")
	})
}

func TestInvokeFailures(t *testing.T) {
	t.Parallel()

	t.Run("untyped nil", func(t *testing.T) {
		c := New()
		assert.Error(t, c.Invoke(nil))
	})

	t.Run("unmet dependency", func(t *testing.T) {
		c := New()
		assert.Error(t, c.Invoke(func(*bytes.Buffer) {}))
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
		require.Contains(t, err.Error(), "could not get field T2")
		require.Contains(t, err.Error(), "dig.type2 isn't in the container")
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
		require.Contains(t, err.Error(), "missing dependencies for type *dig.type3")
		require.Contains(t, err.Error(), "container is missing types: [*dig.type1]")
		// We don't expect type2 to be mentioned in the list because it's
		// optional
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
		require.Contains(t, err.Error(), `invalid value "no" for "optional" tag on field Buffer`)
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

		require.Error(t, err, "expected invoke error")
		require.Contains(t, err.Error(), `invalid value "no" for "optional" tag on field Buffer`)
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
}
