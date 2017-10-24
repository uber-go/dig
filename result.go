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
	"errors"
	"fmt"
	"reflect"
)

// The result interface represents a result produced by a constructor.
//
// The following implementations exist:
//   resultList    All values returned by the constructor.
//   resultSingle  A single value produced by a constructor.
//   resultError   An error returned by the constructor.
//   resultObject  dig.Out struct where each field in the struct can be
//                 another result.
//   resultGrouped A value produced by a constructor that is part of a value
//                 group.
type result interface {
	// Extracts the values for this result from the provided value and
	// stores them into the provided resultReceiver.
	//
	// This MAY panic if the result does not consume a single value.
	Extract(resultReceiver, reflect.Value)
}

// resultReceiver receives the values or failures produced by constructors.
type resultReceiver interface {
	// Notifies the receiver that the constructor failed with the given error.
	SubmitError(error)

	// Submits a new value to the receiver.
	SubmitValue(name string, t reflect.Type, v reflect.Value)

	// Submits a new value to a value group.
	SubmitGroupValue(group string, t reflect.Type, v reflect.Value)
}

var (
	_ result = resultSingle{}
	_ result = resultError{}
	_ result = resultObject{}
	_ result = resultList{}
	_ result = resultGrouped{}
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
	case resultSingle, resultError, resultGrouped:
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

func (resultList) Extract(resultReceiver, reflect.Value) {
	panic("It looks like you have found a bug in dig. " +
		"Please file an issue at https://github.com/uber-go/dig/issues/ " +
		"and provide the following message: " +
		"resultList.Extract() must never be called")
}

func (rl resultList) ExtractList(rr resultReceiver, values []reflect.Value) {
	for i, r := range rl.Results {
		r.Extract(rr, values[i])
	}
}

// resultError is an error returned by a constructor.
type resultError struct{}

func (resultError) Extract(rr resultReceiver, v reflect.Value) {
	if err, _ := v.Interface().(error); err != nil {
		rr.SubmitError(err)
	}
}

// resultSingle is an explicit value produced by a constructor, optionally
// with a name.
//
// This object will be added to the graph as-is.
type resultSingle struct {
	Name string
	Type reflect.Type
}

func (rs resultSingle) Extract(rr resultReceiver, v reflect.Value) {
	rr.SubmitValue(rs.Name, rs.Type, v)
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

		rof, err := newResultObjectField(i, f)
		if err != nil {
			return ro, errWrapf(err, "bad field %q of %v", f.Name, t)
		}

		ro.Fields = append(ro.Fields, rof)
	}
	return ro, nil
}

func (ro resultObject) Extract(rr resultReceiver, v reflect.Value) {
	for _, f := range ro.Fields {
		f.Result.Extract(rr, v.Field(f.FieldIndex))
	}
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

// newResultObjectField(i, f) builds a resultObjectField from the field f at
// index i.
func newResultObjectField(idx int, f reflect.StructField) (resultObjectField, error) {
	rof := resultObjectField{
		FieldName:  f.Name,
		FieldIndex: idx,
	}

	var r result
	switch {
	case f.PkgPath != "":
		return rof, fmt.Errorf(
			"unexported fields not allowed in dig.Out, did you mean to export %q (%v)?", f.Name, f.Type)

	case isError(f.Type):
		return rof, fmt.Errorf(
			"cannot return errors from dig.Out, return it from the constructor instead: "+
				"field %q (%v) is an error field",
			f.Name, f.Type)

	case f.Tag.Get(_groupTag) != "":
		var err error
		r, err = newResultGrouped(f)
		if err != nil {
			return rof, err
		}

	default:
		var err error
		r, err = newResult(f.Type)
		if err != nil {
			return rof, err
		}
	}

	if rs, ok := r.(resultSingle); ok {
		// Field tags apply only if the result is "simple"
		rs.Name = f.Tag.Get(_nameTag)
		r = rs
	}

	rof.Result = r
	return rof, nil
}

// resultGrouped is a value produced by a constructor that is part of a result
// group.
//
// These will be produced as fields of a dig.Out struct.
type resultGrouped struct {
	// Name of the group as specified in the `group:".."` tag.
	Group string

	// Type of value produced.
	Type reflect.Type
}

// newResultGrouped(f) builds a new resultGrouped from the provided field.
func newResultGrouped(f reflect.StructField) (resultGrouped, error) {
	rg := resultGrouped{Group: f.Tag.Get(_groupTag), Type: f.Type}

	name := f.Tag.Get(_nameTag)
	optional, _ := isFieldOptional(f)
	switch {
	case name != "":
		return rg, fmt.Errorf(
			"cannot use named values with value groups: name:%q provided with group:%q", name, rg.Group)
	case optional:
		return rg, errors.New("value groups cannot be optional")
	}

	return rg, nil
}

func (rt resultGrouped) Extract(rr resultReceiver, v reflect.Value) {
	rr.SubmitGroupValue(rt.Group, rt.Type, v)
}
