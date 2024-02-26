// Copyright (c) 2019 Uber Technologies, Inc.
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
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var generate = flag.Bool("generate", false, "generates output to testdata/ if set")

func VerifyVisualization(t *testing.T, testname string, c *Container, opts ...VisualizeOption) {
	var b bytes.Buffer
	require.NoError(t, Visualize(c, &b, opts...))
	got := b.Bytes()
	got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")) // normalize line endings

	dotFile := filepath.Join("testdata", testname+".dot")

	if *generate {
		err := os.WriteFile(dotFile, got, 0o644)
		require.NoError(t, err)
		return
	}

	want, err := os.ReadFile(dotFile)
	require.NoError(t, err)
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")) // normalize line endings

	assert.Equal(t, string(want), string(got),
		"Output did not match. Make sure you updated the testdata by running 'go test -generate'")
}
