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

package dot

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type t1 struct{}
type t2 struct{}
type t3 struct{}

func TestNewGroup(t *testing.T) {
	type1 := reflect.TypeOf(t1{})

	k := nodeKey{t: type1, group: "group1"}
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
		c := &Ctor{ID: 123}
		params := []*Param{p1, p2}
		results := []*Result{r1, r2}

		dg.AddCtor(c, params, results)

		assert.Equal(t, []*Param{p1, p2}, c.Params)
		assert.Equal(t, []*Result{r1, r2}, c.Results)
		assert.Equal(t, map[CtorID]*Ctor{123: c}, dg.ctorMap)
	})

	t.Run("grouped params", func(t *testing.T) {
		dg := NewGraph()
		c := &Ctor{ID: 1234}
		params := []*Param{p3}

		k := nodeKey{
			t:     type3.Elem(),
			group: "foo",
		}
		expectedGroup := NewGroup(k)

		assert.Equal(t, map[nodeKey]*Group{}, dg.groupMap)
		dg.AddCtor(c, params, []*Result{})

		assert.Equal(t, 0, len(c.Params))
		assert.Equal(t, []*Group{expectedGroup}, c.GroupParams)
		assert.Equal(t, map[nodeKey]*Group{k: expectedGroup}, dg.groupMap)
	})

	t.Run("grouped results", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 1234}
		c1 := &Ctor{ID: 5678}
		node0 := &Node{Type: type3, Group: "foo"}
		node1 := &Node{Type: type3, Group: "foo"}
		res0 := &Result{Node: node0, GroupIndex: 0}
		res1 := &Result{Node: node1, GroupIndex: 1}

		k := nodeKey{t: type3, group: "foo"}
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

		assert.Equal(t, map[nodeKey]*Group{}, dg.groupMap)

		dg.AddCtor(c0, []*Param{}, []*Result{res0})
		assert.Equal(t, []*Result{res0}, c0.Results)
		assert.Equal(t, map[nodeKey]*Group{k: group0}, dg.groupMap)

		dg.AddCtor(c1, []*Param{}, []*Result{res1})
		assert.Equal(t, []*Result{res1}, c1.Results)
		assert.Equal(t, map[nodeKey]*Group{k: group1}, dg.groupMap)

		assert.Equal(t, []*Ctor{c0, c1}, dg.Ctors)
	})
}

func TestFailNodes(t *testing.T) {
	type1 := reflect.TypeOf(&t1{})
	type2 := reflect.TypeOf(&t2{})

	n1 := &Node{Type: type1}
	n2 := &Node{Type: type2}
	n3 := &Node{Type: type1, Group: "foo"}
	n4 := &Node{Type: type2, Group: "bar"}

	r1 := &Result{Node: n1}
	r2 := &Result{Node: n2}
	r3 := &Result{Node: n3}
	r4 := &Result{Node: n4}

	t.Parallel()

	t.Run("missing nodes", func(t *testing.T) {
		dg := NewGraph()

		dg.AddMissingNodes([]*Result{r1, r2})
		assert.Equal(t, []*Result{r1, r2}, dg.Failed.RootCauses)
		assert.Equal(t, 0, len(dg.Failed.TransitiveFailures))

		dg.AddMissingNodes([]*Result{r3, r4})
		assert.Equal(t, []*Result{r1, r2}, dg.Failed.RootCauses)
		assert.Equal(t, []*Result{r3, r4}, dg.Failed.TransitiveFailures)
	})

	t.Run("fail nodes", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{}, []*Result{r1})
		dg.AddCtor(c1, []*Param{}, []*Result{r2})

		dg.FailNodes([]*Result{r1}, 123)
		assert.Equal(t, []*Result{r1}, dg.Failed.RootCauses)
		assert.Equal(t, 0, len(dg.Failed.TransitiveFailures))
		assert.Equal(t, rootCause, c0.ErrorType)

		dg.FailNodes([]*Result{r2}, 456)
		assert.Equal(t, []*Result{r1}, dg.Failed.RootCauses)
		assert.Equal(t, []*Result{r2}, dg.Failed.TransitiveFailures)
		assert.Equal(t, transitiveFailure, c1.ErrorType)
	})

	t.Run("fail group nodes", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}
		k0 := nodeKey{t: type1, group: "foo"}
		k1 := nodeKey{t: type2, group: "bar"}

		dg.AddCtor(c0, []*Param{}, []*Result{r3})
		dg.AddCtor(c1, []*Param{}, []*Result{r4})

		dg.FailGroupNodes("foo", type1, 123)
		assert.Equal(t, []*Result{r3}, dg.Failed.RootCauses)
		assert.Equal(t, 0, len(dg.Failed.TransitiveFailures))
		assert.Equal(t, rootCause, c0.ErrorType)
		assert.Equal(t, rootCause, dg.groupMap[k0].ErrorType)

		dg.FailGroupNodes("bar", type2, 456)
		assert.Equal(t, []*Result{r3}, dg.Failed.RootCauses)
		assert.Equal(t, []*Result{r4}, dg.Failed.TransitiveFailures)
		assert.Equal(t, transitiveFailure, c1.ErrorType)
		assert.Equal(t, transitiveFailure, dg.groupMap[k1].ErrorType)
	})
}

func TestPruneSuccess(t *testing.T) {
	type1 := reflect.TypeOf(&t1{})
	type2 := reflect.TypeOf(&t2{})
	type3 := reflect.TypeOf([]t3{})

	n1 := &Node{Type: type1}
	n2 := &Node{Type: type2}
	n3 := &Node{Type: type1, Group: "foo"}
	n4 := &Node{Type: type2, Group: "bar"}

	p1 := &Param{Node: n1}
	p2 := &Param{Node: n2}
	p3 := &Param{Node: n3}
	p4 := &Param{Node: n4}

	r1 := &Result{Node: n1}
	r2 := &Result{Node: n2}
	r3 := &Result{Node: n3}
	r4 := &Result{Node: n4}

	t.Parallel()

	t.Run("all graph entries without failing results should be removed", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 1234}
		c1 := &Ctor{ID: 5678}
		node0 := &Node{Type: type3, Group: "foo"}
		node1 := &Node{Type: type3, Group: "foo"}
		res0 := &Result{Node: node0, GroupIndex: 0}
		res1 := &Result{Node: node1, GroupIndex: 1}

		dg.AddCtor(c0, []*Param{}, []*Result{res0})
		dg.AddCtor(c1, []*Param{}, []*Result{res1})

		dg.PruneSuccess()

		assert.Len(t, dg.Ctors, 0)
		assert.Len(t, dg.Groups, 0)
	})

	t.Run("no constructors should be removed because they all have failing results", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{}, []*Result{r1})
		dg.AddCtor(c1, []*Param{}, []*Result{r2})

		dg.FailNodes([]*Result{r1}, c0.ID)
		dg.FailNodes([]*Result{r2}, c1.ID)
		dg.PruneSuccess()

		assert.Len(t, dg.Ctors, 2)
		assert.Len(t, dg.ctorMap, 2)
	})

	t.Run("remove constructor without failing results", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{}, []*Result{r1})
		dg.AddCtor(c1, []*Param{}, []*Result{r2})

		dg.FailNodes([]*Result{r2}, c1.ID)
		dg.PruneSuccess()

		assert.Len(t, dg.Ctors, 1)
	})

	t.Run("no graph entries should be removed if they all contain failing result groups", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{}, []*Result{r3})
		dg.AddCtor(c1, []*Param{}, []*Result{r4})

		dg.FailGroupNodes("foo", type1, c0.ID)
		dg.FailGroupNodes("bar", type2, c1.ID)
		dg.PruneSuccess()

		assert.Len(t, dg.Ctors, 2)
		assert.Len(t, dg.Groups, 2)
	})

	t.Run("only graph entries without failing result groups should be removed", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{}, []*Result{r3})
		dg.AddCtor(c1, []*Param{}, []*Result{r4})

		dg.FailGroupNodes("foo", type1, c0.ID)
		dg.PruneSuccess()

		assert.Len(t, dg.Ctors, 1)
		assert.Len(t, dg.Groups, 1)
	})

	t.Run("pruned controller results should be pruned from consuming controllers", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{p1, p2}, []*Result{r1})
		dg.AddCtor(c1, []*Param{}, []*Result{r2})
		assert.Len(t, c0.Params, 2)

		// r1 is failed to ensure that c1 is not removed.
		dg.FailNodes([]*Result{r1}, c0.ID)
		dg.PruneSuccess()

		assert.Len(t, c0.Params, 1)
	})

	t.Run("params from controllers that are not pruned should not be removed", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{p2}, []*Result{r1})
		dg.AddCtor(c1, []*Param{}, []*Result{r2})
		assert.Len(t, c0.Params, 1)

		dg.FailNodes([]*Result{r2}, c1.ID)
		dg.PruneSuccess()

		assert.Len(t, c0.Params, 1)
	})

	t.Run("pruned controller grouped results should be pruned from the consuming controllers and the group", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{p4}, []*Result{r3})
		dg.AddCtor(c1, []*Param{}, []*Result{r4})

		assert.Equal(t, len(c0.GroupParams), 1)

		group, ok := dg.groupMap[nodeKey{t: type2, group: "bar"}]
		assert.True(t, ok)

		assert.Equal(t, len(group.Results), 1)

		// r3 is failed to ensure that c0 is not removed.
		dg.FailNodes([]*Result{r3}, c0.ID)
		dg.PruneSuccess()

		assert.Len(t, c0.GroupParams, 0)
		assert.Len(t, group.Results, 0)
	})

	t.Run("grouped params from controllers that are not pruned should not be removed from the consuming controller nor the group", func(t *testing.T) {
		dg := NewGraph()
		c0 := &Ctor{ID: 123}
		c1 := &Ctor{ID: 456}

		dg.AddCtor(c0, []*Param{p4, p3}, []*Result{r3})
		dg.AddCtor(c1, []*Param{}, []*Result{r4})

		assert.Len(t, c0.GroupParams, 2)

		group, ok := dg.groupMap[nodeKey{t: type2, group: "bar"}]
		assert.True(t, ok)

		assert.Len(t, group.Results, 1)

		dg.FailNodes([]*Result{r4}, c1.ID)
		dg.PruneSuccess()

		assert.Len(t, c0.GroupParams, 2)
		assert.Len(t, group.Results, 1)
	})
}

func TestGetGroup(t *testing.T) {
	type1 := reflect.TypeOf(t1{})
	type2 := reflect.TypeOf(t2{})
	type3 := reflect.TypeOf(t3{})

	r1 := &Result{Node: &Node{Type: type1}}

	k1 := nodeKey{t: type1, group: "group1"}
	k2 := nodeKey{t: type2, group: "group1"}
	k3 := nodeKey{t: type3, group: "group1"}

	g := NewGraph()
	group1 := NewGroup(k1)
	group2 := NewGroup(k2)
	group2.Results = append(group2.Results, r1)

	g.groupMap[k1] = group1
	g.groupMap[k2] = group2

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
	g2 := &Group{Type: reflect.TypeOf(t2{}), Name: "group2", ErrorType: rootCause}
	g3 := &Group{Type: reflect.TypeOf(t3{}), Name: "group3", ErrorType: transitiveFailure}

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

	t.Run("result attributes", func(t *testing.T) {
		assert.Equal(t, `label=<dot.t1>`, r1.Attributes())
		assert.Equal(t, `label=<dot.t2<BR /><FONT POINT-SIZE="10">Name: bar</FONT>>`, r2.Attributes())
		assert.Equal(t, `label=<dot.t3<BR /><FONT POINT-SIZE="10">Group: foo</FONT>>`, r3.Attributes())
	})

	t.Run("group attributes", func(t *testing.T) {
		assert.Equal(t, `shape=diamond label=<dot.t1<BR /><FONT POINT-SIZE="10">Group: group1</FONT>>`, g1.Attributes())
		assert.Equal(t, `shape=diamond label=<dot.t2<BR /><FONT POINT-SIZE="10">Group: group2</FONT>> color=red`, g2.Attributes())
		assert.Equal(t, `shape=diamond label=<dot.t3<BR /><FONT POINT-SIZE="10">Group: group3</FONT>> color=orange`, g3.Attributes())
	})
}

func TestColor(t *testing.T) {
	assert.Equal(t, "black", noError.Color())
	assert.Equal(t, "red", rootCause.Color())
	assert.Equal(t, "orange", transitiveFailure.Color())
}
