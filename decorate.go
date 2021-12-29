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
	"fmt"
	"reflect"

	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/graph"
)

type decoratorNode struct {
	dcor  interface{}
	dtype reflect.Type

	// Location where this function was defined.
	location *digreflect.Func

	// Whether the decorator owned by this node was already called.
	called bool

	// parameters being decorated
	params []key

	// order of this node in each Scopes' graphHolders.
	orders map[*Scope]int

	// scope this node was originally provided to.
	s *Scope
}

func newDecoratorNode(dcor interface{}, s *Scope) (*decoratorNode, error) {
	dtype := reflect.ValueOf(dcor).Type()

	dcorParams := make([]key, dtype.NumIn())

	// Iterate through the parameters and make sure there is at least
	// one provider for this decorator.
	// Otherwise, that's an error.

	/*
		for i := 0; i < dtype.NumIn(); i++ {
			k := key{t: dtype.In(i)}

			var providers []provider
			if providers = s.getAllProviders(k); len(providers) == 0 {
				return nil, fmt.Errorf("cannot decorate using function %v: %s was never Provided to Scope [%s]",
					dtype,
					dtype.In(i),
					s.name,
				)
			}
			dcorParams[i] = k
		}
	*/

	// Iterate through the Out types and make sure they are not already
	// decorated.
	for i := 0; i < dtype.NumOut(); i++ {
		k := key{t: dtype.Out(i)}
		if _, ok := s.decoratedValues[k]; ok {
			return nil, fmt.Errorf("cannot decorate using function %v: %s was already Decorated in Scope [%s]",
				dtype,
				dtype.Out(i),
				s.name,
			)
		}

	}

	n := &decoratorNode{
		dcor:     dcor,
		dtype:    dtype,
		location: digreflect.InspectFunc(dcor),
		orders:   make(map[*Scope]int),
		params:   dcorParams,
		s:        s,
	}
	return n, nil
}

type DecorateOption interface {
	applyDecorateOption(*decorateOptions)
}

type decorateOptions struct {
}

// Decorate ...
func (c *Container) Decorate(decorator interface{}, opts ...DecorateOption) error {
	return c.scope.Decorate(decorator, opts...)
}

// Decorate ...
func (s *Scope) Decorate(decorator interface{}, opts ...DecorateOption) error {
	options := decorateOptions{}
	for _, opt := range opts {
		opt.applyDecorateOption(&options)
	}

	dn, err := newDecoratorNode(decorator, s)
	if err != nil {
		return err
	}

	dn.orders[s] = s.gh.NewNode(dn)
	if !s.deferAcyclicVerification {
		if ok, cycle := graph.IsAcyclic(s.gh); !ok {
			return errf("cycle detected in dependency graph", s.cycleDetectedError(cycle))
		}
		s.isVerifiedAcyclic = true
	}

	pl, err := newParamList(dn.dtype, s)
	if err != nil {
		return err
	}

	if err := shallowCheckDependencies(s, pl); err != nil {
		return errMissingDependencies{
			Func:   dn.location,
			Reason: err,
		}
	}

	args, err := pl.BuildList(s)
	if err != nil {
		return errArgumentsFailed{
			Func:   dn.location,
			Reason: err,
		}
	}
	results := reflect.ValueOf(decorator).Call(args)
	/*
		should we check for error here?
		for _, result := range results {
			if err, _ := result.Interface().(error); err != nil {
				return errf("failed to decorate", err)
			}
		}
	*/
	for _, result := range results {
		s.decoratedValues[key{t: result.Type()}] = result
	}
	return nil
}
