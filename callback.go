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

// CallbackInfo contains information about a function called by Dig
// and is passed to a Callback function registered with [WithCallback].
type CallbackInfo struct {

	// Name is the name of the function in the format:
	// <package_name>.<function_name>
	Name string

	// Error contains the error returned by the [Callback]'s associated
	// function, if there was one.
	Error error
}

type Callback func(CallbackInfo)

// WithCallback allows registering a callback function with Dig
// to be called whenever a provided function or decorator of a container
// is called successfully. [Callback] is a simple interface containing
// the function to be called as a callback.
func WithCallback(callback Callback) withCallbackOption {
	return withCallbackOption{
		callback: callback,
	}
}

type withCallbackOption struct {
	callback Callback
}

var _ ProvideOption = withCallbackOption{}

func (o withCallbackOption) applyProvideOption(po *provideOptions) {
	po.Callback = o.callback
}

var _ DecorateOption = withCallbackOption{}

func (o withCallbackOption) apply(do *decorateOptions) {
	do.Callback = o.callback
}
