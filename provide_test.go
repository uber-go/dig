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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvideOptionStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give ProvideOption
		want string
	}{
		{
			desc: "Name",
			give: Name("foo"),
			want: `Name("foo")`,
		},
		{
			desc: "Group",
			give: Group("bar"),
			want: `Group("bar")`,
		},
		{
			desc: "As",
			give: As(new(io.Reader), new(io.Writer)),
			want: `As(io.Reader, io.Writer)`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, fmt.Sprint(tt.give))
		})
	}
}

func TestFillProvideInfoString(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "FillProvideInfo(0x0)", fmt.Sprint(FillProvideInfo(nil)))
	})

	t.Run("not nil", func(t *testing.T) {
		t.Parallel()

		opt := FillProvideInfo(new(ProvideInfo))
		assert.NotEqual(t, fmt.Sprint(opt), "FillProvideInfo(0x0)")
		assert.Contains(t, fmt.Sprint(opt), "FillProvideInfo(0x")
	})
}

func TestLocationForPCString(t *testing.T) {
	opt := LocationForPC(reflect.ValueOf(func() {}).Pointer())
	assert.Contains(t, fmt.Sprint(opt), `LocationForPC("go.uber.org/dig".TestLocationForPCString.func1 `)
}

func TestExportString(t *testing.T) {
	assert.Equal(t, fmt.Sprint(Export(true)), "Export(true)")
	assert.Equal(t, fmt.Sprint(Export(false)), "Export(false)")
}
