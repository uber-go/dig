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
	"math/rand"
	"reflect"
)

// Scope is a scoped DAG of types and their dependencies.
// A Scope may also have one or more child Scopes that "inherit"
// from it.
type Scope struct {
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

	// Parent of this Scope.
	parentScope *Scope

	// All the child scopes of this Scope.
	childScopes []*Scope
}

// ScopeOption configures a Scope. It's included for future functionality;
// currently, there are no concrete implementations.
type ScopeOption interface {
	applyOption(*Scope)
}

type scopeOptionFunc func(*Scope)

func (f scopeOptionFunc) applyOption(s *Scope) { f(s) }
