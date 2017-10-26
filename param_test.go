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
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamListBuild(t *testing.T) {
	p, err := newParamList(reflect.TypeOf(func() io.Writer { return nil }))
	require.NoError(t, err)
	assert.Panics(t, func() {
		p.Build(New())
	})
}

func TestParamObjectSuccess(t *testing.T) {
	type type1 struct{}
	type type2 struct{}
	type type3 struct{}

	type in struct {
		In

		T1 type1
		T2 type2 `optional:"true"`
		T3 type3 `name:"foo"`

		Nested struct {
			In

			A string
			B int32
		} `name:"bar"`
	}

	po, err := newParamObject(reflect.TypeOf(in{}))
	require.NoError(t, err)

	require.Len(t, po.Fields, 4)

	t.Run("no tags", func(t *testing.T) {
		require.Equal(t, "T1", po.Fields[0].FieldName)
		t1, ok := po.Fields[0].Param.(paramSingle)
		require.True(t, ok, "T1 must be a paramSingle")
		assert.Empty(t, t1.Name)
		assert.False(t, t1.Optional)

	})

	t.Run("optional field", func(t *testing.T) {
		require.Equal(t, "T2", po.Fields[1].FieldName)

		t2, ok := po.Fields[1].Param.(paramSingle)
		require.True(t, ok, "T2 must be a paramSingle")
		assert.Empty(t, t2.Name)
		assert.True(t, t2.Optional)

	})

	t.Run("named value", func(t *testing.T) {
		require.Equal(t, "T3", po.Fields[2].FieldName)
		t3, ok := po.Fields[2].Param.(paramSingle)
		require.True(t, ok, "T3 must be a paramSingle")
		assert.Equal(t, "foo", t3.Name)
		assert.False(t, t3.Optional)
	})

	t.Run("tags don't apply to nested dig.In", func(t *testing.T) {
		require.Equal(t, "Nested", po.Fields[3].FieldName)
		nested, ok := po.Fields[3].Param.(paramObject)
		require.True(t, ok, "Nested must be a paramObject")

		assert.Len(t, nested.Fields, 2)
		a, ok := nested.Fields[0].Param.(paramSingle)
		require.True(t, ok, "Nested.A must be a paramSingle")
		assert.Empty(t, a.Name, "Nested.A must not have a name")
	})
}

func TestParamObjectFailure(t *testing.T) {
	t.Run("unexported field gets an error", func(t *testing.T) {
		type A struct{}
		type in struct {
			In

			A1 A
			a2 A
		}

		_, err := newParamObject(reflect.TypeOf(in{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`bad field "a2" of dig.in: unexported fields not allowed in dig.In, did you mean to export "a2" (dig.A)`)
	})
}

func TestParamGroupSliceErrors(t *testing.T) {
	tests := []struct {
		desc    string
		shape   interface{}
		wantErr string
	}{
		{
			desc: "non-slice type are disallowed",
			shape: struct {
				In

				Foo string `group:"foo"`
			}{},
			wantErr: "value groups may be consumed as slices only: " +
				`field "Foo" (string) is not a slice`,
		},
		{
			desc: "cannot provide name for a group",
			shape: struct {
				In

				Foo []string `group:"foo" name:"bar"`
			}{},
			wantErr: "cannot use named values with value groups: " +
				`name:"bar" requested with group:"foo"`,
		},
		{
			desc: "cannot be optional",
			shape: struct {
				In

				Foo []string `group:"foo" optional:"true"`
			}{},
			wantErr: "value groups cannot be optional",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newParamObject(reflect.TypeOf(tt.shape))
			require.Error(t, err, "expected failure")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
