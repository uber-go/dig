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

import "testing"

func benchInvokeFunc(g1 *Grandchild1) (*Parent1, error) {
	return &Parent1{
		c1: &Child1{
			gc1: g1,
		},
	}, nil
}

func Benchmark_CtorInvoke(b *testing.B) {
	c := New()
	c.Provide(
		NewGrandchild1,
	)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Invoke(benchInvokeFunc)
	}
}

func Benchmark_CtorInvokeWithObjects(b *testing.B) {
	c := New()
	c.Provide(
		&Grandchild1{},
	)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Invoke(benchInvokeFunc)
	}
}

func Benchmark_InvokeCtorWithMapsAndSlices(b *testing.B) {
	c := New()
	c.Provide(
		testslice,
		testarray,
		testmap,
		threeObjects,
	)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Invoke(func(t1 []int, t2 [2]string, t3 map[string]int, c1 *Child1) {})
	}
}

func Benchmark_CtorProvideAndResolve(b *testing.B) {
	c := New()
	c.Provide(
		NewGrandchild1,
		benchInvokeFunc,
	)
	var p1 *Parent1
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Resolve(&p1)
	}
}

func Benchmark_CtorResolve(b *testing.B) {
	c := New()
	c.Provide(
		&Grandchild1{},
		benchInvokeFunc,
	)
	var p1 *Parent1
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Resolve(&p1)
	}
}

func Benchmark_ResolveCtors(b *testing.B) {
	c := New()
	c.Provide(
		NewParent1,
		NewChild1,
		NewGrandchild1,
	)
	var p1 Parent1
	var c1 Child1
	var g1 Grandchild1

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Resolve(&p1)
		c.Resolve(&c1)
		c.Resolve(&g1)
	}
}
