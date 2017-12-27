// Copyright (c) 2018 Uber Technologies, Inc.
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

	"go.uber.org/dig/internal"
)

// The result interface represents a result produced by a constructor.
//
// The following implementations exist:
//   resultList    All values returned by the constructor.
//   resultSingle  A single value produced by a constructor.
//   resultObject  dig.Out struct where each field in the struct can be
//                 another result.
//   resultGrouped A value produced by a constructor that is part of a value
//                 group.
type result interface {
	// Extracts the values for this result from the provided value and
	// stores them into the provided containerWriter.
	//
	// This MAY panic if the result does not consume a single value.
	Extract(containerWriter, reflect.Value)

	// Enumerates the Keys produced by this result.
	Produces() []internal.Key
}

var (
	_ result = resultSingle{}
	_ result = resultObject{}
	_ result = resultList{}
	_ result = resultGrouped{}
)

type resultOptions struct {
	// If set, this is the name of the associated result value.
	//
	// For Result Objects, name:".." tags on fields override this.
	Name string
}

// newResult builds a result from the given type.
func newResult(t reflect.Type, opts resultOptions) (result, error) {
	switch {
	case IsIn(t) || (t.Kind() == reflect.Ptr && IsIn(t.Elem())) || embedsType(t, _inPtrType):
		return nil, fmt.Errorf("cannot provide parameter objects: %v embeds a dig.In", t)
	case isError(t):
		return nil, fmt.Errorf("cannot return an error here, return it from the constructor instead")
	case IsOut(t):
		return newResultObject(t, opts)
	case embedsType(t, _outPtrType):
		return nil, fmt.Errorf(
			"cannot build a result object by embedding *dig.Out, embed dig.Out instead: "+
				"%v embeds *dig.Out", t)
	case t.Kind() == reflect.Ptr && IsOut(t.Elem()):
		return nil, fmt.Errorf(
			"cannot return a pointer to a result object, use a value instead: "+
				"%v is a pointer to a struct that embeds dig.Out", t)
	default:
		return newResultSingle(t, opts), nil
	}
}

// resultList holds all values returned by the constructor as results.
type resultList struct {
	ctype reflect.Type

	Results []result
	keys    *internal.KeySet

	// For each item at index i returned by the constructor, resultIndexes[i]
	// is the index in .Results for the corresponding result object.
	// resultIndexes[i] is -1 for errors returned by constructors.
	resultIndexes []int
}

func newResultList(ctype reflect.Type, opts resultOptions) (resultList, error) {
	rl := resultList{
		ctype:         ctype,
		Results:       make([]result, 0, ctype.NumOut()),
		keys:          internal.NewKeySet(),
		resultIndexes: make([]int, ctype.NumOut()),
	}

	resultIdx := 0
	for i := 0; i < ctype.NumOut(); i++ {
		t := ctype.Out(i)
		if isError(t) {
			rl.resultIndexes[i] = -1
			continue
		}

		r, err := newResult(t, opts)
		if err != nil {
			return rl, errWrapf(err, "bad result %d", i+1)
		}

		src := fmt.Sprintf("result %d", i)
		for _, k := range r.Produces() {
			if err := rl.keys.Provide(src, k); err != nil {
				return rl, errWrapf(err, "cannot provide %v from %v", k, src)
			}
		}

		rl.Results = append(rl.Results, r)
		rl.resultIndexes[i] = resultIdx
		resultIdx++
	}

	return rl, nil
}

func (resultList) Extract(containerWriter, reflect.Value) {
	panic("It looks like you have found a bug in dig. " +
		"Please file an issue at https://github.com/uber-go/dig/issues/ " +
		"and provide the following message: " +
		"resultList.Extract() must never be called")
}

func (rl resultList) Produces() []internal.Key { return rl.keys.Items() }

func (rl resultList) ExtractList(cw containerWriter, values []reflect.Value) error {
	for i, v := range values {
		if resultIdx := rl.resultIndexes[i]; resultIdx >= 0 {
			rl.Results[resultIdx].Extract(cw, v)
			continue
		}

		if err, _ := v.Interface().(error); err != nil {
			return err
		}
	}

	return nil
}

// resultSingle is an explicit value produced by a constructor, optionally
// with a name.
//
// This object will be added to the graph as-is.
type resultSingle struct {
	Name string
	Type reflect.Type
}

func newResultSingle(t reflect.Type, opts resultOptions) resultSingle {
	return resultSingle{Name: opts.Name, Type: t}
}

func (rs resultSingle) Produces() []internal.Key {
	return []internal.Key{
		internal.ValueKey{Name: rs.Name, Type: rs.Type},
	}
}

func (rs resultSingle) Extract(cw containerWriter, v reflect.Value) {
	cw.setValue(internal.ValueKey{Name: rs.Name, Type: rs.Type}, v)
}

// resultObject is a dig.Out struct where each field is another result.
//
// This object is not added to the graph. Its fields are interpreted as
// results and added to the graph if needed.
type resultObject struct {
	Type   reflect.Type
	Fields []resultObjectField
	keys   *internal.KeySet
}

func newResultObject(t reflect.Type, opts resultOptions) (resultObject, error) {
	ro := resultObject{
		Type: t,
		keys: internal.NewKeySet(),
	}

	if len(opts.Name) > 0 {
		return ro, fmt.Errorf(
			"cannot specify a name for result objects: %v embeds dig.Out", t)
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _outType {
			// Skip over the dig.Out embed.
			continue
		}

		rof, err := newResultObjectField(i, f, opts)
		if err != nil {
			return ro, errWrapf(err, "bad field %q of %v", f.Name, t)
		}

		src := fmt.Sprintf("field %v", f.Name)
		for _, k := range rof.Result.Produces() {
			if err := ro.keys.Provide(src, k); err != nil {
				return ro, errWrapf(err, "cannot provide %v from %v", k, src)
			}
		}

		ro.Fields = append(ro.Fields, rof)
	}
	return ro, nil
}

func (ro resultObject) Extract(cw containerWriter, v reflect.Value) {
	for _, f := range ro.Fields {
		f.Result.Extract(cw, v.Field(f.FieldIndex))
	}
}

func (ro resultObject) Produces() []internal.Key { return ro.keys.Items() }

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

// newResultObjectField(i, f, opts) builds a resultObjectField from the field
// f at index i.
func newResultObjectField(idx int, f reflect.StructField, opts resultOptions) (resultObjectField, error) {
	rof := resultObjectField{
		FieldName:  f.Name,
		FieldIndex: idx,
	}

	var r result
	switch {
	case f.PkgPath != "":
		return rof, fmt.Errorf(
			"unexported fields not allowed in dig.Out, did you mean to export %q (%v)?", f.Name, f.Type)

	case f.Tag.Get(_groupTag) != "":
		var err error
		r, err = newResultGrouped(f)
		if err != nil {
			return rof, err
		}

	default:
		var err error
		if name := f.Tag.Get(_nameTag); len(name) > 0 {
			// can modify in-place because options are passed-by-value.
			opts.Name = name
		}
		r, err = newResult(f.Type, opts)
		if err != nil {
			return rof, err
		}
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

func (rt resultGrouped) Extract(cw containerWriter, v reflect.Value) {
	cw.submitGroupedValue(internal.GroupKey{Name: rt.Group, Type: rt.Type}, v)
}

func (rt resultGrouped) Produces() []internal.Key {
	return []internal.Key{
		internal.GroupKey{Name: rt.Group, Type: rt.Type},
	}
}
