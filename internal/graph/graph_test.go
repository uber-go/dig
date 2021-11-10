package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Graph struct {
	Nodes map[int][]int
}

func newGraph() *Graph {
	return &Graph{
		Nodes: make(map[int][]int),
	}
}

func (g Graph) Order() int {
	return len(g.Nodes)
}

func (g Graph) Visit(u int, do func(v int) bool) {
	if _, ok := g.Nodes[u]; !ok {
		return
	}
	for _, v := range g.Nodes[u] {
		if ret := do(v); ret {
			return
		}
	}
}

// TODO (sungyoon): Refactor these into table tests.
func TestGraphIsAcyclic1(t *testing.T) {
	g := newGraph()
	g.Nodes[0] = []int{1, 2}
	g.Nodes[1] = []int{2}
	g.Nodes[2] = nil
	assert.True(t, IsAcyclic(g))
}

func TestGraphIsAcyclic2(t *testing.T) {
	g := newGraph()
	g.Nodes[0] = []int{1, 2, 3, 4, 5}
	g.Nodes[1] = []int{2, 4, 5}
	g.Nodes[2] = []int{3, 4, 5}
	g.Nodes[3] = []int{4, 5}
	g.Nodes[4] = []int{5}
	g.Nodes[5] = nil
	assert.True(t, IsAcyclic(g))
}

// TODO (sungyoon) maybe use randomly generated graph such that each iterator only has edges to numbers higher than its own degree?
func TestGraphIsAcyclic3(t *testing.T) {
	g := newGraph()
	g.Nodes[0] = nil
	g.Nodes[1] = nil
	g.Nodes[2] = nil
	assert.True(t, IsAcyclic(g))
}

func TestGraphIsCyclic1(t *testing.T) {
	g := newGraph()
	g.Nodes[0] = []int{1}
	g.Nodes[1] = []int{2}
	g.Nodes[2] = []int{3}
	g.Nodes[3] = []int{0}
	assert.False(t, IsAcyclic(g))
}

func TestGraphIsCyclic2(t *testing.T) {
	g := newGraph()
	g.Nodes[0] = []int{1, 2, 3}
	g.Nodes[1] = []int{0, 2, 3}
	g.Nodes[2] = []int{0, 1, 3}
	g.Nodes[3] = []int{0, 1, 2}
	assert.False(t, IsAcyclic(g))
}

func TestGraphIsCyclic3(t *testing.T) {
	g := newGraph()
	g.Nodes[0] = []int{0}
	assert.False(t, IsAcyclic(g))
}
