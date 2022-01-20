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

package dig

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/dig/internal/dot"
)

func (c *Container) CreateGraph() *dot.Graph {
	return c.createGraph()
}

func (s *Scope) CreateGraph() *dot.Graph {
	return s.createGraph()
}

type nestedErr struct {
	err error
}

var _ causer = nestedErr{}

func (e nestedErr) Error() string {
	return fmt.Sprint(e)
}

func (e nestedErr) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

func (e nestedErr) cause() error {
	return e.err
}

func (e nestedErr) writeMessage(w io.Writer, _ string) {
	io.WriteString(w, "oh no")
}

type visualizableErr struct{}

func (err visualizableErr) Error() string             { return "great sadness" }
func (err visualizableErr) updateGraph(dg *dot.Graph) {}

func TestCanVisualizeError(t *testing.T) {
	tests := []struct {
		desc         string
		err          error
		canVisualize bool
	}{
		{
			desc:         "unvisualizable error",
			err:          errf("great sadness"),
			canVisualize: false,
		},
		{
			desc:         "nested unvisualizable error",
			err:          nestedErr{err: errf("great sadness")},
			canVisualize: false,
		},
		{
			desc:         "visualizable error",
			err:          visualizableErr{},
			canVisualize: true,
		},
		{
			desc:         "nested visualizable error",
			err:          nestedErr{err: visualizableErr{}},
			canVisualize: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.canVisualize, CanVisualizeError(tt.err))
		})
	}
}
