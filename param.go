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

// The param interface represents a dependency for a constructor.
//
// The following implementations exist:
//  paramList     All arguments of the constructor.
//  paramSingle   An explicitly requested type.
//  paramObject   dig.In struct where each field in the struct can be another
//                param.
type param interface {
	// Comprehensive list of dependencies this parameter represents.
	Dependencies() []edge
}

var (
	_ param = paramSingle{}
	_ param = paramObject{}
	_ param = paramList{}
)

// newParam builds a param from the given type. If the provided type is a
// dig.In struct, an paramObject will be returned.
func newParam(t reflect.Type) (param, error) {
	if IsIn(t) {
		return newParamObject(t)
	}
	return paramSingle{Type: t}, nil
}

// paramList holds all arguments of the constructor as params.
type paramList struct {
	ctype reflect.Type // type of the constructor

	Params []param
}

// newParamList builds a paramList from the provided constructor type.
//
// Variadic arguments of a constructor are ignored and not included as
// dependencies.
func newParamList(ctype reflect.Type) (paramList, error) {
	numArgs := ctype.NumIn()
	if ctype.IsVariadic() {
		// NOTE: If the function is variadic, we skip the last argument
		// because we're not filling variadic arguments yet. See #120.
		numArgs--
	}

	pl := paramList{
		ctype:  ctype,
		Params: make([]param, 0, numArgs),
	}

	for i := 0; i < numArgs; i++ {
		p, err := newParam(ctype.In(i))
		if err != nil {
			return pl, errWrapf(err, "bad argument %d", i+1)
		}
		pl.Params = append(pl.Params, p)
	}
	return pl, nil
}

func (pl paramList) Dependencies() []edge {
	var deps []edge
	for _, p := range pl.Params {
		deps = append(deps, p.Dependencies()...)
	}
	return deps
}

// paramSingle is an explicitly requested type, optionally with a name.
//
// This object must be present in the graph as-is unless it's specified as
// optional.
type paramSingle struct {
	Name     string
	Optional bool
	Type     reflect.Type
}

func (ps paramSingle) Dependencies() []edge {
	return []edge{
		{key: key{t: ps.Type, name: ps.Name}, optional: ps.Optional},
	}
}

// paramObjectField is a single field of a dig.In struct.
type paramObjectField struct {
	// Name of the field in the struct.
	//
	// To clarify, this is the name of the *struct field*, not the name of
	// the dig value requested by this field.
	Name string

	// Index of this field in the target struct.
	//
	// We need to track this separately because not all fields of the
	// struct map to params.
	Index int

	// The dependency requested by this field.
	Param param
}

// paramObject is a dig.In struct where each field is another param.
//
// This object is not expected in the graph as-is.
type paramObject struct {
	Type   reflect.Type
	Fields []paramObjectField

	deps []edge
}

// newParamObject builds an paramObject from the provided type. The type MUST
// be a dig.In struct.
func newParamObject(t reflect.Type) (paramObject, error) {
	po := paramObject{Type: t}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _inType {
			// Skip over the dig.In embed.
			continue
		}

		if f.PkgPath != "" {
			return po, fmt.Errorf(
				"private fields not allowed in dig.In, did you mean to export %q (%v) from %v?",
				f.Name, f.Type, t)
		}

		p, err := newParam(f.Type)
		if err != nil {
			return po, errWrapf(err, "bad field %q of %v", f.Name, t)
		}

		name := f.Tag.Get(_nameTag)
		optional, err := isFieldOptional(t, f)
		if err != nil {
			return po, err
		}

		if sp, ok := p.(paramSingle); ok {
			// Field tags apply only if the field is "simple"
			sp.Name = name
			sp.Optional = optional
			p = sp
		}

		po.Fields = append(po.Fields, paramObjectField{
			Name:  f.Name,
			Index: i,
			Param: p,
		})
		po.deps = append(po.deps, p.Dependencies()...)
	}
	return po, nil
}

func (po paramObject) Dependencies() []edge { return po.deps }
