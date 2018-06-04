// Copyright (c) 2018 Uber Technologies, Inc.
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

package dot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeString(t *testing.T) {
	n1 := &Node{Type: "t1"}
	n2 := &Node{Type: "t2", Name: "bar"}
	n3 := &Node{Type: "t3", Group: "foo"}

	assert.Equal(t, "t1", n1.String())
	assert.Equal(t, "t2[name=bar]", n2.String())
	assert.Equal(t, "t3[group=foo]", n3.String())
}

func TestAttributes(t *testing.T) {
	n1 := &Node{Type: "t1"}
	n2 := &Node{Type: "t2", Name: "bar"}
	n3 := &Node{Type: "t3", Group: "foo"}

	assert.Equal(t, "", n1.Attributes())
	assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Name: bar</FONT>`, n2.Attributes())
	assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Group: foo</FONT>`, n3.Attributes())
}
