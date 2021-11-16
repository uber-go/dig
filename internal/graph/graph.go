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

// Graph represents a simple interface for representation
// of a directed graph.
// It is assumed that each node in the graph is uniquely
// identified with an incremental integer (i.e. 0, 1, 2...).
type Graph interface {
	// Order returns the total number of nodes in the graph
	Order() int

	// EdgesFrom returns a list of integers that each
	// represents a node that has an edge from node u.
	EdgesFrom(u int) []int
}

// IsAcyclic uses depth-first search to find cycles
// in a generic graph represented by Iterator interface.
// If a cycle is found, it returns a list of nodes that
// are in the cyclic path, identified by their orders.
func IsAcyclic(g Graph) (bool, []int) {
	var cycleStart int
	var visited []bool
	var onStack []bool
	var backtrack map[int]int
	acyclic := true

	for i := 0; i < g.Order(); i++ {
		visited = make([]bool, g.Order())
		onStack = make([]bool, g.Order())
		backtrack = make(map[int]int, g.Order())

		acyclic, cycleStart = isAcyclicHelper(g, i, visited, onStack, backtrack)
		if !acyclic {
			break
		}
	}

	if acyclic {
		return true, nil
	}

	// compute cycle path using backtrack
	// Cycle is reverse-order.
	cycle := []int{cycleStart}
	curr := cycleStart
	for {
		curr = backtrack[curr]
		cycle = append([]int{curr}, cycle...)
		if curr == cycleStart {
			break
		}
	}
	return false, cycle
}

func isAcyclicHelper(
	g Graph,
	u int,
	visited []bool,
	onStack []bool,
	backtrack map[int]int,
) (bool, int) {
	visited[u] = true
	onStack[u] = true

	for _, v := range g.EdgesFrom(u) {
		if !visited[v] {
			backtrack[v] = u
			if ok, start := isAcyclicHelper(g, v, visited, onStack, backtrack); !ok {
				return ok, start
			}
		} else if onStack[v] {
			backtrack[v] = u
			return false, v
		}
	}
	onStack[u] = false
	return true, -1
}
