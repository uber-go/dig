package graph

type Iterator interface {
	Order() int

	Visit(u int, do func(v int) bool)
}

func IsAcyclic(g Iterator) bool {
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
	return vertexCount == g.Order()
}
