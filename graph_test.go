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

package dig

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/dot"
)

func TestDotGraph(t *testing.T) {
	tparam := func(t reflect.Type, n string, g string, o bool) *dot.Param {
		return &dot.Param{
			Node: &dot.Node{
				Type:  t,
				Name:  n,
				Group: g,
			},
			Optional: o,
		}
	}

	tresult := func(t reflect.Type, n string, g string, gi int) *dot.Result {
		return &dot.Result{
			Node: &dot.Node{
				Type:  t,
				Name:  n,
				Group: g,
			},
			GroupIndex: gi,
		}
	}

	type t1 struct{}
	type t2 struct{}
	type t3 struct{}
	type t4 struct{}

	type1 := reflect.TypeOf(t1{})
	type2 := reflect.TypeOf(t2{})
	type3 := reflect.TypeOf(t3{})
	type4 := reflect.TypeOf(t4{})

	p1 := tparam(type1, "", "", false)
	p2 := tparam(type2, "", "", false)
	p3 := tparam(type3, "", "", false)
	p4 := tparam(type4, "", "", false)

	r1 := tresult(type1, "", "", 0)
	r2 := tresult(type2, "", "", 0)
	r3 := tresult(type3, "", "", 0)
	r4 := tresult(type4, "", "", 0)

	t.Parallel()

	t.Run("create graph with one constructor", func(t *testing.T) {
		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p1},
				Results: []*dot.Result{r2},
			},
		}

		c := New()
		c.Provide(func(A t1) t2 { return t2{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("create graph with multple constructors", func(t *testing.T) {
		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p1},
				Results: []*dot.Result{r2},
			},
			{
				Params:  []*dot.Param{p1},
				Results: []*dot.Result{r3},
			},
			{
				Params:  []*dot.Param{p2},
				Results: []*dot.Result{r4},
			},
		}

		c := New()
		c.Provide(func(A t1) t2 { return t2{} })
		c.Provide(func(A t1) t3 { return t3{} })
		c.Provide(func(A t2) t4 { return t4{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("constructor with multiple params and results", func(t *testing.T) {
		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p3, p4},
				Results: []*dot.Result{r1, r2},
			},
		}

		c := New()
		c.Provide(func(A t3, B t4) (t1, t2) { return t1{}, t2{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("param objects and result objects", func(t *testing.T) {
		type in struct {
			In

			A t1
			B t2
		}

		type out struct {
			Out

			C t3
			D t4
		}

		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p1, p2},
				Results: []*dot.Result{r3, r4},
			},
		}

		c := New()
		c.Provide(func(i in) out { return out{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("nested param object", func(t *testing.T) {
		type in struct {
			In

			A    t1
			Nest struct {
				In

				B    t2
				Nest struct {
					In

					C t3
				}
			}
		}

		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p1, p2, p3},
				Results: []*dot.Result{r4},
			},
		}

		c := New()
		c.Provide(func(p in) t4 { return t4{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("nested result object", func(t *testing.T) {
		type nested1 struct {
			Out

			D t4
		}

		type nested2 struct {
			Out

			C    t3
			Nest nested1
		}

		type out struct {
			Out

			B    t2
			Nest nested2
		}

		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p1},
				Results: []*dot.Result{r2, r3, r4},
			},
		}

		c := New()
		c.Provide(func(A t1) out { return out{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("value groups", func(t *testing.T) {
		type in struct {
			In

			D []t1 `group:"foo"`
		}

		type out1 struct {
			Out

			A t1 `group:"foo"`
		}

		type out2 struct {
			Out

			A t1 `group:"foo"`
		}

		res0 := tresult(type1, "", "foo", 0)
		res1 := tresult(type1, "", "foo", 1)

		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{p2},
				Results: []*dot.Result{res0},
			},
			{
				Params:  []*dot.Param{p4},
				Results: []*dot.Result{res1},
			},
			{
				GroupParams: []*dot.Group{
					{
						Type:    type1,
						Name:    "foo",
						Results: []*dot.Result{res0, res1},
					},
				},
				Results: []*dot.Result{r3},
			},
		}

		c := New()
		c.Provide(func(B t2) out1 { return out1{} })
		c.Provide(func(B t4) out2 { return out2{} })
		c.Provide(func(i in) t3 { return t3{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("named values", func(t *testing.T) {
		type in struct {
			In

			A t1 `name:"A"`
		}

		type out struct {
			Out

			B t2 `name:"B"`
		}

		expected := []*dot.Ctor{
			{
				Params: []*dot.Param{
					tparam(type1, "A", "", false),
				},
				Results: []*dot.Result{
					tresult(type2, "B", "", 0),
				},
			},
		}

		c := New()
		c.Provide(func(i in) out { return out{B: t2{}} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})

	t.Run("optional dependencies", func(t *testing.T) {
		type in struct {
			In

			A t1 `name:"A" optional:"true"`
			B t2 `name:"B"`
			C t3 `optional:"true"`
		}

		par1 := tparam(type1, "A", "", true)
		par2 := tparam(type2, "B", "", false)
		par3 := tparam(type3, "", "", true)

		expected := []*dot.Ctor{
			{
				Params:  []*dot.Param{par1, par2, par3},
				Results: []*dot.Result{r4},
			},
		}

		c := New()
		c.Provide(func(i in) t4 { return t4{} })

		dg := c.createGraph()
		assertCtorsEqual(t, expected, dg.Ctors)
	})
}

func assertCtorEqual(t *testing.T, expected *dot.Ctor, ctor *dot.Ctor) {
	assert.Equal(t, expected.Params, ctor.Params)
	assert.Equal(t, expected.Results, ctor.Results)
	assert.NotZero(t, ctor.Line)
}

func assertCtorsEqual(t *testing.T, expected []*dot.Ctor, ctors []*dot.Ctor) {
	for i, c := range ctors {
		assertCtorEqual(t, expected[i], c)
	}
}

func TestNewDotCtor(t *testing.T) {
	type t1 struct{}
	type t2 struct{}

	n, err := newNode(func(A t1) t2 { return t2{} }, nodeOptions{})
	require.NoError(t, err)

	n.location = &digreflect.Func{
		Name:    "function1",
		Package: "pkg1",
		File:    "file1",
		Line:    24534,
	}

	ctor := newDotCtor(n)
	assert.Equal(t, n.id, ctor.ID)
	assert.Equal(t, "function1", ctor.Name)
	assert.Equal(t, "pkg1", ctor.Package)
	assert.Equal(t, "file1", ctor.File)
	assert.Equal(t, 24534, ctor.Line)
}

func TestVisualize(t *testing.T) {
	type t1 struct{}
	type t2 struct{}
	type t3 struct{}
	type t4 struct{}

	t.Parallel()

	t.Run("empty graph in container", func(t *testing.T) {
		c := New()
		VerifyVisualization(t, "empty", c)
	})

	t.Run("simple graph", func(t *testing.T) {
		c := New()

		c.Provide(func() (t1, t2) { return t1{}, t2{} })
		c.Provide(func(A t1, B t2) (t3, t4) { return t3{}, t4{} })
		VerifyVisualization(t, "simple", c)
	})

	t.Run("named types", func(t *testing.T) {
		c := New()

		type in struct {
			In

			A t3 `name:"foo"`
		}
		type out1 struct {
			Out

			A t1 `name:"bar"`
			B t2 `name:"baz"`
		}
		type out2 struct {
			Out

			A t3 `name:"foo"`
		}

		c.Provide(func(in) out1 { return out1{} })
		c.Provide(func() out2 { return out2{} })
		VerifyVisualization(t, "named", c)
	})

	t.Run("optional params", func(t *testing.T) {
		c := New()

		type in struct {
			In

			A t1 `optional:"true"`
		}

		c.Provide(func() t1 { return t1{} })
		c.Provide(func(in) t2 { return t2{} })
		VerifyVisualization(t, "optional", c)
	})

	t.Run("grouped types", func(t *testing.T) {
		c := New()

		type in struct {
			In

			A []t3 `group:"foo"`
		}

		type out1 struct {
			Out

			A t3 `group:"foo"`
		}

		type out2 struct {
			Out

			A t3 `group:"foo"`
		}

		c.Provide(func() out1 { return out1{} })
		c.Provide(func() out2 { return out2{} })
		c.Provide(func(in) t2 { return t2{} })

		VerifyVisualization(t, "grouped", c)
	})

	t.Run("constructor fails with an error", func(t *testing.T) {
		c := New()

		type in1 struct {
			In

			C []t1 `group:"g1"`
		}

		type in2 struct {
			In

			A []t2 `group:"g2"`
			B t3   `name:"n3"`
		}

		type out1 struct {
			Out

			B t3 `name:"n3"`
			C t2 `group:"g2"`
		}

		type out2 struct {
			Out

			D t2 `group:"g2"`
		}

		type out3 struct {
			Out

			A t1 `group:"g1"`
			B t2 `group:"g2"`
		}

		c.Provide(func(in1) out1 { return out1{} })
		c.Provide(func(in2) t4 { return t4{} })
		c.Provide(func() out2 { return out2{} })
		c.Provide(func() (out3, error) { return out3{}, errf("great sadness") })
		err := c.Invoke(func(t4 t4) { return })

		VerifyVisualization(t, "error", c, VisualizeError(err))

		t.Run("non-failing graph nodes are pruned", func(t *testing.T) {

			t.Run("prune non-failing constructor result", func(t *testing.T) {
				c := New()
				c.Provide(func(in1) out1 { return out1{} })
				c.Provide(func(in2) t4 { return t4{} })
				c.Provide(func() (out2, error) { return out2{}, errf("great sadness") })
				c.Provide(func() out3 { return out3{} })
				err := c.Invoke(func(t4 t4) { return })

				VerifyVisualization(t, "prune_constructor_result", c, VisualizeError(err))
			})

			t.Run("if only the root node fails all node except for the root should be pruned", func(t *testing.T) {
				c := New()
				c.Provide(func(in1) out1 { return out1{} })
				c.Provide(func(in2) (t4, error) { return t4{}, errf("great sadness") })
				c.Provide(func() out2 { return out2{} })
				c.Provide(func() out3 { return out3{} })
				err := c.Invoke(func(t4 t4) { return })

				VerifyVisualization(t, "prune_non_root_nodes", c, VisualizeError(err))
			})
		})
	})

	t.Run("missing types", func(t *testing.T) {
		c := New()

		c.Provide(func(A t1, B t2, C t3) t4 { return t4{} })
		err := c.Invoke(func(t4 t4) { return })

		VerifyVisualization(t, "missing", c, VisualizeError(err))
	})

	t.Run("missing dependency", func(t *testing.T) {
		c := New()
		err := c.Invoke(func(t1 t1) { return })

		VerifyVisualization(t, "missingDep", c, VisualizeError(err))
	})
}

type visualizableErr struct{}

func (err visualizableErr) Error() string             { return "great sadness" }
func (err visualizableErr) updateGraph(dg *dot.Graph) {}

type nestedErr struct {
	err error
}

var _ causer = nestedErr{}

func (e nestedErr) Error() string {
	return fmt.Sprint(e)
}

func (e nestedErr) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

func (e nestedErr) cause() error {
	return e.err
}

func (e nestedErr) writeMessage(w io.Writer, _ string) {
	io.WriteString(w, "oh no")
}

func TestCanVisualizeError(t *testing.T) {
	tests := []struct {
		desc         string
		err          error
		canVisualize bool
	}{
		{
			desc:         "unvisualizable error",
			err:          errf("great sadness"),
			canVisualize: false,
		},
		{
			desc:         "nested unvisualizable error",
			err:          nestedErr{err: errf("great sadness")},
			canVisualize: false,
		},
		{
			desc:         "visualizable error",
			err:          visualizableErr{},
			canVisualize: true,
		},
		{
			desc:         "nested visualizable error",
			err:          nestedErr{err: visualizableErr{}},
			canVisualize: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.canVisualize, CanVisualizeError(tt.err))
		})
	}
}
