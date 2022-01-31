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

package scheduler

import "go.uber.org/dig/internal/promise"

// A scheduler queues work during resolution of params.
// constructorNode uses it to call its constructor function.
// This may happen in parallel with other calls (parallelScheduler) or
// synchronously, right when enqueued.
//
// Work is enqueued when building a paramList, but the user of scheduler
// must call flush() for asynchronous calls to proceed after the top-level
// paramList.BuildList() is called.
type Scheduler interface {
	// schedule will call a the supplied func. The deferred will resolve
	// after the func is called. The func may be called before schedule
	// returns. The deferred will be resolved on the "main" goroutine, so
	// it's safe to mutate containerStore during its resolution. It will
	// always be resolved with a nil error.
	Schedule(func()) *promise.Deferred

	// flush processes enqueued work. This may in turn enqueue more work;
	// flush will keep processing the work until it's empty. After flush is
	// called, every deferred returned from schedule will have been resolved.
	// Asynchronous deferred values returned from schedule are resolved on the
	// same goroutine as the one calling this method.
	//
	// The scheduler is ready for re-use after flush is called.
	Flush()
}

// Synchronous is a stateless synchronous scheduler.
// It invokes functions as soon as they are scheduled.
// This is equivalent to not using a concurrent scheduler at all.
var Synchronous = synchronous{}

// synchronous is stateless and calls funcs as soon as they are schedule. It produces
// the exact same results as the code before deferred was introduced.
type synchronous struct{}

var _ Scheduler = synchronous{}

// schedule calls func and returns an already-resolved deferred.
func (s synchronous) Schedule(fn func()) *promise.Deferred {
	fn()
	return promise.Done
}

// flush does nothing. All returned deferred values are already resolved.
func (s synchronous) Flush() {}
