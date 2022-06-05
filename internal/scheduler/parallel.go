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

// task is used by parallelScheduler to remember which function to
// call and which deferred to notify afterwards.
type task struct {
	fn func()
	d  *promise.Deferred
}

// Parallel processes enqueued work using a fixed-size worker pool.
// The pool is started and stopped during the call to flush.
type Parallel struct {
	concurrency int
	tasks       []task
}

var _ Scheduler = (*Parallel)(nil)

// NewParallel builds a new parallel scheduler that will use the specified
// number of goroutines to run tasks.
func NewParallel(concurrency int) *Parallel {
	return &Parallel{concurrency: concurrency}
}

// Schedule enqueues a task and returns an unresolved deferred.
// It will be resolved during flush.
func (p *Parallel) Schedule(fn func()) *promise.Deferred {
	d := new(promise.Deferred)
	p.tasks = append(p.tasks, task{fn, d})
	return d
}

// Flush processes enqueued work.
// concurrency controls how many executor goroutines are started and thus the
// maximum number of calls that may proceed in parallel.
// The real level of concurrency may be lower for CPU-heavy workloads if Go
// doesn't assign these goroutines to OS threads.
func (p *Parallel) Flush() {
	inFlight := 0
	taskChan := make(chan task)
	resultChan := make(chan *promise.Deferred)

	for n := 0; n < p.concurrency; n++ {
		go func() {
			for t := range taskChan {
				t.fn()
				resultChan <- t.d
			}
		}()
	}

	for inFlight > 0 || len(p.tasks) > 0 {
		var t task
		var outChan chan<- task

		if len(p.tasks) > 0 {
			t = p.tasks[len(p.tasks)-1]
			outChan = taskChan
		}

		select {
		case outChan <- t:
			inFlight++
			p.tasks = p.tasks[:len(p.tasks)-1]
		case d := <-resultChan:
			inFlight--
			d.Resolve(nil)
		}
	}

	close(taskChan)
	close(resultChan)
	p.tasks = nil
}
