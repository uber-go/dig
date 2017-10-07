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
	"strings"
)

// String representation of the entire Container
func (c *Container) String() string {
	b := &bytes.Buffer{}
	fmt.Fprintln(b, "nodes: {")
	for k, vs := range c.producers {
		for _, v := range vs {
			fmt.Fprintln(b, "\t", k, "->", v)
		}
	}
	fmt.Fprintln(b, "}")

	fmt.Fprintln(b, "cache: {")
	for k, v := range c.values {
		fmt.Fprintln(b, "\t", k, "=>", v)
	}
	fmt.Fprintln(b, "}")

	return b.String()
}

func (n *node) String() string {
	return fmt.Sprintf("deps: %v, ctor: %v", n.Params, n.ctype)
}

func (k key) String() string {
	if k.name != "" {
		return fmt.Sprintf("%v:%s", k.t, k.name)
	}
	return k.t.String()
}

func (pl paramList) String() string {
	args := make([]string, len(pl.Params))
	for i, p := range pl.Params {
		args[i] = p.String()
	}
	return fmt.Sprint(args)
}

func (sp paramSingle) String() string {
	// ~tally.Scope means optional
	// ~tally.Scope:foo means named optional

	var prefix, suffix string
	if sp.Optional {
		prefix = "~"
	}
	if sp.Name != "" {
		suffix = ":" + sp.Name
	}

	return fmt.Sprintf("%s%v%s", prefix, sp.Type, suffix)
}

func (op paramObject) String() string {
	fields := make([]string, len(op.Fields))
	for i, f := range op.Fields {
		fields[i] = f.Param.String()
	}
	return strings.Join(fields, " ")
}
