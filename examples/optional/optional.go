package main

import (
	"fmt"

	"go.uber.org/dig"
)

// type1 is a "required" dig type by default
type type1 struct{}

// type2 embeds a dig.Optional object to signal it's ok to ignore in params
type type2 struct {
	dig.Optional
}

func main() {
	c := dig.New()
	c.Provide(&type1{})
	if err := c.Invoke(func(t1 *type1, t2 *type2) {
		fmt.Println("t1", t1) // will get actual instance of type1
		fmt.Println("t2", t2) // note nil and dig doesn't error out, just provides zero value
	}); err != nil {
		panic(err)
	}
}
