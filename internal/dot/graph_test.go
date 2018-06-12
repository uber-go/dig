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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGraph(t *testing.T) {
	dg := NewGraph()

	assert.Equal(t, "*dot.Graph", reflect.TypeOf(dg).String())
	assert.Equal(t, make(map[key][]*Ctor), dg.ctorMap)
	assert.Equal(t, make(map[key][]*Node), dg.nodes)
	assert.Equal(t, make(map[key][]*Ctor), dg.subscribers)
}

func TestNodeString(t *testing.T) {
	n1 := &Node{Type: reflect.TypeOf("123")}
	n2 := &Node{Type: reflect.TypeOf(123), Name: "bar"}
	n3 := &Node{Type: reflect.TypeOf(123.0), Group: "foo"}

	assert.Equal(t, "string", n1.String())
	assert.Equal(t, "int[name=bar]", n2.String())
	assert.Equal(t, "float64[group=foo]0", n3.String())
}

func TestAttributes(t *testing.T) {
	n1 := &Node{Type: reflect.TypeOf(123)}
	n2 := &Node{Type: reflect.TypeOf(123), Name: "bar"}
	n3 := &Node{Type: reflect.TypeOf(123), Group: "foo", GroupIndex: 2}

	assert.Equal(t, "", n1.Attributes())
	assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Name: bar</FONT>`, n2.Attributes())
	assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Group: foo</FONT>`, n3.Attributes())
}
