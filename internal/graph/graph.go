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
// in a generic graph represented by Graph interface.
// If a cycle is found, it returns a list of nodes that
// are in the cyclic path, identified by their orders.
func IsAcyclic(g Graph) (bool, []int) {
	// cycleStart is a node that introduces a cycle in
	// the graph. Values in the range [0, g.Order()] mean
	// that there exists a cycle in g.
	cycleStart := -1
	info := newCycleInfo(g.Order())

	for i := 0; i < g.Order(); i++ {
		info.Reset()

		cycleStart = isAcyclic(g, i, info)
		if cycleStart >= 0 {
			break
		}
	}

	if cycleStart < 0 {
		return true, nil
	}

	// compute cycle path using backtrack
	cycle := []int{cycleStart}
	curr := cycleStart
	for {
		curr = info.backtrack[curr]
		cycle = append(cycle, curr)
		if curr == cycleStart {
			break
		}
	}

	// cycle is reverse-order.
	i, j := 0, len(cycle)-1
	for i < j {
		cycle[i], cycle[j] = cycle[j], cycle[i]
		i++
		j--
	}
	return false, cycle
}

// isAcyclic traverses the given graph starting from a specific node
// using depth-first search using recursion. If a cycle is detected,
// it returns the node that contains the "last" edge that introduces
// a cycle.
// For example, running isAcyclic starting from 1 on the following
// graph will return 3.
// 	1 -> 2 -> 3 -> 1
func isAcyclic(g Graph, u int, info *cycleInfo) int {
	info.visited[u] = true
	info.onStack[u] = true

	for _, v := range g.EdgesFrom(u) {
		if !info.visited[v] {
			info.backtrack[v] = u
			if start := isAcyclic(g, v, info); start >= 0 {
				return start
			}
		} else if info.onStack[v] {
			info.backtrack[v] = u
			return v
		}
	}
	info.onStack[u] = false
	return -1
}

// cycleInfo contains helpful info for cycle detection.
type cycleInfo struct {
	// order is the number of nodes in the graph
	order int

	// records whether ith node has been visited.
	visited []bool
	// records whether ith node is currently on the recursion stack.
	onStack []bool
	// back-tracks each edge info to form cycle path if one is detected.
	backtrack map[int]int
}

func newCycleInfo(order int) *cycleInfo {
	return &cycleInfo{order: order}
}

func (i *cycleInfo) Reset() {
	i.visited = make([]bool, i.order)
	i.onStack = make([]bool, i.order)
	i.backtrack = make(map[int]int, i.order)
}
