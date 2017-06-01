package dig

// In is an embeddable object that signals to dig that the struct
// should be treated differently. Instead of itself becoming an object
// in the graph, memebers of the struct are inserted into the graph.
//
// SturctTags on those memebers control their behavior. For example,
//
// 		type Input struct {
//			dig.In
//
//      S *Something
//			T *Thingy `optional:"true"`
//    }
//
// TODO better usage docs
type In struct{}

var _ digInObject = In{}

// Param is the only instance of parameterObject.
func (In) digInObject() {}

// Users embed the In struct to opt a struct in as a parameter object.
// This provides us an easy way to check if something embeds dig.In
// without iterating through all its fields.
type digInObject interface {
	digInObject()
}
