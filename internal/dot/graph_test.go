// Copyright (c) 2018 Uber Technologies, Inc.
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

package dot

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type t1 struct{}
type t2 struct{}
type t3 struct{}

func TestNewGraph(t *testing.T) {
	g := NewGraph()

	assert.Equal(t, make(map[groupKey]*Group), g.Groups)
	assert.Equal(t, "*dot.Graph", reflect.TypeOf(g).String())
}

func TestNewGroup(t *testing.T) {
	type1 := reflect.TypeOf(t1{})

	k := groupKey{t: type1, group: "group1"}
	group := NewGroup(k)

	assert.Equal(t, type1, group.Type)
	assert.Equal(t, "group1", group.Name)
	assert.Equal(t, "*dot.Group", reflect.TypeOf(group).String())
}

func TestAddCtor(t *testing.T) {
	type1 := reflect.TypeOf(t1{})
	type2 := reflect.TypeOf(t2{})
	type3 := reflect.TypeOf([]t3{})

	n1 := &Node{Type: type1}
	n2 := &Node{Type: type2, Name: "bar"}
	n3 := &Node{Type: type3, Group: "foo"}

	p1 := &Param{Node: n1}
	p2 := &Param{Node: n2}
	p3 := &Param{Node: n3}

	r1 := &Result{Node: n1}
	r2 := &Result{Node: n2}

	t.Run("ungrouped params and results", func(t *testing.T) {
		dg := NewGraph()
		c := &Ctor{}
		params := []*Param{p1, p2}
		results := []*Result{r1, r2}

		dg.AddCtor(c, params, results)

		assert.Equal(t, []*Param{p1, p2}, c.Params)
		assert.Equal(t, []*Result{r1, r2}, c.Results)
	})

	t.Run("grouped params", func(t *testing.T) {
		dg := NewGraph()
		c := &Ctor{}
		params := []*Param{p3}

		k := groupKey{
			t:     type3.Elem(),
			group: "foo",
		}
		expectedGroup := &Group{
			Type: type3.Elem(),
			Name: "foo",
		}

		assert.Equal(t, map[groupKey]*Group{}, dg.Groups)
		dg.AddCtor(c, params, []*Result{})

		assert.Equal(t, 0, len(c.Params))
		assert.Equal(t, []*Group{expectedGroup}, c.GroupParams)
		assert.Equal(t, map[groupKey]*Group{k: expectedGroup}, dg.Groups)
	})

	t.Run("grouped results", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{}
		c1 := &Ctor{}
		node0 := &Node{Type: type3, Group: "foo"}
		node1 := &Node{Type: type3, Group: "foo"}
		res0 := &Result{Node: node0, GroupIndex: 0}
		res1 := &Result{Node: node1, GroupIndex: 1}

		k := groupKey{t: type3, group: "foo"}
		group0 := &Group{
			Type:    type3,
			Name:    "foo",
			Results: []*Result{res0},
		}
		group1 := &Group{
			Type:    type3,
			Name:    "foo",
			Results: []*Result{res0, res1},
		}

		assert.Equal(t, map[groupKey]*Group{}, dg.Groups)

		dg.AddCtor(c0, []*Param{}, []*Result{res0})
		assert.Equal(t, []*Result{res0}, c0.Results)
		assert.Equal(t, map[groupKey]*Group{k: group0}, dg.Groups)

		dg.AddCtor(c1, []*Param{}, []*Result{res1})
		assert.Equal(t, []*Result{res1}, c1.Results)
		assert.Equal(t, map[groupKey]*Group{k: group1}, dg.Groups)

		assert.Equal(t, []*Ctor{c0, c1}, dg.Ctors)
	})
}

func TestGetGroup(t *testing.T) {
	type1 := reflect.TypeOf(t1{})
	type2 := reflect.TypeOf(t2{})
	type3 := reflect.TypeOf(t3{})

	r1 := &Result{Node: &Node{Type: type1}}

	k1 := groupKey{t: type1, group: "group1"}
	k2 := groupKey{t: type2, group: "group1"}
	k3 := groupKey{t: type3, group: "group1"}

	g := NewGraph()
	group1 := NewGroup(k1)
	group2 := NewGroup(k2)
	group2.Results = append(group2.Results, r1)

	g.Groups[k1] = group1
	g.Groups[k2] = group2

	assert.Equal(t, group1, g.getGroup(k1))
	assert.Equal(t, group2, g.getGroup(k2))
	assert.Equal(t, NewGroup(k3), g.getGroup(k3))
}

func TestStringerAndAttribute(t *testing.T) {
	type1 := reflect.TypeOf(t1{})
	type2 := reflect.TypeOf(t2{})
	type3 := reflect.TypeOf(t3{})

	n1 := &Node{Type: type1}
	n2 := &Node{Type: type2, Name: "bar"}
	n3 := &Node{Type: type3, Group: "foo"}

	p1 := &Param{Node: n1}
	p2 := &Param{Node: n2}

	r1 := &Result{Node: n1}
	r2 := &Result{Node: n2}
	r3 := &Result{Node: n3, GroupIndex: 5}

	g1 := &Group{Type: reflect.TypeOf(t1{}), Name: "group1"}

	t.Parallel()

	t.Run("param stringer", func(t *testing.T) {
		assert.Equal(t, "dot.t1", p1.String())
		assert.Equal(t, "dot.t2[name=bar]", p2.String())
	})

	t.Run("result stringer", func(t *testing.T) {
		assert.Equal(t, "dot.t1", r1.String())
		assert.Equal(t, "dot.t2[name=bar]", r2.String())
		assert.Equal(t, "dot.t3[group=foo]5", r3.String())
	})

	t.Run("group stringer", func(t *testing.T) {
		assert.Equal(t, "[type=dot.t1 group=group1]", g1.String())
	})

	t.Run("param attributes", func(t *testing.T) {
		assert.Equal(t, "", p1.Attributes())
		assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Name: bar</FONT>`, p2.Attributes())
	})

	t.Run("result attributes", func(t *testing.T) {
		assert.Equal(t, "", r1.Attributes())
		assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Name: bar</FONT>`, r2.Attributes())
		assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Group: foo</FONT>`, r3.Attributes())
	})

	t.Run("group attributes", func(t *testing.T) {
		assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Group: group1</FONT>`, g1.Attributes())
	})
}
