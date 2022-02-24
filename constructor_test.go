// Copyright (c) 2022 Uber Technologies, Inc.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig/internal/digreflect"
)

func TestNewDotCtor(t *testing.T) {
	type t1 struct{}
	type t2 struct{}

	s := newScope()
	n, err := newConstructorNode(func(A t1) t2 { return t2{} }, s, s, constructorOptions{})
	require.NoError(t, err)

	n.location = &digreflect.Func{
		Name:    "function1",
		Package: "pkg1",
		File:    "file1",
		Line:    24534,
	}

	ctor := newDotCtor(n)
	assert.Equal(t, n.id, ctor.ID)
	assert.Equal(t, "function1", ctor.Name)
	assert.Equal(t, "pkg1", ctor.Package)
	assert.Equal(t, "file1", ctor.File)
	assert.Equal(t, 24534, ctor.Line)
}

func TestNodeAlreadyCalled(t *testing.T) {
	type type1 struct{}
	f := func() type1 { return type1{} }

	s := newScope()
	n, err := newConstructorNode(f, s, s, constructorOptions{})
	require.NoError(t, err, "failed to build node")
	require.False(t, n.called, "node must not have been called")

	c := New()
	require.NoError(t, n.Call(c.scope), "invoke failed")
	require.True(t, n.called, "node must be called")
	require.NoError(t, n.Call(c.scope), "calling again should be okay")
}
