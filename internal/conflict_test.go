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
	"github.com/stretchr/testify/require"
)

func TestKeySet(t *testing.T) {
	typeOfReader := reflect.TypeOf((*io.Reader)(nil)).Elem()

	ks := NewKeySet()

	t.Run("simple value without a conflict", func(t *testing.T) {
		require.NoError(t, ks.Provide("a", ValueKey{Type: typeOfReader}))
	})

	t.Run("named value without a conflict", func(t *testing.T) {
		require.NoError(t, ks.Provide("b", ValueKey{Name: "zzz", Type: typeOfReader}))
	})

	t.Run("group doesn't cause a conflict", func(t *testing.T) {
		require.NoError(t, ks.Provide("c", GroupKey{Name: "zzz", Type: typeOfReader}))
	})

	t.Run("simple value with a conflict", func(t *testing.T) {
		err := ks.Provide("d", ValueKey{Type: typeOfReader})
		require.Error(t, err, "expected failure")
		assert.Contains(t, err.Error(), "already provided by a")
	})

	t.Run("named value with a conflict", func(t *testing.T) {
		err := ks.Provide("d", ValueKey{Name: "zzz", Type: typeOfReader})
		require.Error(t, err, "expected failure")
		assert.Contains(t, err.Error(), "already provided by b")
	})

	t.Run("group still doesn't cause a conflict", func(t *testing.T) {
		require.NoError(t, ks.Provide("c", GroupKey{Name: "zzz", Type: typeOfReader}))
	})

	assert.Equal(t,
		[]Key{
			ValueKey{Type: typeOfReader},
			ValueKey{Name: "zzz", Type: typeOfReader},
			GroupKey{Name: "zzz", Type: typeOfReader},
			GroupKey{Name: "zzz", Type: typeOfReader},
			// ^this appears twice because we provided it twice
		}, ks.Items(), "items should match")
}
