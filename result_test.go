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
	}{
		{
			desc: "returns dig.In",
			give: func() struct{ In } { panic("invalid") },
		},
		{
			desc: "returns dig.Out+dig.In",
			give: func() struct {
				Out
				In
			} {
				panic("invalid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newResultList(reflect.TypeOf(tt.give), resultOptions{})
			require.Error(t, err)
			assertErrorMatches(t, err,
				"bad result 1:",
				"cannot provide parameter objects:",
				"embeds a dig.In")
		})
	}
}

func TestResultListExtractFails(t *testing.T) {
	rl, err := newResultList(reflect.TypeOf(func() (io.Writer, error) {
		panic("function should not be called")
	}), resultOptions{})
	require.NoError(t, err)
	assert.Panics(t, func() {
		rl.Extract(newStagingContainerWriter(), reflect.ValueOf("irrelevant"))
	})
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
			_, err := newResult(give, resultOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestNewResultObject(t *testing.T) {
	typeOfReader := reflect.TypeOf((*io.Reader)(nil)).Elem()
	typeOfWriter := reflect.TypeOf((*io.Writer)(nil)).Elem()

	tests := []struct {
		desc string
		give interface{}
		opts resultOptions

		wantFields []resultObjectField
	}{
		{desc: "empty", give: struct{ Out }{}},
		{
			desc: "multiple values",
			give: struct {
				Out

				Reader io.Reader
				Writer io.Writer
			}{},
			wantFields: []resultObjectField{
				{
					FieldName:  "Reader",
					FieldIndex: 1,
					Result:     resultSingle{Type: typeOfReader},
				},
				{
					FieldName:  "Writer",
					FieldIndex: 2,
					Result:     resultSingle{Type: typeOfWriter},
				},
			},
		},
		{
			desc: "name tag",
			give: struct {
				Out

				A io.Writer `name:"stream-a"`
				B io.Writer `name:"stream-b" `
			}{},
			wantFields: []resultObjectField{
				{
					FieldName:  "A",
					FieldIndex: 1,
					Result:     resultSingle{Name: "stream-a", Type: typeOfWriter},
				},
				{
					FieldName:  "B",
					FieldIndex: 2,
					Result:     resultSingle{Name: "stream-b", Type: typeOfWriter},
				},
			},
		},
		{
			desc: "group tag",
			give: struct {
				Out

				Writer io.Writer `group:"writers"`
			}{},
			wantFields: []resultObjectField{
				{
					FieldName:  "Writer",
					FieldIndex: 1,
					Result:     resultGrouped{Group: "writers", Type: typeOfWriter},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := newResultObject(reflect.TypeOf(tt.give), tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFields, got.Fields)
		})
	}

}

func TestNewResultObjectErrors(t *testing.T) {
	tests := []struct {
		desc string
		give interface{}
		opts resultOptions
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
			err: `bad field "Error" of struct { dig.Out; Error error }: cannot return an error here, return it from the constructor instead`,
		},
		{
			desc: "nested dig.In",
			give: struct {
				Out

				Nested struct{ In }
			}{},
			err: `bad field "Nested"`,
		},
		{
			desc: "group with name should fail",
			give: struct {
				Out

				Foo string `group:"foo" name:"bar"`
			}{},
			err: "cannot use named values with value groups: " +
				`name:"bar" provided with group:"foo"`,
		},
		{
			desc: "group marked as optional",
			give: struct {
				Out

				Foo string `group:"foo" optional:"true"`
			}{},
			err: "value groups cannot be optional",
		},
		{
			desc: "name option",
			give: struct {
				Out

				Reader io.Reader
			}{},
			opts: resultOptions{Name: "foo"},
			err:  `cannot specify a name for result objects`,
		},
		{
			desc: "name option with name tag",
			give: struct {
				Out

				A io.Writer `name:"stream-a"`
				B io.Writer
			}{},
			opts: resultOptions{Name: "stream"},
			err:  `cannot specify a name for result objects`,
		},
		{
			desc: "group tag with name option",
			give: struct {
				Out

				Reader io.Reader
				Writer io.Writer `group:"writers"`
			}{},
			opts: resultOptions{Name: "foo"},
			err:  `cannot specify a name for result objects`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newResultObject(reflect.TypeOf(tt.give), tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}
