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
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	_optionalTag = "optional"
	_nameTag     = "name"
	_groupTag    = "group"
)

// Unique identification of an object in the graph.
type key struct {
	t reflect.Type

	// Only one of name or group will be set.
	name  string
	group string
}

// Option configures a Container. It's included for future functionality;
// currently, there are no concrete implementations.
type Option interface {
	applyOption(*Container)
}

type optionFunc func(*Container)

func (f optionFunc) applyOption(c *Container) { f(c) }

// A ProvideOption modifies the default behavior of Provide. It's included for
// future functionality; currently, there are no concrete implementations.
type ProvideOption interface {
	unimplemented()
}

// An InvokeOption modifies the default behavior of Invoke. It's included for
// future functionality; currently, there are no concrete implementations.
type InvokeOption interface {
	unimplemented()
}

// Container is a directed acyclic graph of types and their dependencies.
type Container struct {
	// Mapping from key to all the nodes that can provide a value for that
	// key.
	providers map[key][]*node

	// Values that have already been generated in the container.
	values map[key]reflect.Value

	// Values groups that have already been generated in the container.
	groups map[key][]reflect.Value

	// Source of randomness.
	rand *rand.Rand
}

// New constructs a Container.
func New(opts ...Option) *Container {
	c := &Container{
		providers: make(map[key][]*node),
		values:    make(map[key]reflect.Value),
		groups:    make(map[key][]reflect.Value),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for _, opt := range opts {
		opt.applyOption(c)
	}
	return c
}

// Changes the source of randomness for the container.
//
// This will help provide determinism during tests.
func setRand(r *rand.Rand) Option {
	return optionFunc(func(c *Container) {
		c.rand = r
	})
}

// Provide teaches the container how to build values of one or more types and
// expresses their dependencies.
//
// The first argument of Provide is a function that accepts zero or more
// parameters and returns one or more results. The function may optionally
// return an error to indicate that it failed to build the value. This
// function will be treated as the constructor for all the types it returns.
// This function will be called AT MOST ONCE when a type produced by it, or a
// type that consumes this function's output, is requested via Invoke. If the
// same types are requested multiple times, the previously produced value will
// be reused.
//
// In addition to accepting constructors that accept dependencies as separate
// arguments and produce results as separate return values, Provide also
// accepts constructors that specify dependencies as dig.In structs and/or
// specify results as dig.Out structs.
func (c *Container) Provide(constructor interface{}, opts ...ProvideOption) error {
	ctype := reflect.TypeOf(constructor)
	if ctype == nil {
		return errors.New("can't provide an untyped nil")
	}
	if ctype.Kind() != reflect.Func {
		return fmt.Errorf("must provide constructor function, got %v (type %v)", constructor, ctype)
	}
	if err := c.provide(constructor, ctype); err != nil {
		return errWrapf(err, "can't provide %v", ctype)
	}
	return nil
}

// Invoke runs the given function after instantiating its dependencies.
//
// Any arguments that the function has are treated as its dependencies. The
// dependencies are instantiated in an unspecified order along with any
// dependencies that they might have.
//
// The function may return an error to indicate failure. The error will be
// returned to the caller as-is.
func (c *Container) Invoke(function interface{}, opts ...InvokeOption) error {
	ftype := reflect.TypeOf(function)
	if ftype == nil {
		return errors.New("can't invoke an untyped nil")
	}
	if ftype.Kind() != reflect.Func {
		return fmt.Errorf("can't invoke non-function %v (type %v)", function, ftype)
	}

	pl, err := newParamList(ftype)
	if err != nil {
		return err
	}

	args, err := pl.BuildList(c)
	if err != nil {
		return errWrapf(err, "failed to get arguments for %v (type %v)", function, ftype)
	}

	returned := reflect.ValueOf(function).Call(args)
	if len(returned) == 0 {
		return nil
	}
	if last := returned[len(returned)-1]; isError(last.Type()) {
		if err, _ := last.Interface().(error); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) provide(ctor interface{}, ctype reflect.Type) error {
	n, err := newNode(ctor, ctype)
	if err != nil {
		return err
	}

	keys, err := c.findAndValidateResults(n)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return fmt.Errorf("%v must provide at least one non-error type", ctype)
	}

	for k := range keys {
		oldProducers := c.providers[k]
		c.providers[k] = append(oldProducers, n)
		if err := c.isAcyclic(n.Params, k); err != nil {
			c.providers[k] = oldProducers
			return errWrapf(err, "%v (%v) introduces a cycle", ctor, ctype)
		}
	}

	return nil
}

// Builds a collection of all result types produced by this node.
func (c *Container) findAndValidateResults(n *node) (map[key]struct{}, error) {
	var err error
	keyPaths := make(map[key]string)
	walkResult(n.Results, connectionVisitor{
		c:        c,
		n:        n,
		err:      &err,
		keyPaths: keyPaths,
	})

	if err != nil {
		return nil, err
	}

	keys := make(map[key]struct{}, len(keyPaths))
	for k := range keyPaths {
		keys[k] = struct{}{}
	}
	return keys, nil
}

// Visits the results of a node and compiles a collection of all the keys
// produced by that node.
type connectionVisitor struct {
	c *Container
	n *node

	// If this points to a non-nil value, we've already encountered an error
	// and should stop traversing.
	err *error

	// Map of keys provided to path that provided this. The path is a string
	// documenting which positional return value or dig.Out attribute is
	// providing this particular key.
	//
	// For example, "[0].Foo" indicates that the value was provided by the Foo
	// attribute of the dig.Out returned as the first result of the
	// constructor.
	keyPaths map[key]string

	// We track the path to the current result here. For example, this will
	// be, ["[1]", "Foo", "Bar"] when we're visiting Bar in,
	//
	//   func() (io.Writer, struct {
	//     dig.Out
	//
	//     Foo struct {
	//       dig.Out
	//
	//       Bar io.Reader
	//     }
	//   })
	currentResultPath []string
}

func (cv connectionVisitor) AnnotateWithField(f resultObjectField) resultVisitor {
	cv.currentResultPath = append(cv.currentResultPath, f.FieldName)
	return cv
}

func (cv connectionVisitor) AnnotateWithPosition(i int) resultVisitor {
	cv.currentResultPath = append(cv.currentResultPath, fmt.Sprintf("[%d]", i))
	return cv
}

func (cv connectionVisitor) Visit(res result) resultVisitor {
	// Already failed. Stop looking.
	if *cv.err != nil {
		return nil
	}

	path := strings.Join(cv.currentResultPath, ".")

	switch r := res.(type) {
	case resultSingle:
		k := key{name: r.Name, t: r.Type}

		if conflict, ok := cv.keyPaths[k]; ok {
			*cv.err = fmt.Errorf(
				"cannot provide %v from %v in constructor %v: already provided by %v",
				k, path, cv.n.ctype, conflict)
			return nil
		}

		if _, ok := cv.c.providers[k]; ok {
			*cv.err = fmt.Errorf(
				"cannot provide %v from %v in constructor %v: already in the container",
				k, path, cv.n.ctype)
			return nil
		}

		cv.keyPaths[k] = path

	case resultGrouped:
		// we don't really care about the path for this since conflicts are
		// okay for group results. We'll track it for the sake of having a
		// value there.
		k := key{group: r.Group, t: r.Type}
		cv.keyPaths[k] = path
	}

	return cv
}

func (c *Container) isAcyclic(p param, k key) error {
	return detectCycles(p, c.providers, []key{k})
}

// node is a node in the dependency graph. Each node maps to a single
// constructor provided by the user.
//
// Nodes can produce zero or more values that they store into the container.
// For the Provide path, we verify that nodes produce at least one value,
// otherwise the function will never be called.
type node struct {
	ctor  interface{}
	ctype reflect.Type

	// Whether the constructor owned by this node was already called.
	called bool

	// Type information about constructor parameters.
	Params paramList

	// Type information about constructor results.
	Results resultList
}

func newNode(ctor interface{}, ctype reflect.Type) (*node, error) {
	params, err := newParamList(ctype)
	if err != nil {
		return nil, err
	}

	results, err := newResultList(ctype)
	if err != nil {
		return nil, err
	}

	return &node{
		ctor:    ctor,
		ctype:   ctype,
		Params:  params,
		Results: results,
	}, err
}

// Call calls this node's constructor if it hasn't already been called and
// injects any values produced by it into the provided container.
func (n *node) Call(c *Container) error {
	if n.called {
		return nil
	}

	args, err := n.Params.BuildList(c)
	if err != nil {
		return errWrapf(err, "couldn't get arguments for constructor %v", n.ctype)
	}

	receiver := newStagingReceiver()
	results := reflect.ValueOf(n.ctor).Call(args)
	n.Results.ExtractList(receiver, results)

	if err := receiver.Commit(c); err != nil {
		return errWrapf(err, "constructor %v failed", n.ctype)
	}

	n.called = true
	return nil
}

type errCycleDetected struct {
	Path []key
	Key  key
}

func (e errCycleDetected) Error() string {
	b := new(bytes.Buffer)
	for _, k := range e.Path {
		fmt.Fprintf(b, "%v ->", k.t)
	}
	fmt.Fprintf(b, "%v", e.Key.t)
	return b.String()
}

func detectCycles(par param, graph map[key][]*node, path []key) error {
	var err error
	walkParam(par, paramVisitorFunc(func(param param) bool {
		if err != nil {
			return false
		}

		var k key
		switch p := param.(type) {
		case paramSingle:
			k = key{name: p.Name, t: p.Type}
		case paramGroupedSlice:
			// NOTE: The key uses the element type, not the slice type.
			k = key{group: p.Group, t: p.Type.Elem()}
		default:
			return true
		}

		for _, p := range path {
			if p == k {
				err = errCycleDetected{Path: path, Key: k}
				return false
			}
		}

		for _, n := range graph[k] {
			if e := detectCycles(n.Params, graph, append(path, k)); e != nil {
				err = e
				return false
			}
		}

		return true
	}))

	return err
}

// Checks if a field of an In struct is optional.
func isFieldOptional(f reflect.StructField) (bool, error) {
	tag := f.Tag.Get(_optionalTag)
	if tag == "" {
		return false, nil
	}

	optional, err := strconv.ParseBool(tag)
	if err != nil {
		err = errWrapf(err,
			"invalid value %q for %q tag on field %v",
			tag, _optionalTag, f.Name)
	}

	return optional, err
}

// Checks that all direct dependencies of the provided param are present in
// the container. Returns an error if not.
func shallowCheckDependencies(c *Container, p param) error {
	var missing []key
	walkParam(p, paramVisitorFunc(func(p param) bool {
		ps, ok := p.(paramSingle)
		if !ok {
			return true
		}

		k := key{name: ps.Name, t: ps.Type}
		if ns := c.providers[k]; len(ns) == 0 && !ps.Optional {
			missing = append(missing, k)
		}

		return true
	}))

	if len(missing) > 0 {
		return fmt.Errorf("container is missing: %v", missing)
	}
	return nil
}

type stagingReceiver struct {
	err    error
	values map[key]reflect.Value
	groups map[key][]reflect.Value
}

func newStagingReceiver() *stagingReceiver {
	return &stagingReceiver{
		values: make(map[key]reflect.Value),
		groups: make(map[key][]reflect.Value),
	}
}

func (sr *stagingReceiver) SubmitError(err error) {
	// record failure only if we haven't already failed
	if sr.err == nil {
		sr.err = err
	}
}

func (sr *stagingReceiver) SubmitValue(name string, t reflect.Type, v reflect.Value) {
	sr.values[key{t: t, name: name}] = v
}

func (sr *stagingReceiver) SubmitGroupValue(group string, t reflect.Type, v reflect.Value) {
	k := key{t: t, group: group}
	sr.groups[k] = append(sr.groups[k], v)
}

// Commit commits the received results to the provided container.
//
// If the resultReceiver failed, no changes are committed to the container.
func (sr *stagingReceiver) Commit(c *Container) error {
	if sr.err != nil {
		return sr.err
	}

	for k, v := range sr.values {
		c.values[k] = v
	}

	for k, vs := range sr.groups {
		c.groups[k] = append(c.groups[k], vs...)
	}

	return nil
}
