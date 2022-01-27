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

package digtest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

// Container wraps dig.Container to provide methods for easier testing.
type Container struct {
	*dig.Container

	t testing.TB
}

// Scope wraps dig.Scope to provide methods for easier testing.
type Scope struct {
	*scope

	t testing.TB
}

// scope is an alias of dig.Scope that allows us to embed dig.Scope into
// digtest.Scope, while still having a method named Scope on it.
//
// Without this, digtest.Scope cannot have a Scope method because it has
// an exported attribute Scope (the embedded dig.Scope) field. That is,
//
//   type Foo struct{ *dig.Scope }
//
//   func (*Foo) Scope()
//
// The above is illegal because it's unclear what Foo.Scope refers to: the
// attribute or the method.
type scope = dig.Scope

// New builds a new testing container.
func New(t testing.TB, opts ...dig.Option) *Container {
	return &Container{
		t:         t,
		Container: dig.New(opts...),
	}
}

// RequireProvide provides the given function to the container,
// halting the test if it fails.
func (c *Container) RequireProvide(f interface{}, opts ...dig.ProvideOption) {
	c.t.Helper()

	require.NoError(c.t, c.Provide(f, opts...), "failed to provide")
}

// RequireProvide provides the given function to the scope,
// halting the test if it fails.
func (s *Scope) RequireProvide(f interface{}, opts ...dig.ProvideOption) {
	s.t.Helper()

	require.NoError(s.t, s.Provide(f, opts...), "failed to provide")
}

// RequireInvoke invokes the given function to the container,
// halting the test if it fails.
func (c *Container) RequireInvoke(f interface{}, opts ...dig.InvokeOption) {
	c.t.Helper()

	require.NoError(c.t, c.Invoke(f, opts...), "failed to invoke")
}

// RequireInvoke invokes the given function to the scope,
// halting the test if it fails.
func (s *Scope) RequireInvoke(f interface{}, opts ...dig.InvokeOption) {
	s.t.Helper()

	require.NoError(s.t, s.Invoke(f, opts...), "failed to invoke")
}

// RequireDecorate decorates the scope using the given function,
// halting the test if it fails.
func (c *Container) RequireDecorate(f interface{}, opts ...dig.DecorateOption) {
	c.t.Helper()

	require.NoError(c.t, c.Decorate(f, opts...), "failed to decorate")
}

// RequireDecorate decorates the scope using the given function,
// halting the test if it fails.
func (s *Scope) RequireDecorate(f interface{}, opts ...dig.DecorateOption) {
	s.t.Helper()

	require.NoError(s.t, s.Decorate(f, opts...), "failed to decorate")
}

// Scope builds a subscope of this container with the given name.
// The returned Scope is similarly augmented to ease testing.
func (c *Container) Scope(name string, opts ...dig.ScopeOption) *Scope {
	return &Scope{
		scope: c.Container.Scope(name, opts...),
		t:     c.t,
	}
}

// Scope builds a subscope of this scope with the given name.
// The returned Scope is similarly augmented to ease testing.
func (s *Scope) Scope(name string, opts ...dig.ScopeOption) *Scope {
	return &Scope{
		scope: s.scope.Scope(name, opts...),
		t:     s.t,
	}
}
