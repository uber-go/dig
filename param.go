// Copyright (c) 2019-2021 Uber Technologies, Inc.
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
	"strconv"
	"strings"

	"go.uber.org/dig/internal/digerror"
	"go.uber.org/dig/internal/dot"
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

	// Build this dependency and any of its dependencies from the provided
	// Container. It stores the result in the pointed-to reflect.Value, allocating
	// it first if it points to an invalid reflect.Value.
	//
	// Build returns a deferred that resolves once the reflect.Value is filled in.
	//
	// This MAY panic if the param does not produce a single value.
	Build(store containerStore, decorating bool, target *reflect.Value) *deferred

	// DotParam returns a slice of dot.Param(s).
	DotParam() []*dot.Param
}

var (
	_ param = paramSingle{}
	_ param = paramObject{}
	_ param = paramList{}
	_ param = paramGroupedSlice{}
)

// newParam builds a param from the given type. If the provided type is a
// dig.In struct, an paramObject will be returned.
func newParam(t reflect.Type, c containerStore) (param, error) {
	switch {
	case IsOut(t) || (t.Kind() == reflect.Ptr && IsOut(t.Elem())) || embedsType(t, _outPtrType):
		return nil, errf("cannot depend on result objects", "%v embeds a dig.Out", t)
	case IsIn(t):
		return newParamObject(t, c)
	case embedsType(t, _inPtrType):
		return nil, errf(
			"cannot build a parameter object by embedding *dig.In, embed dig.In instead",
			"%v embeds *dig.In", t)
	case t.Kind() == reflect.Ptr && IsIn(t.Elem()):
		return nil, errf(
			"cannot depend on a pointer to a parameter object, use a value instead",
			"%v is a pointer to a struct that embeds dig.In", t)
	default:
		return paramSingle{Type: t}, nil
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

func (pl paramList) DotParam() []*dot.Param {
	var types []*dot.Param
	for _, param := range pl.Params {
		types = append(types, param.DotParam()...)
	}
	return types
}

func (pl paramList) String() string {
	args := make([]string, len(pl.Params))
	for i, p := range pl.Params {
		args[i] = p.String()
	}
	return fmt.Sprint(args)
}

// newParamList builds a paramList from the provided constructor type.
//
// Variadic arguments of a constructor are ignored and not included as
// dependencies.
func newParamList(ctype reflect.Type, c containerStore) (paramList, error) {
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
		p, err := newParam(ctype.In(i), c)
		if err != nil {
			return pl, errf("bad argument %d", i+1, err)
		}
		pl.Params = append(pl.Params, p)
	}

	return pl, nil
}

func (pl paramList) Build(containerStore, bool, *reflect.Value) *deferred {
	digerror.BugPanicf("paramList.Build() must never be called")
	panic("") // Unreachable, as BugPanicf above will panic.
}

// BuildList builds an ordered list of values which may be passed directly
// to the underlying constructor and stores them in the pointed-to slice.
// It returns a deferred that resolves when the slice is filled out.
func (pl paramList) BuildList(c containerStore, decorating bool, targets *[]reflect.Value) *deferred {
	children := make([]*deferred, len(pl.Params))
	*targets = make([]reflect.Value, len(pl.Params))
	for i, p := range pl.Params {
		children[i] = p.Build(c, decorating, &(*targets)[i])
	}
	return whenAll(children...)
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

func (ps paramSingle) DotParam() []*dot.Param {
	return []*dot.Param{
		{
			Node: &dot.Node{
				Type: ps.Type,
				Name: ps.Name,
			},
			Optional: ps.Optional,
		},
	}
}

func (ps paramSingle) String() string {
	// tally.Scope[optional] means optional
	// tally.Scope[optional, name="foo"] means named optional

	var opts []string
	if ps.Optional {
		opts = append(opts, "optional")
	}
	if ps.Name != "" {
		opts = append(opts, fmt.Sprintf("name=%q", ps.Name))
	}

	if len(opts) == 0 {
		return fmt.Sprint(ps.Type)
	}

	return fmt.Sprintf("%v[%v]", ps.Type, strings.Join(opts, ", "))
}

// search the given container and its ancestors for a decorated value.
func (ps paramSingle) getDecoratedValue(c containerStore) (reflect.Value, bool) {
	for _, c := range c.storesToRoot() {
		if v, ok := c.getDecoratedValue(ps.Name, ps.Type); ok {
			return v, ok
		}
	}
	return _noValue, false
}

// search the given container and its ancestors for a matching value.
func (ps paramSingle) getValue(c containerStore) (reflect.Value, bool) {
	for _, c := range c.storesToRoot() {
		if v, ok := c.getValue(ps.Name, ps.Type); ok {
			return v, ok
		}
	}
	return _noValue, false
}

// builds the parameter using decorators, if any. useDecorators controls whether to use decorator functions (true) or
// provider functions (false).
func (ps paramSingle) buildWith(c containerStore, useDecorators bool, target *reflect.Value) *deferred {
	var decorators []decorator

	if useDecorators {
		decorators = c.getValueDecorators(ps.Name, ps.Type)

		if len(decorators) == 0 {
			return &alreadyResolved
		}
	} else {
		// A provider is-a decorator ({methods of decorator} ⊆ {methods of provider})
		var providers []provider
		for _, container := range c.storesToRoot() {
			providers = container.getValueProviders(ps.Name, ps.Type)
			if len(providers) > 0 {
				break
			}
		}

		if len(providers) == 0 {
			if ps.Optional {
				target.Set(reflect.Zero(ps.Type))
				return &alreadyResolved
			} else {
				return failedDeferred(newErrMissingTypes(c, key{name: ps.Name, t: ps.Type}))
			}
		}

		decorators = make([]decorator, len(providers))
		for i, provider := range providers {
			decorators[i] = provider.(decorator)
		}
	}

	var (
		doNext func(i int)
		d      = new(deferred)
	)

	doNext = func(i int) {
		if i == len(decorators) {
			// If we get here, it's impossible for the value to be absent from the
			// container.
			v, _ := ps.getValue(c)
			if v.IsValid() {
				// Not valid during a dry run
				target.Set(v)
			}
			d.resolve(nil)
			return
		}

		n := decorators[i]

		n.Call(c).observe(func(err error) {
			if err != nil {
				// If we're missing dependencies but the parameter itself is optional,
				// we can just move on.
				if _, ok := err.(errMissingDependencies); !ok || !ps.Optional {
					d.resolve(errParamSingleFailed{
						CtorID: n.ID(),
						Key:    key{t: ps.Type, name: ps.Name},
						Reason: err,
					})
					return
				}
			}
			doNext(i + 1)
		})
	}

	doNext(0)
	return d
}

func (ps paramSingle) Build(c containerStore, decorating bool, target *reflect.Value) *deferred {
	if !target.IsValid() {
		*target = reflect.New(ps.Type).Elem()
	}

	d := &alreadyResolved

	if !decorating {
		d = ps.buildWith(c, true, target)
	}

	return d.then(func() *deferred {
		// Check whether the value is a decorated value first.
		if v, ok := ps.getDecoratedValue(c); ok {
			target.Set(v)
			return &alreadyResolved
		}

		// See if it's already in the store
		if v, ok := ps.getValue(c); ok {
			target.Set(v)
			return &alreadyResolved
		}

		return ps.buildWith(c, false, target)
	})
}

// paramObject is a dig.In struct where each field is another param.
//
// This object is not expected in the graph as-is.
type paramObject struct {
	Type        reflect.Type
	Fields      []paramObjectField
	FieldOrders []int
}

func (po paramObject) DotParam() []*dot.Param {
	var types []*dot.Param
	for _, field := range po.Fields {
		types = append(types, field.DotParam()...)
	}
	return types
}

func (po paramObject) String() string {
	fields := make([]string, len(po.Fields))
	for i, f := range po.Fields {
		fields[i] = f.Param.String()
	}
	return strings.Join(fields, " ")
}

// getParamOrder returns the order(s) of a parameter type.
func getParamOrder(gh *graphHolder, param param) []int {
	var orders []int
	switch p := param.(type) {
	case paramSingle:
		providers := gh.s.getAllValueProviders(p.Name, p.Type)
		for _, provider := range providers {
			orders = append(orders, provider.Order(gh.s))
		}
	case paramGroupedSlice:
		// value group parameters have nodes of their own.
		// We can directly return that here.
		orders = append(orders, p.orders[gh.s])
	case paramObject:
		for _, pf := range p.Fields {
			orders = append(orders, getParamOrder(gh, pf.Param)...)
		}
	}
	return orders
}

// newParamObject builds an paramObject from the provided type. The type MUST
// be a dig.In struct.
func newParamObject(t reflect.Type, c containerStore) (paramObject, error) {
	po := paramObject{Type: t}

	// Check if the In type supports ignoring unexported fields.
	var ignoreUnexported bool
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _inType {
			var err error
			ignoreUnexported, err = isIgnoreUnexportedSet(f)
			if err != nil {
				return po, err
			}
			break
		}
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _inType {
			// Skip over the dig.In embed.
			continue
		}
		if f.PkgPath != "" && ignoreUnexported {
			// Skip over an unexported field if it is allowed.
			continue
		}
		pof, err := newParamObjectField(i, f, c)
		if err != nil {
			return po, errf("bad field %q of %v", f.Name, t, err)
		}
		po.Fields = append(po.Fields, pof)
	}
	return po, nil
}

func (po paramObject) Build(c containerStore, decorating bool, target *reflect.Value) *deferred {
	if !target.IsValid() {
		*target = reflect.New(po.Type).Elem()
	}

	children := make([]*deferred, len(po.Fields))
	for i, f := range po.Fields {
		f := f
		field := target.Field(f.FieldIndex)
		children[i] = f.Build(c, decorating, &field)
	}

	return whenAll(children...)
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

func (pof paramObjectField) DotParam() []*dot.Param {
	return pof.Param.DotParam()
}

func newParamObjectField(idx int, f reflect.StructField, c containerStore) (paramObjectField, error) {
	pof := paramObjectField{
		FieldName:  f.Name,
		FieldIndex: idx,
	}

	var p param
	switch {
	case f.PkgPath != "":
		return pof, errf(
			"unexported fields not allowed in dig.In, did you mean to export %q (%v)?",
			f.Name, f.Type)

	case f.Tag.Get(_groupTag) != "":
		var err error
		p, err = newParamGroupedSlice(f, c)
		if err != nil {
			return pof, err
		}

	default:
		var err error
		p, err = newParam(f.Type, c)
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

func (pof paramObjectField) Build(c containerStore, decorating bool, target *reflect.Value) *deferred {
	return pof.Param.Build(c, decorating, target)
}

// paramGroupedSlice is a param which produces a slice of values with the same
// group name.
type paramGroupedSlice struct {
	// Name of the group as specified in the `group:".."` tag.
	Group string

	// Type of the slice.
	Type reflect.Type

	orders map[*Scope]int
}

func (pt paramGroupedSlice) String() string {
	// io.Reader[group="foo"] refers to a group of io.Readers called 'foo'
	return fmt.Sprintf("%v[group=%q]", pt.Type.Elem(), pt.Group)
}

func (pt paramGroupedSlice) DotParam() []*dot.Param {
	return []*dot.Param{
		{
			Node: &dot.Node{
				Type:  pt.Type,
				Group: pt.Group,
			},
		},
	}
}

// newParamGroupedSlice builds a paramGroupedSlice from the provided type with
// the given name.
//
// The type MUST be a slice type.
func newParamGroupedSlice(f reflect.StructField, c containerStore) (paramGroupedSlice, error) {
	g, err := parseGroupString(f.Tag.Get(_groupTag))
	if err != nil {
		return paramGroupedSlice{}, err
	}
	pg := paramGroupedSlice{Group: g.Name, Type: f.Type, orders: make(map[*Scope]int)}

	name := f.Tag.Get(_nameTag)
	optional, _ := isFieldOptional(f)
	switch {
	case f.Type.Kind() != reflect.Slice:
		return pg, errf("value groups may be consumed as slices only",
			"field %q (%v) is not a slice", f.Name, f.Type)
	case g.Flatten:
		return pg, errf("cannot use flatten in parameter value groups",
			"field %q (%v) specifies flatten", f.Name, f.Type)
	case name != "":
		return pg, errf(
			"cannot use named values with value groups",
			"name:%q requested with group:%q", name, pg.Group)

	case optional:
		return pg, errors.New("value groups cannot be optional")
	}
	c.newGraphNode(&pg, pg.orders)
	return pg, nil
}

// retrieves any decorated values that may be committed in this scope, or
// any of the parent Scopes. In the case where there are multiple scopes that
// are decorating the same type, the closest scope in effect will be replacing
// any decorated value groups provided in further scopes.
func (pt paramGroupedSlice) getDecoratedValues(c containerStore) (reflect.Value, bool) {
	for _, c := range c.storesToRoot() {
		if items, ok := c.getDecoratedValueGroup(pt.Group, pt.Type); ok {
			return items, true
		}
	}
	return _noValue, false
}

// search the given container and its parents for matching group decorators
// and call them to commit values. If any decorators return an error,
// that error is returned immediately. If all decorators succeeds, nil is returned.
// The order in which the decorators are invoked is from the top level scope to
// the current scope, to account for decorators that decorate values that were
// already decorated.
func (pt paramGroupedSlice) callGroupDecorators(c containerStore) *deferred {
	var children []*deferred
	stores := c.storesToRoot()
	for i := len(stores) - 1; i >= 0; i-- {
		c := stores[i]
		for _, d := range c.getGroupDecorators(pt.Group, pt.Type.Elem()) {
			d := d
			child := d.Call(c)
			children = append(children, child.catch(func(err error) error {
				return errParamGroupFailed{
					CtorID: d.ID(),
					Key:    key{group: pt.Group, t: pt.Type.Elem()},
					Reason: err,
				}
			}))
		}
	}
	return whenAll(children...)
}

// search the given container and its parent for matching group providers and
// call them to commit values. If an error is encountered, return the number
// of providers called and a non-nil error from the first provided.
func (pt paramGroupedSlice) callGroupProviders(c containerStore) *deferred {
	var children []*deferred
	for _, c := range c.storesToRoot() {
		providers := c.getGroupProviders(pt.Group, pt.Type.Elem())
		for _, n := range providers {
			n := n
			child := n.Call(c)
			children = append(children, child.catch(func(err error) error {
				return errParamGroupFailed{
					CtorID: n.ID(),
					Key:    key{group: pt.Group, t: pt.Type.Elem()},
					Reason: err,
				}
			}))
		}
	}
	return whenAll(children...)
}

func (pt paramGroupedSlice) Build(c containerStore, decorating bool, target *reflect.Value) *deferred {
	d := &alreadyResolved

	// do not call this if we are already inside a decorator since
	// it will result in an infinite recursion. (i.e. decorate -> params.BuildList() -> Decorate -> params.BuildList...)
	// this is safe since a value can be decorated at most once in a given scope.
	if !decorating {
		d = pt.callGroupDecorators(c)
	}

	return d.then(func() *deferred {
		// Check if we have decorated values
		if decoratedItems, ok := pt.getDecoratedValues(c); ok {
			if !target.IsValid() {
				newCap := 0
				if decoratedItems.Kind() == reflect.Slice {
					newCap = decoratedItems.Len()
				}
				*target = reflect.MakeSlice(pt.Type, 0, newCap)
			}

			target.Set(decoratedItems)
			return &alreadyResolved
		}

		// If we do not have any decorated values, find the
		// providers and call them.
		return pt.callGroupProviders(c).then(func() *deferred {
			for _, c := range c.storesToRoot() {
				target.Set(reflect.Append(*target, c.getValueGroup(pt.Group, pt.Type.Elem())...))
			}
			return &alreadyResolved
		})
	})
}

// Checks if ignoring unexported files in an In struct is allowed.
// The struct field MUST be an _inType.
func isIgnoreUnexportedSet(f reflect.StructField) (bool, error) {
	tag := f.Tag.Get(_ignoreUnexportedTag)
	if tag == "" {
		return false, nil
	}

	allowed, err := strconv.ParseBool(tag)
	if err != nil {
		err = errf(
			"invalid value %q for %q tag on field %v",
			tag, _ignoreUnexportedTag, f.Name, err)
	}

	return allowed, err
}
