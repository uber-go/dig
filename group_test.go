// Copyright (c) 2020 Uber Technologies, Inc.
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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGroup(t *testing.T) {
	t.Run("no group tag on struct panics", func(t *testing.T) {
		require.Panics(t, func() { parseGroupTag(reflect.StructField{}) })
	})

	tests := []struct {
		name    string
		args    reflect.StructField
		wantG   group
		wantErr string
	}{
		{
			name:  "simple group",
			args:  reflect.StructField{Tag: `group:"somegroup"`},
			wantG: group{Name: "somegroup"},
		},
		{
			name:  "flattened group",
			args:  reflect.StructField{Tag: `group:"somegroup,flatten"`},
			wantG: group{Name: "somegroup", Flatten: true},
		},
		{
			name:    "error",
			args:    reflect.StructField{Tag: `group:"somegroup,abc"`},
			wantErr: `invalid option "abc" for "group" tag on field`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotG, err := parseGroupTag(tt.args)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantG, gotG)
		})
	}
}
