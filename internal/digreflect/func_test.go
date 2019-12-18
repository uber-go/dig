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

package digreflect

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	myrepository "go.uber.org/dig/internal/digreflect/tests/myrepository.git"
	mypackage "go.uber.org/dig/internal/digreflect/tests/myrepository.git/mypackage"
)

func SomeExportedFunction() {}

func unexportedFunction() {}

func nestedFunctions() (nested1, nested2, nested3 func()) {
	// we call the functions to satisfy the linter.
	nested1 = func() {}
	nested2 = func() {
		nested3 = func() {}
	}
	nested2() // set nested3
	return
}

func TestInspectFunc(t *testing.T) {
	nested1, nested2, nested3 := nestedFunctions()

	tests := []struct {
		desc        string
		give        interface{}
		wantName    string
		wantPackage string

		// We don't match the exact file name because $GOPATH can be anywhere
		// on someone's system. Instead we'll match the suffix.
		wantFileSuffix string
	}{
		{
			desc:           "exported function",
			give:           SomeExportedFunction,
			wantName:       "SomeExportedFunction",
			wantPackage:    "go.uber.org/dig/internal/digreflect",
			wantFileSuffix: "/internal/digreflect/func_test.go",
		},
		{
			desc:           "unexported function",
			give:           unexportedFunction,
			wantName:       "unexportedFunction",
			wantPackage:    "go.uber.org/dig/internal/digreflect",
			wantFileSuffix: "/internal/digreflect/func_test.go",
		},
		{
			desc:           "nested function",
			give:           nested1,
			wantName:       "nestedFunctions.func1",
			wantPackage:    "go.uber.org/dig/internal/digreflect",
			wantFileSuffix: "/internal/digreflect/func_test.go",
		},
		{
			desc:           "second nested function",
			give:           nested2,
			wantName:       "nestedFunctions.func2",
			wantPackage:    "go.uber.org/dig/internal/digreflect",
			wantFileSuffix: "/internal/digreflect/func_test.go",
		},
		{
			desc:           "nested inside a nested function",
			give:           nested3,
			wantName:       "nestedFunctions.func2.1",
			wantPackage:    "go.uber.org/dig/internal/digreflect",
			wantFileSuffix: "/internal/digreflect/func_test.go",
		},
		{
			desc:           "inside a .git package",
			give:           myrepository.Hello,
			wantName:       "Hello",
			wantPackage:    "go.uber.org/dig/internal/digreflect/tests/myrepository.git",
			wantFileSuffix: "/internal/digreflect/tests/myrepository.git/hello.go",
		},
		{
			desc:           "subpackage of a .git package",
			give:           mypackage.Add,
			wantName:       "Add",
			wantPackage:    "go.uber.org/dig/internal/digreflect/tests/myrepository.git/mypackage",
			wantFileSuffix: "/internal/digreflect/tests/myrepository.git/mypackage/add.go",
		},
		{
			desc:           "dependency",
			give:           assert.Contains,
			wantName:       "Contains",
			wantPackage:    "github.com/stretchr/testify/assert",
			wantFileSuffix: "/assert/assertions.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f := InspectFunc(tt.give)
			assert.Equal(t, tt.wantName, f.Name, "function name did not match")
			assert.Equal(t, tt.wantPackage, f.Package, "package name did not match")

			assert.True(t, strings.HasSuffix(f.File, tt.wantFileSuffix),
				"file path %q does not end with src/%v", f.File, tt.wantFileSuffix)
		})
	}
}

func TestSplitFunc(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		pname, fname := splitFuncName("")
		assert.Empty(t, pname, "package name must be empty")
		assert.Empty(t, fname, "function name must be empty")
	})

	t.Run("vendored dependency", func(t *testing.T) {
		pname, fname := splitFuncName("go.uber.orgc/dig/vendor/example.com/foo/bar.Baz")
		assert.Equal(t, "example.com/foo/bar", pname)
		assert.Equal(t, "Baz", fname)
	})
}

func TestFuncFormatting(t *testing.T) {
	f := Func{
		Package: "foo/bar/baz",
		Name:    "Qux",
		File:    "src/foo/bar/baz/qux.go",
		Line:    42,
	}

	assert.Equal(t,
		`"foo/bar/baz".Qux (src/foo/bar/baz/qux.go:42)`,
		f.String(), "%v did not match")

	assert.Equal(t, `"foo/bar/baz".Qux
	src/foo/bar/baz/qux.go:42`, fmt.Sprintf("%+v", &f), "%v did not match")
}
