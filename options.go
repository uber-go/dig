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

// Option configures a Container. It's included for future functionality;
// currently, there are no concrete implementations.
type Option interface {
	apply(*Container)
}

type optionFunc func(*Container)

func (f optionFunc) apply(c *Container) {
	f(c)
}

// WithFrameSkipper option overrides the default FrameSkipper function to allow
// for a more fine-grained configuration on which stack frames should be ignored
// when considering the true caller.
//
// This is extremely helpful when the dig functions are wrapped, or used
// as part of another framework, such as go.uber.org/fx.
func WithFrameSkipper(s FrameSkipper) Option {
	return optionFunc(func(c *Container) {
		c.skipper = s
	})
}

// A ProvideOption modifies the default behavior of Provide. It's included for
// future functionality; currently, there are no concrete implementations.
type ProvideOption interface {
	apply(*provider)
}

// ProvideHook is a function to be executed as a result of Provide.
type ProvideHook func(ProvideEvent)

type provideOptionFunc func(*provider)

func (f provideOptionFunc) apply(p *provider) {
	f(p)
}

// WithProvideHook allows to provide a custom hook for a given Provide execution
func WithProvideHook(ph ProvideHook) ProvideOption {
	return provideOptionFunc(func(p *provider) {
		p.hook = ph
	})
}

// An InvokeOption modifies the default behavior of Invoke. It's included for
// future functionality; currently, there are no concrete implementations.
type InvokeOption interface {
	unimplemented()
}
