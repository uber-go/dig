package dig

import "reflect"

var (
	_noValue          reflect.Value
	_errType          = reflect.TypeOf((*error)(nil)).Elem()
	_inInterfaceType  = reflect.TypeOf((*digInObject)(nil)).Elem()
	_outInterfaceType = reflect.TypeOf((*digOutObject)(nil)).Elem()
	_inType           = reflect.TypeOf(In{})
)

// In is an embeddable object that signals to dig that the struct
// should be treated differently. Instead of itself becoming an object
// in the graph, memebers of the struct are inserted into the graph.
//
// Tags on those memebers control their behavior. For example,
//
//    type Input struct {
//      dig.In
//
//      S *Something
//      T *Thingy `optional:"true"`
//    }
//
type In struct{}

// Out is an embeddable type that signals to dig that the returned
// struct should be treated differently. Instead of the struct itself
// becoming part of the container, all members of the struct will.
type Out struct{}

// TODO: better usage docs
// Try to add some symmetry for In-Out docs as well.

// In is the only instance that implements the digInObject interface.
func (In) digInObject() {}

// Out is the only instance that implements the digOutObject interface
func (Out) digOutObject() {}

// Users embed the In struct to opt a struct in as a parameter object.
// This provides us an easy way to check if something embeds dig.In
// without iterating through all its fields.
type digInObject interface {
	digInObject()
}

type digOutObject interface {
	digOutObject()
}

func isInObject(t reflect.Type) bool {
	return t.Implements(_inInterfaceType) && t.Kind() == reflect.Struct
}

func isOutObject(t reflect.Type) bool {
	return t.Implements(_outInterfaceType) && t.Kind() == reflect.Struct
}

// Validate interfaces are satisfied
var (
	_ digInObject  = In{}
	_ digOutObject = Out{}
)
