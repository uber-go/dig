// Copyright (c) 2021 Uber Technologies, Inc.
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
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"time"

	"go.uber.org/dig/internal/dot"
)

const (
	_optionalTag         = "optional"
	_nameTag             = "name"
	_ignoreUnexportedTag = "ignore-unexported"
)

// Unique identification of an object in the graph.
type key struct {
	t reflect.Type

	// Only one of name or group will be set.
	name  string
	group string
}

func (k key) String() string {
	if k.name != "" {
		return fmt.Sprintf("%v[name=%q]", k.t, k.name)
	}
	if k.group != "" {
		return fmt.Sprintf("%v[group=%q]", k.t, k.group)
	}
	return k.t.String()
}

// Option configures a Container. It's included for future functionality;
// currently, there are no concrete implementations.
type Option interface {
	applyOption(*Container)
}

// Container is a directed acyclic graph of types and their dependencies.
type Container struct {
	// Mapping from key to all the constructor node that can provide a value for that
	// key.
	providers map[key][]*constructorNode

	nodes []*constructorNode

	// Values that have already been generated in the container.
	values map[key]reflect.Value

	// Values groups that have already been generated in the container.
	groups map[key][]reflect.Value

	// Source of randomness.
	rand *rand.Rand

	// Flag indicating whether the graph has been checked for cycles.
	isVerifiedAcyclic bool

	// Defer acyclic check on provide until Invoke.
	deferAcyclicVerification bool

	// invokerFn calls a function with arguments provided to Provide or Invoke.
	invokerFn invokerFn

	gh *graphHolder
}

// containerWriter provides write access to the Container's underlying data
// store.
type containerWriter interface {
	// setValue sets the value with the given name and type in the container.
	// If a value with the same name and type already exists, it will be
	// overwritten.
	setValue(name string, t reflect.Type, v reflect.Value)

	// submitGroupedValue submits a value to the value group with the provided
	// name.
	submitGroupedValue(name string, t reflect.Type, v reflect.Value)
}

// containerStore provides access to the Container's underlying data store.
type containerStore interface {
	containerWriter

	// Adds a new graph node to the Container and returns its order.
	newGraphNode(w interface{}) int

	// Returns a slice containing all known types.
	knownTypes() []reflect.Type

	// Retrieves the value with the provided name and type, if any.
	getValue(name string, t reflect.Type) (v reflect.Value, ok bool)

	// Retrieves all values for the provided group and type.
	//
	// The order in which the values are returned is undefined.
	getValueGroup(name string, t reflect.Type) []reflect.Value

	// Returns the providers that can produce a value with the given name and
	// type.
	getValueProviders(name string, t reflect.Type) []provider

	// Returns the providers that can produce values for the given group and
	// type.
	getGroupProviders(name string, t reflect.Type) []provider

	createGraph() *dot.Graph

	// Returns invokerFn function to use when calling arguments.
	invoker() invokerFn
}

// New constructs a Container.
func New(opts ...Option) *Container {
	c := &Container{
		providers: make(map[key][]*constructorNode),
		values:    make(map[key]reflect.Value),
		groups:    make(map[key][]reflect.Value),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
		invokerFn: defaultInvoker,
	}

	c.gh = newGraphHolder(c)

	for _, opt := range opts {
		opt.applyOption(c)
	}
	return c
}

// DeferAcyclicVerification is an Option to override the default behavior
// of container.Provide, deferring the dependency graph validation to no longer
// run after each call to container.Provide. The container will instead verify
// the graph on first `Invoke`.
//
// Applications adding providers to a container in a tight loop may experience
// performance improvements by initializing the container with this option.
func DeferAcyclicVerification() Option {
	return deferAcyclicVerificationOption{}
}

type deferAcyclicVerificationOption struct{}

func (deferAcyclicVerificationOption) String() string {
	return "DeferAcyclicVerification()"
}

func (deferAcyclicVerificationOption) applyOption(c *Container) {
	c.deferAcyclicVerification = true
}

// Changes the source of randomness for the container.
//
// This will help provide determinism during tests.
func setRand(r *rand.Rand) Option {
	return setRandOption{r: r}
}

type setRandOption struct{ r *rand.Rand }

func (o setRandOption) String() string {
	return fmt.Sprintf("setRand(%p)", o.r)
}

func (o setRandOption) applyOption(c *Container) {
	c.rand = o.r
}

// DryRun is an Option which, when set to true, disables invocation of functions supplied to
// Provide and Invoke. Use this to build no-op containers.
func DryRun(dry bool) Option {
	return dryRunOption(dry)
}

type dryRunOption bool

func (o dryRunOption) String() string {
	return fmt.Sprintf("DryRun(%v)", bool(o))
}

func (o dryRunOption) applyOption(c *Container) {
	if o {
		c.invokerFn = dryInvoker
	} else {
		c.invokerFn = defaultInvoker
	}
}

// invokerFn specifies how the container calls user-supplied functions.
type invokerFn func(fn reflect.Value, args []reflect.Value) (results []reflect.Value)

func defaultInvoker(fn reflect.Value, args []reflect.Value) []reflect.Value {
	return fn.Call(args)
}

// Generates zero values for results without calling the supplied function.
func dryInvoker(fn reflect.Value, _ []reflect.Value) []reflect.Value {
	ft := fn.Type()
	results := make([]reflect.Value, ft.NumOut())
	for i := 0; i < ft.NumOut(); i++ {
		results[i] = reflect.Zero(fn.Type().Out(i))
	}

	return results
}

func (c *Container) knownTypes() []reflect.Type {
	typeSet := make(map[reflect.Type]struct{}, len(c.providers))
	for k := range c.providers {
		typeSet[k.t] = struct{}{}
	}

	types := make([]reflect.Type, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Sort(byTypeName(types))
	return types
}

func (c *Container) getValue(name string, t reflect.Type) (v reflect.Value, ok bool) {
	v, ok = c.values[key{name: name, t: t}]
	return
}

func (c *Container) setValue(name string, t reflect.Type, v reflect.Value) {
	c.values[key{name: name, t: t}] = v
}

func (c *Container) getValueGroup(name string, t reflect.Type) []reflect.Value {
	items := c.groups[key{group: name, t: t}]
	// shuffle the list so users don't rely on the ordering of grouped values
	return shuffledCopy(c.rand, items)
}

func (c *Container) submitGroupedValue(name string, t reflect.Type, v reflect.Value) {
	k := key{group: name, t: t}
	c.groups[k] = append(c.groups[k], v)
}

func (c *Container) getValueProviders(name string, t reflect.Type) []provider {
	return c.getProviders(key{name: name, t: t})
}

func (c *Container) getGroupProviders(name string, t reflect.Type) []provider {
	return c.getProviders(key{group: name, t: t})
}

func (c *Container) getProviders(k key) []provider {
	nodes := c.providers[k]
	providers := make([]provider, len(nodes))
	for i, n := range nodes {
		providers[i] = n
	}
	return providers
}

// invokerFn return a function to run when calling function provided to Provide or Invoke. Used for
// running container in dry mode.
func (c *Container) invoker() invokerFn {
	return c.invokerFn
}

func (c *Container) newGraphNode(wrapped interface{}) int {
	return c.gh.NewNode(wrapped)
}

func (c *Container) cycleDetectedError(cycle []int) error {
	var path []cycleErrPathEntry
	for _, n := range cycle {
		if n, ok := c.gh.Lookup(n).(*constructorNode); ok {
			path = append(path, cycleErrPathEntry{
				Key: key{
					t: n.CType(),
				},
				Func: n.Location(),
			})
		}
	}
	return errCycleDetected{Path: path}
}

// String representation of the entire Container
func (c *Container) String() string {
	b := &bytes.Buffer{}
	fmt.Fprintln(b, "nodes: {")
	for k, vs := range c.providers {
		for _, v := range vs {
			fmt.Fprintln(b, "\t", k, "->", v)
		}
	}
	fmt.Fprintln(b, "}")

	fmt.Fprintln(b, "values: {")
	for k, v := range c.values {
		fmt.Fprintln(b, "\t", k, "=>", v)
	}
	for k, vs := range c.groups {
		for _, v := range vs {
			fmt.Fprintln(b, "\t", k, "=>", v)
		}
	}
	fmt.Fprintln(b, "}")

	return b.String()
}

type byTypeName []reflect.Type

func (bs byTypeName) Len() int {
	return len(bs)
}

func (bs byTypeName) Less(i int, j int) bool {
	return fmt.Sprint(bs[i]) < fmt.Sprint(bs[j])
}

func (bs byTypeName) Swap(i int, j int) {
	bs[i], bs[j] = bs[j], bs[i]
}

func shuffledCopy(rand *rand.Rand, items []reflect.Value) []reflect.Value {
	newItems := make([]reflect.Value, len(items))
	for i, j := range rand.Perm(len(items)) {
		newItems[i] = items[j]
	}
	return newItems
}
