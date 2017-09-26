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
	fmt.Stringer

	// Builds this dependency and any of its from the provided Container.
	//
	// This MAY panic if the param does not produce a single value.
	Build(*Container) (reflect.Value, error)
}

var (
	_ param = paramSingle{}
	_ param = paramObject{}
	_ param = paramList{}
)

// newParam builds a param from the given type. If the provided type is a
// dig.In struct, an paramObject will be returned.
func newParam(t reflect.Type) (param, error) {
	switch {
	case IsIn(t):
		return newParamObject(t)
	case embedsType(t, _inPtrType):
		return nil, fmt.Errorf(
			"%v embeds *dig.In which is not supported, embed dig.In value instead", t)
	case t.Kind() == reflect.Ptr && IsIn(t.Elem()):
		return nil, fmt.Errorf(
			"dependency %v is a pointer to dig.In, use value type instead", t)
	default:
		return paramSingle{Type: t}, nil
	}
}

// Calls the provided function on all paramSingles in the given param tree.
func forEachParamSingle(param param, f func(paramSingle)) {
	switch p := param.(type) {
	case paramList:
		for _, arg := range p.Params {
			forEachParamSingle(arg, f)
		}
	case paramSingle:
		f(p)
	case paramObject:
		for _, field := range p.Fields {
			forEachParamSingle(field.Param, f)
		}
	default:
		panic(fmt.Sprintf(
			"It looks like you have found a bug in dig. "+
				"Please file an issue at https://github.com/uber-go/dig/issues/ "+
				"and provide the following message: "+
				"received unknown param type %T", param))
	}
}

// paramList holds all arguments of the constructor as params.
//
// NOTE: Build() MUST NOT be called on paramList. Instead, BuildParams
// must be called.
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

func (pl paramList) Build(*Container) (reflect.Value, error) {
	panic("paramList.Build() must never be called")
}

// BuildParams returns an ordered list of values which may be passed directly
// to the underlying constructor.
func (pl paramList) BuildParams(c *Container) ([]reflect.Value, error) {
	args := make([]reflect.Value, len(pl.Params))
	for i, p := range pl.Params {
		var err error
		args[i], err = p.Build(c)
		if err != nil {
			return nil, errWrapf(err, "could not build argument %d for constructor %v", i, pl.ctype)
		}
	}
	return args, nil
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

func (ps paramSingle) Build(c *Container) (reflect.Value, error) {
	k := key{name: ps.Name, t: ps.Type}
	if v, ok := c.cache[k]; ok {
		return v, nil
	}

	n, ok := c.nodes[k]
	if !ok {
		// Unlike in the fallback case below, if a user makes an error requesting
		// a mixed type for an optional parameter, a good error message "did you mean X?"
		// will not be used and dig will return zero value.
		if ps.Optional {
			return reflect.Zero(ps.Type), nil
		}

		// If the type being asked for is the pointer that is not found,
		// check if the graph contains the value type element - perhaps the user
		// accidentally included a splat and vice versa.
		var typo reflect.Type
		if ps.Type.Kind() == reflect.Ptr {
			typo = ps.Type.Elem()
		} else {
			typo = reflect.PtrTo(ps.Type)
		}

		tk := key{t: typo, name: ps.Name}
		if _, ok := c.nodes[tk]; ok {
			return _noValue, fmt.Errorf(
				"type %v is not in the container, did you mean to use %v?", k, tk)
		}

		return _noValue, fmt.Errorf("type %v isn't in the container", k)
	}

	if err := shallowCheckDependencies(c, n.Params); err != nil {
		if ps.Optional {
			return reflect.Zero(ps.Type), nil
		}
		return _noValue, errWrapf(err, "missing dependencies for %v", k)
	}

	args, err := n.Params.BuildParams(c)
	if err != nil {
		return _noValue, errWrapf(err, "couldn't get arguments for constructor %v", n.ctype)
	}

	constructed := reflect.ValueOf(n.ctor).Call(args)

	// Provide-time validation ensures that all constructors return at least
	// one value.
	if errV := constructed[len(constructed)-1]; isError(errV.Type()) {
		if err, _ := errV.Interface().(error); err != nil {
			return _noValue, errWrapf(err, "constructor %v for type %v failed", n.ctype, ps.Type)
		}
	}

	for _, con := range constructed {
		// Set the resolved object into the cache.
		// This might look confusing at first like we're ignoring named types,
		// but `con` in this case will be the dig.Out object, which will
		// cause a recursion into the .set for each of it's memebers.
		c.set(key{t: con.Type()}, con)
	}
	return c.cache[k], nil
}

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

// paramObject is a dig.In struct where each field is another param.
//
// This object is not expected in the graph as-is.
type paramObject struct {
	Type   reflect.Type
	Fields []paramObjectField
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
				"unexported fields not allowed in dig.In, did you mean to export %q (%v) from %v?",
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

		if ps, ok := p.(paramSingle); ok {
			// Field tags apply only if the field is "simple"
			ps.Name = name
			ps.Optional = optional
			p = ps
		}

		po.Fields = append(po.Fields, paramObjectField{
			FieldName:  f.Name,
			FieldIndex: i,
			Param:      p,
		})
	}
	return po, nil
}

func (po paramObject) Build(c *Container) (reflect.Value, error) {
	dest := reflect.New(po.Type).Elem()
	for _, f := range po.Fields {
		v, err := f.Param.Build(c)
		if err != nil {
			return v, errWrapf(err, "could not get field %v of %v", f.FieldName, po.Type)
		}
		dest.Field(f.FieldIndex).Set(v)
	}
	return dest, nil
}
