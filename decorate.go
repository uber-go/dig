// Copyright (c) 2022 Uber Technologies, Inc.
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

	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/dot"
	"go.uber.org/dig/internal/promise"
)

type decorator interface {
	Call(c containerStore) *promise.Deferred
	ID() dot.CtorID
}

type decoratorNode struct {
	dcor  interface{}
	dtype reflect.Type

	id dot.CtorID

	// Location where this function was defined.
	location *digreflect.Func

	// Whether this node is already building its paramList and calling the constructor
	calling bool

	// Whether the decorator owned by this node was already called.
	called bool

	// Parameters of the decorator.
	params paramList

	// The result of calling the constructor
	deferred promise.Deferred

	// Results of the decorator.
	results resultList

	// order of this node in each Scopes' graphHolders.
	orders map[*Scope]int

	// scope this node was originally provided to.
	s *Scope
}

func newDecoratorNode(dcor interface{}, s *Scope) (*decoratorNode, error) {
	dval := reflect.ValueOf(dcor)
	dtype := dval.Type()
	dptr := dval.Pointer()

	pl, err := newParamList(dtype, s)
	if err != nil {
		return nil, err
	}

	rl, err := newResultList(dtype, resultOptions{})
	if err != nil {
		return nil, err
	}

	n := &decoratorNode{
		dcor:     dcor,
		dtype:    dtype,
		id:       dot.CtorID(dptr),
		location: digreflect.InspectFunc(dcor),
		orders:   make(map[*Scope]int),
		params:   pl,
		results:  rl,
		s:        s,
	}
	return n, nil
}

// Call calls this decorator if it hasn't already been called and injects any values produced by it into the container
// passed to newConstructorNode.
//
// If constructorNode has a unresolved deferred already in the process of building, it will return that one. If it has
// already been successfully called, it will return an already-resolved deferred. Together these mean it will try the
// call again if it failed last time.
//
// On failure, the returned pointer is not guaranteed to stay in a failed state; another call will reset it back to its
// zero value; don't store the returned pointer. (It will still call each observer only once.)
func (n *decoratorNode) Call(s containerStore) *promise.Deferred {
	if n.calling || n.called {
		return &n.deferred
	}

	n.calling = true
	n.deferred = promise.Deferred{}

	if err := shallowCheckDependencies(s, n.params); err != nil {
		n.deferred.Resolve(errMissingDependencies{
			Func:   n.location,
			Reason: err,
		})
	}

	var args []reflect.Value
	d := n.params.BuildList(s, true /* decorating */, &args)

	d.Observe(func(err error) {
		if err != nil {
			n.calling = false
			n.deferred.Resolve(errArgumentsFailed{
				Func:   n.location,
				Reason: err,
			})
			return
		}

		var results []reflect.Value

		s.scheduler().Schedule(func() {
			results = s.invoker()(reflect.ValueOf(n.dcor), args)
		}).Observe(func(_ error) {
			n.calling = false
			if err := n.results.ExtractList(n.s, true /* decorated */, results); err != nil {
				n.deferred.Resolve(err)
				return
			}

			n.called = true
			n.deferred.Resolve(nil)
		})
	})

	return &n.deferred
}

func (n *decoratorNode) ID() dot.CtorID { return n.id }

// DecorateOption modifies the default behavior of Provide.
// Currently, there is no implementation of it yet.
type DecorateOption interface {
	noOptionsYet()
}

// Decorate provides a decorator for a type that has already been provided in the Container.
// Decorations at this level affect all scopes of the container.
// See Scope.Decorate for information on how to use this method.
func (c *Container) Decorate(decorator interface{}, opts ...DecorateOption) error {
	return c.scope.Decorate(decorator, opts...)
}

// Decorate provides a decorator for a type that has already been provided in the Scope.
//
// Similar to Provide, Decorate takes in a function with zero or more dependencies and one
// or more results. Decorate can be used to modify a type that was already introduced to the
// Scope, or completely replace it with a new object.
//
// For example,
//  s.Decorate(func(log *zap.Logger) *zap.Logger {
//    return log.Named("myapp")
//  })
//
// This takes in a value, augments it with a name, and returns a replacement for it. Functions
// in the Scope's dependency graph that use *zap.Logger will now use the *zap.Logger
// returned by this decorator.
//
// A decorator can also take in multiple parameters and replace one of them:
//  s.Decorate(func(log *zap.Logger, cfg *Config) *zap.Logger {
//    return log.Named(cfg.Name)
//  })
//
// Or replace a subset of them:
//  s.Decorate(func(
//    log *zap.Logger,
//    cfg *Config,
//    scope metrics.Scope
//  ) (*zap.Logger, metrics.Scope) {
//    log = log.Named(cfg.Name)
//    scope = scope.With(metrics.Tag("service", cfg.Name))
//    return log, scope
//  })
//
// Decorating a Scope affects all the child scopes of this Scope.
//
// Similar to a provider, the decorator function gets called *at most once*.
func (s *Scope) Decorate(decorator interface{}, opts ...DecorateOption) error {
	_ = opts // there are no options at this time

	dn, err := newDecoratorNode(decorator, s)
	if err != nil {
		return err
	}

	keys := findResultKeys(dn.results)
	for _, k := range keys {
		if len(s.decorators[k]) > 0 {
			return fmt.Errorf("cannot decorate using function %v: %s already decorated",
				dn.dtype,
				k,
			)
		}
		s.decorators[k] = append(s.decorators[k], dn)
	}
	return nil
}

func findResultKeys(r resultList) []key {
	// use BFS to search for all keys included in a resultList.
	var (
		q    []result
		keys []key
	)
	q = append(q, r)

	for len(q) > 0 {
		res := q[0]
		q = q[1:]

		switch innerResult := res.(type) {
		case resultSingle:
			keys = append(keys, key{t: innerResult.Type, name: innerResult.Name})
		case resultGrouped:
			keys = append(keys, key{t: innerResult.Type.Elem(), group: innerResult.Group})
		case resultObject:
			for _, f := range innerResult.Fields {
				q = append(q, f.Result)
			}
		case resultList:
			q = append(q, innerResult.Results...)
		}
	}
	return keys
}
