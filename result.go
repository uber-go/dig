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
	"fmt"
	"reflect"
)

// The result interface represents a result produced by a constructor.
//
// The following implementations exist:
//   resultList    All values returned by the constructor.
//   resultSingle  An explicitly requested type.
//   resultError   An error returned by the constructor.
//   resultObject  dig.Out struct where each field in the struct can be
//                 another result.
type result interface {
	// Extracts the values for this result from the provided value and
	// stores them in the container.
	//
	// This MAY panic if the result does not consume a single value.
	Extract(*Container, reflect.Value) error
}

var (
	_ result = resultSingle{}
	_ result = resultError{}
	_ result = resultObject{}
	_ result = resultList{}
)

// newResult builds a result from the given type.
func newResult(t reflect.Type) (result, error) {
	switch {
	case IsIn(t) || (t.Kind() == reflect.Ptr && IsIn(t.Elem())) || embedsType(t, _inPtrType):
		return nil, fmt.Errorf("cannot provide parameter objects: %v embeds a dig.In", t)
	case isError(t):
		return resultError{}, nil
	case IsOut(t):
		return newResultObject(t)
	case embedsType(t, _outPtrType):
		return nil, fmt.Errorf(
			"cannot build a result object by embedding *dig.Out, embed dig.Out instead: "+
				"%v embeds *dig.Out", t)
	case t.Kind() == reflect.Ptr && IsOut(t.Elem()):
		return nil, fmt.Errorf(
			"cannot return a pointer to a result object, use a value instead: "+
				"%v is a pointer to a struct that embeds dig.Out", t)
	default:
		return resultSingle{Type: t}, nil
	}
}

// resultVisitor visits every result in a result tree, allowing tracking state
// at each level.
type resultVisitor interface {
	// Visit is called on the result being visited.
	//
	// If Visit returns a non-nil resultVisitor, that resultVisitor visits all
	// the child results of this result.
	Visit(result) resultVisitor

	// AnnotateWithField is called on each field of a resultObject after
	// visiting it but before walking its descendants.
	//
	// The same resultVisitor is used for all fields: the one returned upon
	// visiting the resultObject.
	//
	// For each visited field, if AnnotateWithField returns a non-nil
	// resultVisitor, it will be used to walk the result of that field.
	AnnotateWithField(resultObjectField) resultVisitor

	// AnnotateWithPosition is called with the index of each result of a
	// resultList after vising it but before walking its descendants.
	//
	// The same resultVisitor is used for all results: the one returned upon
	// visiting the resultList.
	//
	// For each position, if AnnotateWithPosition returns a non-nil
	// resultVisitor, it will be used to walk the result at that index.
	AnnotateWithPosition(idx int) resultVisitor
}

// walkResult walks the result tree for the given result with the provided
// visitor.
//
// resultVisitor.Visit will be called on the provided result and if a non-nil
// resultVisitor is received, it will be used to walk its descendants. If a
// resultObject or resultList was visited, AnnotateWithField and
// AnnotateWithPosition respectively will be called before visiting the
// descendants of that resultObject/resultList.
//
// This is very similar to how go/ast.Walk works.
func walkResult(r result, v resultVisitor) {
	v = v.Visit(r)
	if v == nil {
		return
	}

	switch res := r.(type) {
	case resultSingle, resultError:
		// No sub-results
	case resultObject:
		w := v
		for _, f := range res.Fields {
			if v := w.AnnotateWithField(f); v != nil {
				walkResult(f.Result, v)
			}
		}
	case resultList:
		w := v
		for i, r := range res.Results {
			if v := w.AnnotateWithPosition(i); v != nil {
				walkResult(r, v)
			}
		}
	default:
		panic(fmt.Sprintf(
			"It looks like you have found a bug in dig. "+
				"Please file an issue at https://github.com/uber-go/dig/issues/ "+
				"and provide the following message: "+
				"received unknown result type %T", res))
	}
}

// resultList holds all values returned by the constructor as results.
type resultList struct {
	ctype reflect.Type

	Results []result
}

func newResultList(ctype reflect.Type) (resultList, error) {
	rl := resultList{
		ctype:   ctype,
		Results: make([]result, ctype.NumOut()),
	}

	for i := 0; i < ctype.NumOut(); i++ {
		r, err := newResult(ctype.Out(i))
		if err != nil {
			return rl, errWrapf(err, "bad result %d", i+1)
		}
		rl.Results[i] = r
	}

	return rl, nil
}

func (resultList) Extract(*Container, reflect.Value) error {
	panic("It looks like you have found a bug in dig. " +
		"Please file an issue at https://github.com/uber-go/dig/issues/ " +
		"and provide the following message: " +
		"resultList.Extract() must never be called")
}

func (rl resultList) ExtractList(c *Container, values []reflect.Value) error {
	for i, r := range rl.Results {
		if err := r.Extract(c, values[i]); err != nil {
			return err
		}
	}
	return nil
}

// resultError is an error returned by a constructor.
type resultError struct{}

func (resultError) Extract(_ *Container, v reflect.Value) error {
	err, _ := v.Interface().(error)
	return err
}

// resultSingle is an explicit value produced by a constructor, optionally
// with a name.
//
// This object will be added to the graph as-is.
type resultSingle struct {
	Name string
	Type reflect.Type
}

func (rs resultSingle) Extract(c *Container, v reflect.Value) error {
	c.cache[key{name: rs.Name, t: rs.Type}] = v
	return nil
}

// resultObjectField is a single field inside a dig.Out struct.
type resultObjectField struct {
	// Name of the field in the struct.
	FieldName string

	// Index of the field in the struct.
	//
	// We need to track this separately because not all fields of the struct
	// map to results.
	FieldIndex int

	// Result produced by this field.
	Result result
}

// resultObject is a dig.Out struct where each field is another result.
//
// This object is not added to the graph. Its fields are interpreted as
// results and added to the graph if needed.
type resultObject struct {
	Type   reflect.Type
	Fields []resultObjectField
}

func newResultObject(t reflect.Type) (resultObject, error) {
	ro := resultObject{Type: t}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _outType {
			// Skip over the dig.Out embed.
			continue
		}

		if f.PkgPath != "" {
			return ro, fmt.Errorf(
				"unexported fields not allowed in dig.Out, did you mean to export %q (%v) from %v?",
				f.Name, f.Type, t)
		}

		if isError(f.Type) {
			return ro, fmt.Errorf(
				"cannot return errors from dig.Out, return it from the constructor instead: "+
					"field %q (%v) of %v is an error field",
				f.Name, f.Type, t)
		}

		r, err := newResult(f.Type)
		if err != nil {
			return ro, errWrapf(err, "bad field %q of %v", f.Name, t)
		}

		name := f.Tag.Get(_nameTag)
		if rs, ok := r.(resultSingle); ok {
			// field tags apply only if the result is "simple"
			rs.Name = name
			r = rs
		}

		ro.Fields = append(ro.Fields, resultObjectField{
			FieldName:  f.Name,
			FieldIndex: i,
			Result:     r,
		})
	}
	return ro, nil
}

func (ro resultObject) Extract(c *Container, v reflect.Value) error {
	for _, f := range ro.Fields {
		if err := f.Result.Extract(c, v.Field(f.FieldIndex)); err != nil {
			// In reality, this will never fail because none of the fields of
			// a resultObject can be resultError.
			panic(fmt.Sprintf(
				"It looks like you have found a bug in dig. "+
					"Please file an issue at https://github.com/uber-go/dig/issues/ "+
					"and provide the following message: "+
					"result.Extract() encountered an error: %v", err))
		}
	}
	return nil
}
