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

// The param interface represents a dependency for a constructor.
//
// The following implementations exist:
//  paramList     All arguments of the constructor.
//  paramSingle   An explicitly requested type.
//  paramObject   dig.In struct where each field in the struct can be another
//                param.
//  paramGroupedSlice
//                A slice consuming a value group. This will receive all
//                values produced with a `group:".."` tag with the same name
//                as a slice.
type param interface {
	fmt.Stringer

	// Builds this dependency and any of its dependencies from the provided
	// Container.
	//
	// This MAY panic if the param does not produce a single value.
	Build(containerStore) (reflect.Value, error)

	// Enumerates all the dependencies consumed by this param directly.
	Consumes() []internal.Dependency
}

var (
	_ param = paramSingle{}
	_ param = paramObject{}
	_ param = paramList{}
	_ param = paramGroupedSlice{}
)

// newParam builds a param from the given type. If the provided type is a
// dig.In struct, an paramObject will be returned.
func newParam(t reflect.Type) (param, error) {
	switch {
	case IsOut(t) || (t.Kind() == reflect.Ptr && IsOut(t.Elem())) || embedsType(t, _outPtrType):
		return nil, fmt.Errorf("cannot depend on result objects: %v embeds a dig.Out", t)
	case IsIn(t):
		return newParamObject(t)
	case embedsType(t, _inPtrType):
		return nil, fmt.Errorf(
			"cannot build a parameter object by embedding *dig.In, embed dig.In instead: "+
				"%v embeds *dig.In", t)
	case t.Kind() == reflect.Ptr && IsIn(t.Elem()):
		return nil, fmt.Errorf(
			"cannot depend on a pointer to a parameter object, use a value instead: "+
				"%v is a pointer to a struct that embeds dig.In", t)
	default:
		return newParamSingle(t), nil
	}
}

// paramList holds all arguments of the constructor as params.
//
// NOTE: Build() MUST NOT be called on paramList. Instead, BuildList
// must be called.
type paramList struct {
	ctype reflect.Type // type of the constructor

	Params   []param
	consumes []internal.Dependency
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
		pl.consumes = append(pl.consumes, p.Consumes()...)
	}

	return pl, nil
}

func (pl paramList) Build(containerStore) (reflect.Value, error) {
	panic("It looks like you have found a bug in dig. " +
		"Please file an issue at https://github.com/uber-go/dig/issues/ " +
		"and provide the following message: " +
		"paramList.Build() must never be called")
}

// BuildList returns an ordered list of values which may be passed directly
// to the underlying constructor.
func (pl paramList) BuildList(c containerStore) ([]reflect.Value, error) {
	args := make([]reflect.Value, len(pl.Params))
	for i, p := range pl.Params {
		var err error
		args[i], err = p.Build(c)
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}

func (pl paramList) Consumes() []internal.Dependency { return pl.consumes }

// paramSingle is an explicitly requested type, optionally with a name.
//
// This object must be present in the graph as-is unless it's specified as
// optional.
type paramSingle struct {
	Name     string
	Optional bool
	Type     reflect.Type
}

func newParamSingle(t reflect.Type) paramSingle {
	return paramSingle{Type: t}
}

func (ps paramSingle) Build(c containerStore) (reflect.Value, error) {
	k := internal.ValueKey{Name: ps.Name, Type: ps.Type}
	if v, ok := c.getValue(k); ok {
		return v, nil
	}

	providers := c.getProviders(k)
	if len(providers) == 0 {
		if ps.Optional {
			return reflect.Zero(ps.Type), nil
		}
		return _noValue, newErrMissingType(c, k)
	}

	for _, n := range providers {
		err := n.Call(c)
		if err == nil {
			continue
		}

		// If we're missing dependencies but the parameter itself is optional,
		// we can just move on.
		if _, ok := err.(errMissingDependencies); ok && ps.Optional {
			return reflect.Zero(ps.Type), nil
		}

		return _noValue, errParamSingleFailed{Key: k, Reason: err}
	}

	// If we get here, it's impossible for the value to be absent from the
	// container.
	v, _ := c.getValue(k)
	return v, nil
}

func (ps paramSingle) Consumes() []internal.Dependency {
	return []internal.Dependency{
		{
			Key:      internal.ValueKey{Name: ps.Name, Type: ps.Type},
			Optional: ps.Optional,
		},
	}
}

// paramObject is a dig.In struct where each field is another param.
//
// This object is not expected in the graph as-is.
type paramObject struct {
	Type     reflect.Type
	Fields   []paramObjectField
	consumes []internal.Dependency
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

		pof, err := newParamObjectField(i, f)
		if err != nil {
			return po, errWrapf(err, "bad field %q of %v", f.Name, t)
		}

		po.Fields = append(po.Fields, pof)
		po.consumes = append(po.consumes, pof.Param.Consumes()...)
	}

	return po, nil
}

func (po paramObject) Build(c containerStore) (reflect.Value, error) {
	dest := reflect.New(po.Type).Elem()
	for _, f := range po.Fields {
		v, err := f.Build(c)
		if err != nil {
			return dest, err
		}
		dest.Field(f.FieldIndex).Set(v)
	}
	return dest, nil
}

func (po paramObject) Consumes() []internal.Dependency { return po.consumes }

// paramObjectField is a single field of a dig.In struct.
type paramObjectField struct {
	// Name of the field in the struct.
	FieldName string

	// Index of this field in the target struct.
	//
	// We need to track this separately because not all fields of the
	// struct map to params.
	FieldIndex int

	// The dependency requested by this field.
	Param param
}

func newParamObjectField(idx int, f reflect.StructField) (paramObjectField, error) {
	pof := paramObjectField{
		FieldName:  f.Name,
		FieldIndex: idx,
	}

	var p param
	switch {
	case f.PkgPath != "":
		return pof, fmt.Errorf(
			"unexported fields not allowed in dig.In, did you mean to export %q (%v)?",
			f.Name, f.Type)

	case f.Tag.Get(_groupTag) != "":
		var err error
		p, err = newParamGroupedSlice(f)
		if err != nil {
			return pof, err
		}

	default:
		var err error
		p, err = newParam(f.Type)
		if err != nil {
			return pof, err
		}
	}

	if ps, ok := p.(paramSingle); ok {
		ps.Name = f.Tag.Get(_nameTag)

		var err error
		ps.Optional, err = isFieldOptional(f)
		if err != nil {
			return pof, err
		}

		p = ps
	}

	pof.Param = p
	return pof, nil
}

func (pof paramObjectField) Build(c containerStore) (reflect.Value, error) {
	v, err := pof.Param.Build(c)
	if err != nil {
		return v, err
	}
	return v, nil
}

// paramGroupedSlice is a param which produces a slice of values with the same
// group name.
type paramGroupedSlice struct {
	// Name of the group as specified in the `group:".."` tag.
	Group string

	// Type of the slice.
	Type reflect.Type
}

// newParamGroupedSlice builds a paramGroupedSlice from the provided type with
// the given name.
//
// The type MUST be a slice type.
func newParamGroupedSlice(f reflect.StructField) (paramGroupedSlice, error) {
	pg := paramGroupedSlice{Group: f.Tag.Get(_groupTag), Type: f.Type}

	name := f.Tag.Get(_nameTag)
	optional, _ := isFieldOptional(f)
	switch {
	case f.Type.Kind() != reflect.Slice:
		return pg, fmt.Errorf("value groups may be consumed as slices only: "+
			"field %q (%v) is not a slice", f.Name, f.Type)
	case name != "":
		return pg, fmt.Errorf(
			"cannot use named values with value groups: name:%q requested with group:%q", name, pg.Group)

	case optional:
		return pg, errors.New("value groups cannot be optional")
	}

	return pg, nil
}

func (pt paramGroupedSlice) Build(c containerStore) (reflect.Value, error) {
	k := internal.GroupKey{Name: pt.Group, Type: pt.Type.Elem()}

	for _, n := range c.getProviders(k) {
		if err := n.Call(c); err != nil {
			return _noValue, errParamGroupFailed{
				Key:    k,
				Reason: err,
			}
		}
	}

	items := c.getValueGroup(k)

	result := reflect.MakeSlice(pt.Type, len(items), len(items))
	for i, v := range items {
		result.Index(i).Set(v)
	}
	return result, nil
}

func (pt paramGroupedSlice) Consumes() []internal.Dependency {
	return []internal.Dependency{
		{
			Key: internal.GroupKey{Name: pt.Group, Type: pt.Type.Elem()},
			// Value group dependencies are always optional.
			Optional: true,
		},
	}
}
