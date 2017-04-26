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
	err := c.ProvideAll(newOne, newTwo, newThree)
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
