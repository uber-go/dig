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

type t1 struct{}
type t2 struct{}
type t3 struct{}
type t4 struct{}

type helper struct{}

func (h helper) f1(c *Container) {
	c.Provide(func(A t1) t2 { return t2{} })
}

func (h helper) f2(c *Container) {
	c.Provide(func(A t1) t2 { return t2{} })
	c.Provide(func(A t1) t3 { return t3{} })
	c.Provide(func(A t2) t4 { return t4{} })
}

func (h helper) f3(c *Container) {
	c.Provide(func(A t1, B t2) (t3, t4) { return t3{}, t4{} })
}

func (h helper) f4(c *Container) {
	type in struct {
		In

		A t1
		B t2
	}

	type out struct {
		Out

		C t3
		D t4
	}
	c.Provide(func(i in) out { return out{Out{}, t3{}, t4{}} })
}

func (h helper) f5(c *Container) {
	type in struct {
		In
		A    t1
		Nest struct {
			In
			B    t2
			Nest struct {
				In
				C t3
			}
		}
	}

	c.Provide(func(p in) t4 { return t4{} })
}

func (h helper) f6(c *Container) {
	type nested1 struct {
		Out
		D t4
	}

	type nested2 struct {
		Out
		C    t3
		Nest nested1
	}

	type out struct {
		Out
		B    t2
		Nest nested2
	}

	c.Provide(func(A t1) out {
		return out{Out{}, t2{}, nested2{Out{}, t3{}, nested1{Out{}, t4{}}}}
	})
}

func (h helper) f7(c *Container) {
	type in struct {
		In

		D []t1 `group:"foo"`
	}

	type out struct {
		Out

		A t1 `group:"foo"`
		B t1 `group:"foo"`
		C t1 `group:"foo"`
	}

	c.Provide(func(B t2) out { return out{Out{}, t1{}, t1{}, t1{}} })
	c.Provide(func(i in) t3 { return t3{} })
}

func (h helper) f8(c *Container) {
	type in struct {
		In

		A t1 `name:"A"`
	}

	type out struct {
		Out

		B t2 `name:"B"`
	}

	c.Provide(func(i in) out { return out{B: t2{}} })
}

func (h helper) f9(c *Container) {
	type in struct {
		In

		A t1 `name:"A" optional:"true"`
		B t2 `name:"B"`
		C t3 `optional:"true"`
	}

	c.Provide(func(i in) t4 { return t4{} })
}
