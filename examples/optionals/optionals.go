package main

import (
	"fmt"

	"go.uber.org/dig"
)

type Type1 struct{}

type Type2 struct{}

func main() {
	c := dig.New()
	c.Optionals(&Type1{})
	c.Provide(&Type2{})
	if err := c.Invoke(func(t1 *Type1, t2 *Type2) {
		fmt.Println("t1", t1)
		fmt.Println("t1", t2)
	}); err != nil {
		panic(err)
	}
	fmt.Println(c.String())
}
