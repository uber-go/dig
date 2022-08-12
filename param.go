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
	"go.uber.org/dig/internal/promise"
)

// The param interface represents a dependency for a constructor.
//
// The following implementations exist:
//
//	paramList     All arguments of the constructor.
//	paramSingle   An explicitly requested type.
//	paramObject   dig.In struct where each field in the struct can be another
//	              param.
//	paramGroupedSlice
//	              A slice consuming a value group. This will receive all
//	              values produced with a `group:".."` tag with the same name
//	              as a slice.
type param interface {
	fmt.Stringer

	// Build this dependency and any of its dependencies from the provided
	// Container. It stores the result in the pointed-to reflect.Value, allocating
	// it first if it points to an invalid reflect.Value.
	//
	// Build returns a deferred that resolves once the reflect.Value is filled in.
	//
	// This MAY panic if the param does not produce a single value.
	Build(store containerStore, target *reflect.Value) *promise.Deferred

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

func (pl paramList) Build(containerStore, *reflect.Value) *promise.Deferred {
	digerror.BugPanicf("paramList.Build() must never be called")
	panic("") // Unreachable, as BugPanicf above will panic.
}

// BuildList builds an ordered list of values which may be passed directly
// to the underlying constructor and stores them in the pointed-to slice.
// It returns a deferred that resolves when the slice is filled out.
func (pl paramList) BuildList(c containerStore, targets *[]reflect.Value) *promise.Deferred {
	children := make([]*promise.Deferred, len(pl.Params))
	*targets = make([]reflect.Value, len(pl.Params))
	for i, p := range pl.Params {
		children[i] = p.Build(c, &(*targets)[i])
	}
	return promise.WhenAll(children...)
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

func (ps paramSingle) buildWithDecorators(c containerStore, target *reflect.Value) (*promise.Deferred, bool) {
	var (
		d               decorator
		decoratingScope containerStore
		found           bool
	)

	def := new(promise.Deferred)
	for _, s := range c.storesToRoot() {
		if d, found = s.getValueDecorator(ps.Name, ps.Type); !found {
			continue
		}
		// This is for avoiding cycles i.e decorator -> function
		//                                      ^           |
		//                                       \ ------- /
		if d.State() == functionVisited {
			d = nil
			continue
		}
		decoratingScope = s
		break
	}
	if !found || d == nil {
		return promise.Done, false
	}
	d.Call(decoratingScope).Observe(func(err error) {
		if err != nil {
			def.Resolve(errParamSingleFailed{
				CtorID: d.ID(),
				Key:    key{t: ps.Type, name: ps.Name},
				Reason: err,
			})
			return
		}
		v, _ := decoratingScope.getDecoratedValue(ps.Name, ps.Type)
		if v.IsValid() {
			target.Set(v)
		}
		def.Resolve(nil)
	})
	return def, found
}

func (ps paramSingle) build(c containerStore, target *reflect.Value) *promise.Deferred {
	var providingContainer containerStore
	var providers []provider
	def := new(promise.Deferred)
	for _, container := range c.storesToRoot() {
		// First we check if the value it's stored in the current store
		if v, ok := container.getValue(ps.Name, ps.Type); ok {
			target.Set(v)
			def.Resolve(nil)
			return def
		}

		providers = container.getValueProviders(ps.Name, ps.Type)
		if len(providers) > 0 {
			providingContainer = container
			break
		}
	}
	if len(providers) == 0 {
		if ps.Optional {
			target.Set(reflect.Zero(ps.Type))
			def.Resolve(nil)
		}
		def.Resolve(newErrMissingTypes(c, key{name: ps.Name, t: ps.Type}))
		return def
	}
	for _, n := range providers {
		n.Call(n.OrigScope()).Observe(func(err error) {
			if err == nil {
				// If we get here, it's impossible for the value to be absent from the
				// container.
				v, _ := providingContainer.getValue(ps.Name, ps.Type)
				if target.IsValid() {
					target.Set(v)
				}
				def.Resolve(nil)
				return
			}

			// If we're missing dependencies but the parameter itself is optional,
			// we can just move on.
			if _, ok := err.(errMissingDependencies); ok && ps.Optional {
				return
			}
			def.Resolve(errParamSingleFailed{
				CtorID: n.ID(),
				Key:    key{t: ps.Type, name: ps.Name},
				Reason: err,
			})
		})
	}
	return def
}

func (ps paramSingle) Build(c containerStore, target *reflect.Value) *promise.Deferred {
	if !target.IsValid() {
		*target = reflect.New(ps.Type).Elem()
	}

	// try building with decorators first, in case this parameter has decorators.
	d, found := ps.buildWithDecorators(c, target)

	return d.Then(func() *promise.Deferred {
		// Check whether the value is a decorated value first.
		if found {
			return promise.Done
		}

		return ps.build(c, target)
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

func (po paramObject) Build(c containerStore, target *reflect.Value) *promise.Deferred {
	if !target.IsValid() {
		*target = reflect.New(po.Type).Elem()
	}
	// We have to build soft groups after all other fields, to avoid cases
	// when a field calls a provider for a soft value group, but the value is
	// not provided to it because the value group is declared before the field
	var softGroupsQueue []paramObjectField
	var fields []paramObjectField
	for _, f := range po.Fields {
		if p, ok := f.Param.(paramGroupedSlice); ok && p.Soft {
			softGroupsQueue = append(softGroupsQueue, f)
			continue
		}
		fields = append(fields, f)
	}

	buildFields := func(fields []paramObjectField) *promise.Deferred {
		children := make([]*promise.Deferred, len(fields))

		for i, f := range fields {
			f := f
			field := target.Field(f.FieldIndex)
			children[i] = f.Build(c, &field)
		}

		return promise.WhenAll(children...)
	}

	return buildFields(fields).Then(func() *promise.Deferred {
		return buildFields(softGroupsQueue)
	})
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

func (pof paramObjectField) Build(c containerStore, target *reflect.Value) *promise.Deferred {
	return pof.Param.Build(c, target)
}

// paramGroupedSlice is a param which produces a slice of values with the same
// group name.
type paramGroupedSlice struct {
	// Name of the group as specified in the `group:".."` tag.
	Group string

	// Type of the slice.
	Type reflect.Type

	// Soft is used to denote a soft dependency between this param and its
	// constructors, if it's true its constructors are only called if they
	// provide another value requested in the graph
	Soft bool

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
	pg := paramGroupedSlice{
		Group:  g.Name,
		Type:   f.Type,
		orders: make(map[*Scope]int),
		Soft:   g.Soft,
	}

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
// The order in which the decorators are invoked is from the top level scope to
// the current scope, to account for decorators that decorate values that were
// already decorated.
func (pt paramGroupedSlice) callGroupDecorators(c containerStore) *promise.Deferred {
	var children []*promise.Deferred
	stores := c.storesToRoot()
	for i := len(stores) - 1; i >= 0; i-- {
		c := stores[i]
		if d, ok := c.getGroupDecorator(pt.Group, pt.Type.Elem()); ok {

			if d.State() == functionVisited {
				// This decorator is already being run. Avoid cycle
				// and look further.
				continue
			}

			child := d.Call(c)
			children = append(children, child.Catch(func(err error) error {
				return errParamGroupFailed{
					CtorID: d.ID(),
					Key:    key{group: pt.Group, t: pt.Type.Elem()},
					Reason: err,
				}
			}))
		}
	}
	return promise.WhenAll(children...)
}

// search the given container and its parent for matching group providers and
// call them to commit values. If an error is encountered, return the number
// of providers called and a non-nil error from the first provided.
func (pt paramGroupedSlice) callGroupProviders(c containerStore) *promise.Deferred {
	var children []*promise.Deferred
	for _, c := range c.storesToRoot() {
		providers := c.getGroupProviders(pt.Group, pt.Type.Elem())
		for _, n := range providers {
			n := n
			child := n.Call(c)
			children = append(children, child.Catch(func(err error) error {
				return errParamGroupFailed{
					CtorID: n.ID(),
					Key:    key{group: pt.Group, t: pt.Type.Elem()},
					Reason: err,
				}
			}))
		}
	}
	return promise.WhenAll(children...)
}

func (pt paramGroupedSlice) Build(c containerStore, target *reflect.Value) *promise.Deferred {
	// do not call this if we are already inside a decorator since
	// it will result in an infinite recursion. (i.e. decorate -> params.BuildList() -> Decorate -> params.BuildList...)
	// this is safe since a value can be decorated at most once in a given scope.
	d := pt.callGroupDecorators(c)

	return d.Then(func() *promise.Deferred {
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
			return promise.Done
		}
		setValues := func() *promise.Deferred {
			for _, c := range c.storesToRoot() {
				target.Set(reflect.Append(*target, c.getValueGroup(pt.Group, pt.Type.Elem())...))
			}
			return promise.Done
		}
		if pt.Soft {
			return setValues()
		}
		// If we do not have any decorated values and the group isn't soft,
		// find the providers and call them.
		return pt.callGroupProviders(c).Then(setValues)
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
