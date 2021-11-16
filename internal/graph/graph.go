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

// IsAcyclic uses depth-first search to find cycles
// in a generic graph represented by Iterator interface.
// If a cycle is found, it returns a list of nodes that
// are in the cyclic path, identified by their orders.
func IsAcyclic(g Iterator) (bool, []int) {
	// special case
	if g.Order() < 1 {
		return true, nil
	}

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
