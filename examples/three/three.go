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

package main

import (
	"fmt"

	"go.uber.org/dig"
)

type one struct{}
type two struct {
	*one
}
type three struct {
	*one
	*two
}

func newOne() *one {
	fmt.Println("new *one is created")
	return &one{}
}
func newTwo(o1 *one) *two {
	fmt.Println("new *two is created")
	return &two{one: o1}
}
func newThree(o1 *one, o2 *two) *three {
	fmt.Println("new *three is created")
	return &three{one: o1, two: o2}
}

func main() {
	c := dig.New()

	// Register all the constructors in a dig container.
	//
	// At this point no functions are called and no objects are created.
	// dig is merely constructing a directional graph of dependencies.
	err := c.Provide(newOne, newTwo, newThree)
	if err != nil {
		panic(err)
	}

	// Lets get an object of type *three through the graph!
	var t *three
	err = c.Resolve(&t)
	if err != nil {
		panic(err)
	}

	// Print a detailed description of what's inside object t
	fmt.Printf("%#v\n", t)

	// Output:
	//
	// new *one is created
	// new *two is created
	// new *three is created
	// &main.three{one:(*main.one)(0x117b178), two:(*main.two)(0xc42000c040)}
}
