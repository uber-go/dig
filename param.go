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

	// Builds this dependency and any of its dependencies from the provided
	// Container.
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
		return paramSingle{Type: t}, nil
	}
}

// paramVisitor visits every param in a param tree, allowing tracking state at
// each level.
type paramVisitor interface {
	// Visit is called on the param being visited.
	//
	// If Visit returns a non-nil paramVisitor, that paramVisitor visits all
	// the child params of this param.
	Visit(param) paramVisitor

	// We can implement AnnotateWithField and AnnotateWithPosition like
	// resultVisitor if we need to track that information in the future.
}

// paramVisitorFunc is a paramVisitor that visits param in a tree with the
// return value deciding whether the descendants of this param should be
// recursed into.
type paramVisitorFunc func(param) (recurse bool)

func (f paramVisitorFunc) Visit(p param) paramVisitor {
	if f(p) {
		return f
	}
	return nil
}

// walkParam walks the param tree for the given param with the provided
// visitor.
//
// paramVisitor.Visit will be called on the provided param and if a non-nil
// paramVisitor is received, this param's descendants will be walked with that
// visitor.
//
// This is very similar to how go/ast.Walk works.
func walkParam(p param, v paramVisitor) {
	v = v.Visit(p)
	if v == nil {
		return
	}

	switch par := p.(type) {
	case paramSingle:
		// No sub-results
	case paramObject:
		for _, f := range par.Fields {
			walkParam(f.Param, v)
		}
	case paramList:
		for _, p := range par.Params {
			walkParam(p, v)
		}
	default:
		panic(fmt.Sprintf(
			"It looks like you have found a bug in dig. "+
				"Please file an issue at https://github.com/uber-go/dig/issues/ "+
				"and provide the following message: "+
				"received unknown param type %T", p))
	}
}

// paramList holds all arguments of the constructor as params.
//
// NOTE: Build() MUST NOT be called on paramList. Instead, BuildList
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
	panic("It looks like you have found a bug in dig. " +
		"Please file an issue at https://github.com/uber-go/dig/issues/ " +
		"and provide the following message: " +
		"paramList.Build() must never be called")
}

// BuildList returns an ordered list of values which may be passed directly
// to the underlying constructor.
func (pl paramList) BuildList(c *Container) ([]reflect.Value, error) {
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
	if v, ok := c.values[k]; ok {
		return v, nil
	}

	nodes := c.providers[k]
	if len(nodes) == 0 {
		// Unlike in the fallback case below, if a user makes an error
		// requesting an optional value, a good error message ("did you mean
		// X?") will not be used and we'll return a zero value instead.
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
		if _, ok := c.providers[tk]; ok {
			return _noValue, fmt.Errorf(
				"type %v is not in the container, did you mean to use %v?", k, tk)
		}

		return _noValue, fmt.Errorf("type %v isn't in the container", k)
	}

	for _, n := range nodes {
		if err := shallowCheckDependencies(c, n.Params); err != nil {
			if ps.Optional {
				return reflect.Zero(ps.Type), nil
			}
			return _noValue, errWrapf(err, "missing dependencies for %v", k)
		}

		if err := n.Call(c); err != nil {
			return _noValue, errWrapf(err, "failed to build %v", k)
		}
	}

	return c.values[k], nil
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
