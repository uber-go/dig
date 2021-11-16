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

// Iterator TODO
type Iterator interface {
	Order() int

	Visit(u int, do func(v int) bool)
}

// IsAcyclic ...
func IsAcyclic(g Iterator) (bool, []int) {
	visited := make(map[int]bool)
	start := 0
	queue := []int{start}
	backtrack := make(map[int]int)

	var cycleStart int
	var curr int
	isAcyclic := true

	for len(queue) > 0 {
		curr = queue[0]
		queue = queue[1:]

		if visited[curr] {
			isAcyclic = false
			break
		}

		visited[curr] = true
		g.Visit(curr, func(v int) bool {
			backtrack[v] = curr
			queue = append(queue, v)
			// return false to do DFS, not BFS.
			return false
		})
	}
	if isAcyclic {
		return true, nil
	}

	cycle := []int{cycleStart}
	curr = cycleStart
	for {
		curr = backtrack[curr]
		cycle = append([]int{curr}, cycle...)
		if curr == cycleStart {
			break
		}
	}
	return false, cycle
}

// IsAcyclic2 verifies whether the given directed graph is acyclic using
// topological sort based on Kahn's algorithm (ref: Topological sorting
// of large networks: https://dl.acm.org/doi/abs/10.1145/368996.369025)
// If the graph is not acyclic, it returns a list of ints that identifies
// a cycle in the graph.
func IsAcyclic2(g Iterator) (bool, []int) {
	// use topological sort to check if DAG is acyclic.
	degrees := make([]int, g.Order())
	var q []int

	for u := range degrees {
		g.Visit(u, func(v int) bool {
			degrees[v]++
			return false
		})
	}

	// find roots (nodes w/o any other nodes depending on it)
	// to determine where to start traversing the graph from.
	for u, deg := range degrees {
		if deg == 0 {
			q = append(q, u)
		}
	}

	vertexCount := 0
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		vertexCount++
		g.Visit(u, func(v int) bool {
			degrees[v]--
			if degrees[v] == 0 {
				q = append(q, v)
			}
			return false
		})
	}

	if vertexCount == g.Order() {
		return true, nil
	}

	// If the graph contains a cycle, we can get the precise
	// cycle by examining each node's degree (nodes whose
	// degree is not 0 is part of the cycle).
	maxDegree := -1
	start := -1
	for u, degree := range degrees {
		if degree != 0 && degree > maxDegree {
			start = u
			maxDegree = degree
		}
	}

	// DFS to find cycle path from remaining cycles.
	curr := start
	backtrack := make(map[int]int)
	visited := make(map[int]bool)
	queue := []int{curr}

	for len(queue) > 0 {
		curr = queue[0]
		queue = queue[1:]

		g.Visit(curr, func(v int) bool {
			visited[curr] = true
			backtrack[v] = curr
			if !visited[v] {
				queue = append(queue, v)
			}
			return false
		})
	}

	var cycle []int
	for {
		cycle = append(cycle, curr)
		curr = backtrack[curr]
		if curr == start {
			cycle = append(cycle, curr)
			break
		}
	}
	return false, cycle
}
