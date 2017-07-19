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

import (
	"container/list"
	"reflect"
)

var (
	_noValue reflect.Value
	_errType = reflect.TypeOf((*error)(nil)).Elem()
	_inType  = reflect.TypeOf((*In)(nil)).Elem()
	_outType = reflect.TypeOf((*Out)(nil)).Elem()
)

// Special interface embedded inside dig sentinel values (dig.In, dig.Out) to
// make their special nature obvious in the godocs. Otherwise they will appear
// as plain empty structs.
type digSentinel interface {
	digSentinel()
}

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
type In struct{ digSentinel }

// Out is an embeddable type that signals to dig that the returned
// struct should be treated differently. Instead of the struct itself
// becoming part of the container, all members of the struct will.
type Out struct{ digSentinel }

// TODO: better usage docs
// Try to add some symmetry for In-Out docs as well.

func isError(t reflect.Type) bool {
	return t.Implements(_errType)
}

// IsIn returns true if passed in type embeds dig.In either directly
// or through another struct field.
func IsIn(t reflect.Type) bool {
	return embedsType(t, _inType)
}

// IsOut returns true if passed in type embeds dig.Out either directly
// or through another struct field.
func IsOut(t reflect.Type) bool {
	return embedsType(t, _outType)
}

// Returns true if t embeds e or if any of the types embedded by t embed e.
func embedsType(t reflect.Type, e reflect.Type) bool {
	// We are going to do a breadth-first search of all embedded fields.
	types := list.New()
	types.PushBack(t)
	for types.Len() > 0 {
		t := types.Remove(types.Front()).(reflect.Type)

		if t == e {
			return true
		}
		if t.Kind() != reflect.Struct {
			continue
		}

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				types.PushBack(f.Type)
			}
		}
	}

	// If perf is an issue, we can cache known In objects and Out objects in a
	// map[reflect.Type]struct{}.
	return false
}
