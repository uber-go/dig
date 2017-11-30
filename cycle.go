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

	"go.uber.org/dig/internal/digreflect"
)

type cycleEntry struct {
	Key  key
	Func *digreflect.Func
}

type errCycleDetected struct {
	Path []cycleEntry
}

func (e errCycleDetected) Error() string {
	// We get something like,
	//
	//   foo provided by "path/to/package".NewFoo (path/to/file.go:42)
	//   	depends on bar provided by "another/package".NewBar (somefile.go:1)
	//   	depends on baz provided by "somepackage".NewBar (anotherfile.go:2)
	//   	depends on foo provided by "path/to/package".NewFoo (path/to/file.go:42)
	//
	b := new(bytes.Buffer)

	for i, entry := range e.Path {
		if i > 0 {
			b.WriteString("\n\tdepends on ")
		}
		fmt.Fprintf(b, "%v provided by %v", entry.Key, entry.Func)
	}
	return b.String()
}

func verifyAcyclic(c *Container, n *node, k key) error {
	err := detectCycles(n, c.providers, []cycleEntry{
		{Key: k, Func: n.Func},
	})
	if err != nil {
		err = errWrapf(err, "this function introduces a cycle")
	}
	return err
}

func detectCycles(n *node, graph map[key][]*node, path []cycleEntry) error {
	var err error
	walkParam(n.Params, paramVisitorFunc(func(param param) bool {
		if err != nil {
			return false
		}

		var k key
		switch p := param.(type) {
		case paramSingle:
			k = key{name: p.Name, t: p.Type}
		case paramGroupedSlice:
			// NOTE: The key uses the element type, not the slice type.
			k = key{group: p.Group, t: p.Type.Elem()}
		default:
			// Recurse for non-edge params.
			return true
		}

		entry := cycleEntry{Func: n.Func, Key: k}

		for _, p := range path {
			if p.Key == k {
				err = errCycleDetected{Path: append(path, entry)}
				return false
			}
		}

		for _, n := range graph[k] {
			if e := detectCycles(n, graph, append(path, entry)); e != nil {
				err = e
				return false
			}
		}

		return true
	}))

	return err
}
