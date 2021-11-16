// Copyright (c) 2021 Uber Technologies, Inc.
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

	"github.com/stretchr/testify/assert"
)

type TestGraph struct {
	Nodes map[int][]int
}

func newTestGraph() *TestGraph {
	return &TestGraph{
		Nodes: make(map[int][]int),
	}
}

func (g TestGraph) Order() int {
	return len(g.Nodes)
}

func (g TestGraph) EdgesFrom(u int) []int {
	return g.Nodes[u]
}

// TODO (sungyoon): Refactor these into table tests.
func TestGraphIsAcyclic1(t *testing.T) {
	g := newTestGraph()
	g.Nodes[0] = []int{1, 2}
	g.Nodes[1] = []int{2}
	g.Nodes[2] = nil
	ok, _ := IsAcyclic(g)
	assert.True(t, ok)
}

func TestGraphIsAcyclic2(t *testing.T) {
	g := newTestGraph()
	g.Nodes[0] = []int{1, 2, 3, 4, 5}
	g.Nodes[1] = []int{2, 4, 5}
	g.Nodes[2] = []int{3, 4, 5}
	g.Nodes[3] = []int{4, 5}
	g.Nodes[4] = []int{5}
	g.Nodes[5] = nil
	ok, _ := IsAcyclic(g)
	assert.True(t, ok)
}

// TODO (sungyoon) maybe use randomly generated graph such that each iterator only has edges to numbers higher than its own degree?
func TestGraphIsAcyclic3(t *testing.T) {
	g := newTestGraph()
	g.Nodes[0] = nil
	g.Nodes[1] = nil
	g.Nodes[2] = nil
	ok, _ := IsAcyclic(g)
	assert.True(t, ok)
}

func TestGraphIsCyclic1(t *testing.T) {
	g := newTestGraph()
	g.Nodes[0] = []int{1}
	g.Nodes[1] = []int{2}
	g.Nodes[2] = []int{3}
	g.Nodes[3] = []int{0}
	ok, cycle := IsAcyclic(g)
	assert.False(t, ok)
	assert.Contains(t, cycle, 0)
	assert.Contains(t, cycle, 1)
	assert.Contains(t, cycle, 2)
	assert.Contains(t, cycle, 3)
}

func TestGraphIsCyclic2(t *testing.T) {
	g := newTestGraph()
	g.Nodes[0] = []int{1, 2, 3}
	g.Nodes[1] = []int{0, 2, 3}
	g.Nodes[2] = []int{0, 1, 3}
	g.Nodes[3] = []int{0, 1, 2}
	ok, _ := IsAcyclic(g)
	assert.False(t, ok)
}

func TestGraphIsCyclic3(t *testing.T) {
	g := newTestGraph()
	g.Nodes[0] = []int{0}
	ok, cycle := IsAcyclic(g)
	assert.False(t, ok)
	assert.Contains(t, cycle, 0)
}
