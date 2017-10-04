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
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResultListErrors(t *testing.T) {
	tests := []struct {
		desc string
		give interface{}
		err  string
	}{
		{
			desc: "no results",
			give: func() {},
			err:  "must provide at least one non-error type",
		},
		{
			desc: "only error",
			give: func() error { panic("invalid") },
			err:  "must provide at least one non-error type",
		},
		{
			desc: "empty dig.Out",
			give: func() struct{ Out } { panic("invalid") },
			err:  "must provide at least one non-error type",
		},
		{
			desc: "returns dig.In",
			give: func() struct{ In } { panic("invalid") },
			err:  "bad result 1: cannot provide parameter objects",
		},
		{
			desc: "returns dig.Out+dig.In",
			give: func() struct {
				Out
				In
			} {
				panic("invalid")
			},
			err: "bad result 1: cannot provide parameter objects",
		},
		{
			desc: "type conflict",
			give: func() (io.Reader, io.Writer, io.Reader) { panic("invalid") },
			err:  "returns multiple io.Reader",
		},
		{
			desc: "name conflict",
			give: func() struct {
				Out

				NamedWriter   io.Writer `name:"what"`
				AnotherWriter io.Writer `name:"what"`
			} {
				panic("invalid")
			},
			err: "returns multiple io.Writer:what",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newResultList(reflect.TypeOf(tt.give))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestNewResultErrors(t *testing.T) {
	type outPtr struct{ *Out }
	type out struct{ Out }
	type in struct{ In }
	type inOut struct {
		In
		Out
	}

	tests := []struct {
		give interface{}
		err  string
	}{
		{
			give: outPtr{},
			err:  "cannot build a result object by embedding *dig.Out, embed dig.Out instead: dig.outPtr embeds *dig.Out",
		},
		{
			give: (*out)(nil),
			err:  "cannot return a pointer to a result object, use a value instead: *dig.out is a pointer to a struct that embeds dig.Out",
		},
		{
			give: in{},
			err:  "cannot provide parameter objects: dig.in embeds a dig.In",
		},
		{
			give: inOut{},
			err:  "cannot provide parameter objects: dig.inOut embeds a dig.In",
		},
	}

	for _, tt := range tests {
		give := reflect.TypeOf(tt.give)
		t.Run(fmt.Sprint(give), func(t *testing.T) {
			_, err := newResult(give)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestNewResultObjectErrors(t *testing.T) {
	tests := []struct {
		desc string
		give interface{}
		err  string
	}{
		{
			desc: "unexported fields",
			give: struct {
				Out

				writer io.Writer
			}{},
			err: `unexported fields not allowed in dig.Out, did you mean to export "writer" (io.Writer)`,
		},
		{
			desc: "error field",
			give: struct {
				Out

				Error error
			}{},
			err: `cannot provide errors from dig.Out: field "Error" (error)`,
		},
		{
			desc: "type conflict",
			give: struct {
				Out

				Reader io.Reader
				Writer io.Writer

				Nested struct {
					Out

					AnotherReader io.Reader
					AnotherWriter io.Writer `name:"conflict-free-writer"`
				}
			}{},
			err: "returns multiple io.Reader",
		},
		{
			desc: "name conflict",
			give: struct {
				Out

				Reader        io.Reader
				NamedWriter   io.Writer `name:"what"`
				AnotherWriter io.Writer `name:"what"`
			}{},
			err: "returns multiple io.Writer:what",
		},
		{
			desc: "nested dig.In",
			give: struct {
				Out

				Nested struct{ In }
			}{},
			err: `bad field "Nested"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newResultObject(reflect.TypeOf(tt.give))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}
