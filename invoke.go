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
	"errors"
	"reflect"

	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/graph"
)

// An InvokeOption modifies the default behavior of Invoke. It's included for
// future functionality; currently, there are no concrete implementations.
type InvokeOption interface {
	unimplemented()
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
	return c.scope.Invoke(function, opts...)
}

// Invoke runs the given function after instantiating its dependencies.
//
// Any arguments that the function has are treated as its dependencies. The
// dependencies are instantiated in an unspecified order along with any
// dependencies that they might have.
//
// The function may return an error to indicate failure. The error will be
// returned to the caller as-is.
func (s *Scope) Invoke(function interface{}, opts ...InvokeOption) error {
	ftype := reflect.TypeOf(function)
	if ftype == nil {
		return errors.New("can't invoke an untyped nil")
	}
	if ftype.Kind() != reflect.Func {
		return errf("can't invoke non-function %v (type %v)", function, ftype)
	}

	pl, err := newParamList(ftype, s)
	if err != nil {
		return err
	}

	if err := shallowCheckDependencies(s, pl); err != nil {
		return errMissingDependencies{
			Func:   digreflect.InspectFunc(function),
			Reason: err,
		}
	}

	if !s.isVerifiedAcyclic {
		if ok, cycle := graph.IsAcyclic(s.gh); !ok {
			return errf("cycle detected in dependency graph", s.cycleDetectedError(cycle))
		}
		s.isVerifiedAcyclic = true
	}

	var args []reflect.Value

	d := pl.BuildList(s, false /* decorating */, &args)
	d.Observe(func(err2 error) {
		err = err2
	})
	s.sched.Flush()

	if err != nil {
		return errArgumentsFailed{
			Func:   digreflect.InspectFunc(function),
			Reason: err,
		}
	}
	returned := s.invokerFn(reflect.ValueOf(function), args)
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

// Checks that all direct dependencies of the provided parameters are present in
// the container. Returns an error if not.
func shallowCheckDependencies(c containerStore, pl paramList) error {
	var err errMissingTypes

	missingDeps := findMissingDependencies(c, pl.Params...)
	for _, dep := range missingDeps {
		err = append(err, newErrMissingTypes(c, key{name: dep.Name, t: dep.Type})...)
	}

	if len(err) > 0 {
		return err
	}
	return nil
}

func findMissingDependencies(c containerStore, params ...param) []paramSingle {
	var missingDeps []paramSingle

	for _, param := range params {
		switch p := param.(type) {
		case paramSingle:
			allProviders := c.getAllValueProviders(p.Name, p.Type)
			_, hasDecoratedValue := c.getDecoratedValue(p.Name, p.Type)
			// This means that there is no provider that provides this value,
			// and it is NOT being decorated and is NOT optional.
			// In the case that there is no providers but there is a decorated value
			// of this type, it can be provided safely so we can safely skip this.
			if len(allProviders) == 0 && !hasDecoratedValue && !p.Optional {
				missingDeps = append(missingDeps, p)
			}
		case paramObject:
			for _, f := range p.Fields {
				missingDeps = append(missingDeps, findMissingDependencies(c, f.Param)...)
			}
		}
	}
	return missingDeps
}
