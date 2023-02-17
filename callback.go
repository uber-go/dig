// Copyright (c) 2023 Uber Technologies, Inc.
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

// CallbackFuncKind represents the kind of function called in a CallbackInfo.
// Specifically, the function will be either Provided or Decorated.
type CallbackFuncKind int

const (
	// Invalid should never be set in a [CallbackInfo]
	Invalid = iota

	// Provided represents a provided function
	Provided

	// Decorated represents a decorator
	Decorated
)

// CallbackInfo contains information about a function called by Dig
// and is passed to a Callback function registered with [WithCallback].
type CallbackInfo struct {

	// Func is the actual function called by Dig.
	Func interface{}

	// Name is the name of the function, including the package name:
	// <package_name>.<function_name>
	Name string

	// Kind tells whether the function was a provided function
	// or was given as a decorator. See [CallbackFuncKind].
	Kind CallbackFuncKind
}

// Callback is an type containing a function to call when Dig calls
// a provided function or decorator successfully.
type Callback interface {

	// Called gets called when Dig calls a provided function or
	// decorator. A [CallbackInfo] is given as parameter, containing
	// information about the function Dig called.
	Called(CallbackInfo)
}

// WithCallback allows registering a callback function with Dig
// to be called whenever a provided function or decorator of a container
// is called successfully. [Callback] is a simple interface containing
// the function to be called as a callback.
func WithCallback(callback Callback) Option {
	return withCallbackOption{
		callback: callback,
	}
}

type withCallbackOption struct {
	callback Callback
}

var _ Option = withCallbackOption{}

func (o withCallbackOption) String() string {
	return "WithCallback()"
}

func (o withCallbackOption) applyOption(c *Container) {
	c.scope.callbackFunc = o.callback
}
