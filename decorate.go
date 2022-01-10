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
)

type decorator interface {
	Call(c containerStore) error
	ID() dot.CtorID
}

type decoratorNode struct {
	dcor  interface{}
	dtype reflect.Type

	id dot.CtorID

	// Location where this function was defined.
	location *digreflect.Func

	// Whether the decorator owned by this node was already called.
	called bool

	// Parameters of the decorator
	params paramList

	// Results of the decorator
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

	// Create parameter / result list.
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

func (n *decoratorNode) Call(s containerStore) error {
	if n.called {
		return nil
	}

	if err := shallowCheckDependencies(s, n.params); err != nil {
		return errMissingDependencies{
			Func:   n.location,
			Reason: err,
		}
	}

	args, err := n.params.BuildList(n.s, true)
	if err != nil {
		return errArgumentsFailed{
			Func:   n.location,
			Reason: err,
		}
	}

	results := reflect.ValueOf(n.dcor).Call(args)
	if err != nil {
		return nil
	}

	if err := n.results.ExtractList(n.s, true, results); err != nil {
		return err
	}
	n.called = true
	return nil
}

func (n *decoratorNode) ID() dot.CtorID { return n.id }

// DecorateOption modifies the default behavior of Provide.
// Currently there is no implementation of it yet.
type DecorateOption interface {
	applyDecorateOption(*decorateOptions)
}

type decorateOptions struct {
}

// Decorate provides a decorator for a type that has already been provided in the Container.
//
// Similar to Provide, Decorate takes in a function with zero or more dependencies and one
// or more results. Decorate can be used to modify a type that was already introduced to the
// container, or completely replace it with a new object.
//
// For example,
//  c.Decorate(func(log *zap.Logger) *zap.Logger {
//    return log.Named("myapp")
//  }
//
// takes in a value, augments it with a name, and returns a replacement for it. Functions
// in the container's dependency graph that use *zap.Logger will now use the *zap.Logger
// returned by this decorator.
//
// A decorator can also take in multiple parameters and replace one of them:
//  c.Decorate(func(log *zap.Logger, cfg *Config) *zap.Logger {
//    return log.Named(cfg.Name)
//  }
//
// Or replace a subset of them:
//  c.Decorate(func(
//    log *zap.Logger,
//    cfg *Config,
//    scope metrics.Scope
//  ) (*zap.Logger, metrics.Scope) {
//    log = log.Named(cfg.Name)
//    scope = scope.With(metrics.Tag("service", cfg.Name))
//    return log, scope
//  })
//
// Decorating a Container affects all the child scopes of this Container.
//
// Similar to a provider, the decorator function gets called *at most once*.
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
//  }
//
// takes in a value, augments it with a name, and returns a replacement for it. Functions
// in the Scope's dependency graph that use *zap.Logger will now use the *zap.Logger
// returned by this decorator.
//
// A decorator can also take in multiple parameters and replace one of them:
//  s.Decorate(func(log *zap.Logger, cfg *Config) *zap.Logger {
//    return log.Named(cfg.Name)
//  }
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
	options := decorateOptions{}
	for _, opt := range opts {
		opt.applyDecorateOption(&options)
	}

	dn, err := newDecoratorNode(decorator, s)
	if err != nil {
		return err
	}

	keys := findResultKeys(dn.results)
	for _, k := range keys {
		if len(s.decorators[k]) > 0 {
			return fmt.Errorf("cannot decorate using function %v: decorating %s multiple times",
				dn.dtype,
				k,
			)
		}
		s.decorators[k] = append(s.decorators[k], dn)
	}
	return nil
}

func findResultKeys(r resultList) []key {
	// use BFS to search for all keys included in a resultList
	var q []result
	var keys []key
	q = append(q, r)

	for len(q) > 0 {
		res := q[0]
		q = q[1:]

		switch innerResult := res.(type) {
		case resultSingle:
			keys = append(keys, key{t: innerResult.Type, name: innerResult.Name})
		case resultGrouped:
			// Flatten the result.
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
