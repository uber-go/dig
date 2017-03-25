// Package dig is the dig - A dependency injection framework for Go.
//
// package dig provides an opinionated way of resolving object dependencies.
// There are two sides of dig:
// Provide and Resolve.
//
// Status
//
// BETA. Expect potential API changes.
//
// Provide
//
// Provide adds an object, or a constructor of an object to the container.
//
// There are two ways to Provide an object:
//
// • Provide a pointer to an existing object
//
// • Provide a "constructor function" that returns one pointer (or interface)
//
// Provide an object
//
// Registering an object means it has no dependencies, and will be used as a
// **shared** singleton instance for all resolutions within the container.
//
//   type Fake struct {
//       Name string
//   }
//
//   c := dig.New()
//   err := c.Provide(&Fake{Name: "I am an thing"})
//   require.NoError(t, err)
//
//   var f1 *Fake
//   err = c.Resolve(&f1)
//   require.NoError(t, err)
//
//   // f1 is ready to use here...
//
// Provide a constructor
//
// This is a more interesting and widely used scenario. Constructor is defined as a
// function that returns exactly one pointer (or interface) and takes 0-N number of
// arguments. Each one of the arguments is automatically registered as a
// **dependency** and must also be an interface or a pointer.
//
// The following example illustrates registering a constructor function for type
// *Object that requires *Dep to be present in the container.
//
//   c := dig.New()
//
//   type Dep struct{}
//   type Object struct{
//     Dep
//   }
//
//   func NewObject(d *Dep) *Object {
//     return &Object{Dep: d}
//   }
//
//   err := c.Provide(NewObject)
//
// Resolve
//
// Resolve retrieves objects from the container by building the object graph.
//
// There are future plans to do named retrievals to support multiple
// objects of the same type in the container.
//
//
//   c := dig.New()
//
//   var o *Object
//   err := c.Resolve(&o) // notice the pointer to a pointer as param type
//   if err == nil {
//       // o is ready to use
//   }
//
//   type Do interface{}
//   var d Do
//   err := c.Resolve(&d) // notice pointer to an interface
//   if err == nil {
//       // d is ready to use
//   }
//
//
package dig
