// Copyright (c) 2017 Uber Technologies, Inc.
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

package dig

import (
	"bytes"
	"fmt"
)

// String representation of the entire Container
func (c *Container) String() string {
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

func (n *node) String() string {
	deps := make([]string, len(n.deps))
	for i, d := range n.deps {
		if d.optional {
			// ~tally.Scope means optional
			// ~tally.Scope:foo means named optional
			deps[i] = fmt.Sprintf("~%v", d.key)
			continue
		}
		deps[i] = d.key.String()
	}
	return fmt.Sprintf(
		"deps: %v, ctor: %v", deps, n.ctype,
	)
}

func (k key) String() string {
	if k.name != "" {
		return fmt.Sprintf("%v:%s", k.t, k.name)
	}
	return k.t.String()
}
