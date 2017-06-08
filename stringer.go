package dig

import (
	"bytes"
	"fmt"
)

// String representation of the entire Container
func (c Container) String() string {
	b := &bytes.Buffer{}
	fmt.Fprintln(b, "nodes: {")
	for k, v := range c.nodes {
		fmt.Fprintln(b, "\t", k, "->", v)
	}
	fmt.Fprintln(b, "}")

	fmt.Fprintln(b, "cache: {")
	for k, v := range c.cache {
		fmt.Fprintln(b, "\t", k, "=>", v)
	}
	fmt.Fprintln(b, "}")

	return b.String()
}

func (n node) String() string {
	deps := make([]string, len(n.deps))
	for i, d := range n.deps {
		deps[i] = fmt.Sprint(d.Type)
	}
	return fmt.Sprintf(
		"deps: %v, constructor: %v", deps, n.ctype,
	)
}
