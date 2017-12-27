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

package internal

import (
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupAndValueKey(t *testing.T) {
	typeOfReader := reflect.TypeOf((*io.Reader)(nil)).Elem()

	tests := []struct {
		desc       string
		give       Key
		wantString string
		wantType   reflect.Type
	}{
		{
			desc:       "simple value",
			give:       ValueKey{Type: typeOfReader},
			wantString: "io.Reader",
			wantType:   typeOfReader,
		},
		{
			desc:       "named value",
			give:       ValueKey{Name: "foo", Type: typeOfReader},
			wantString: `io.Reader[name="foo"]`,
			wantType:   typeOfReader,
		},
		{
			desc:       "name and type",
			give:       GroupKey{Name: "foo", Type: typeOfReader},
			wantString: `io.Reader[group="foo"]`,
			wantType:   typeOfReader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt.give.keyType() // lol coverage

			assert.Equal(t, tt.wantString, tt.give.String())
			assert.Equal(t, tt.wantType, tt.give.GetType())
		})
	}
}
