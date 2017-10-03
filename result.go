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

type (
	result interface {
		Produces() map[key]struct{}
	}

	resultList struct {
		ctype    reflect.Type
		produces map[key]struct{}

		Results []result
	}

	resultError struct{}

	resultSingle struct {
		Name string
		Type reflect.Type
	}

	resultObject struct {
		produces map[key]struct{}

		Type   reflect.Type
		Fields []resultObjectField
	}

	resultObjectField struct {
		Name   string
		Index  int
		Result result
	}
)

var (
	_ result = resultSingle{}
	_ result = resultError{}
	_ result = resultObject{}
	_ result = resultList{}
)

func newResultList(ctype reflect.Type) (resultList, error) {
	rl := resultList{
		ctype:    ctype,
		Results:  make([]result, ctype.NumOut()),
		produces: make(map[key]struct{}),
	}

	for i := 0; i < ctype.NumOut(); i++ {
		r, err := newResult(ctype.Out(i))
		if err != nil {
			return rl, errWrapf(err, "bad result %d", i+1)
		}
		rl.Results[i] = r

		for k := range r.Produces() {
			if _, ok := rl.produces[k]; ok {
				return rl, fmt.Errorf("returns multiple %v", k)
			}
			rl.produces[k] = struct{}{}
		}
	}

	if len(rl.produces) == 0 {
		return rl, fmt.Errorf("%v must provide at least one non-error type", ctype)
	}

	return rl, nil
}

func (rl resultList) Produces() map[key]struct{} { return rl.produces }

func newResult(t reflect.Type) (result, error) {
	switch {
	case isError(t):
		return resultError{}, nil
	case IsOut(t):
		return newResultObject(t)
	case embedsType(t, _outPtrType):
		return nil, fmt.Errorf(
			"%v embeds *dig.Out which is not supported, embed dig.Out value instead", t)
	case t.Kind() == reflect.Ptr && IsOut(t.Elem()):
		return nil, fmt.Errorf("%v is a pointer to dig.Out, use value type instead", t)
		// Make sure we're not producing dig.In's either.
	case IsIn(t) || (t.Kind() == reflect.Ptr && IsIn(t.Elem())) || embedsType(t, _inPtrType):
		return nil, fmt.Errorf("cannot provide parameter objects: %v embeds a dig.In", t)
	default:
		return resultSingle{Type: t}, nil
	}
}

func (rs resultSingle) Produces() map[key]struct{} {
	return map[key]struct{}{
		{name: rs.Name, t: rs.Type}: {},
	}
}

// resultError doesn't produce anything
func (resultError) Produces() map[key]struct{} { return nil }

func newResultObject(t reflect.Type) (resultObject, error) {
	ro := resultObject{Type: t, produces: make(map[key]struct{})}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _outType {
			// Skip over the dig.Out embed.
			continue
		}

		if f.PkgPath != "" {
			return ro, fmt.Errorf(
				"private fields not allowed in dig.Out, did you mean to export %q (%v) from %v?",
				f.Name, f.Type, t)
		}

		if isError(f.Type) {
			return ro, fmt.Errorf(
				"cannot provide errors from dig.Out: field %q (%v) of %v is an error field",
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

		for k := range r.Produces() {
			if _, ok := ro.produces[k]; ok {
				return ro, fmt.Errorf("returns multiple %v", k)
			}
			ro.produces[k] = struct{}{}
		}

		ro.Fields = append(ro.Fields, resultObjectField{
			Name:   f.Name,
			Index:  i,
			Result: r,
		})
	}
	return ro, nil
}

func (ro resultObject) Produces() map[key]struct{} { return ro.produces }
