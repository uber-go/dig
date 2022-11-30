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

// Unbounded starts a goroutine per task.
// Maximum concurrency is controlled by Go's allocation of OS threads to
// goroutines.
type Unbounded struct {
	tasks []task
}

var _ Scheduler = (*Unbounded)(nil)

// Schedule enqueues a task and returns an unresolved deferred.
// It will be resolved during flush.
func (p *Unbounded) Schedule(fn func()) *promise.Deferred {
	d := new(promise.Deferred)
	p.tasks = append(p.tasks, task{fn, d})
	return d
}

// Flush processes enqueued work with unlimited concurrency.
// The actual limit is up to Go's allocation of OS resources to goroutines.
func (p *Unbounded) Flush() {
	inFlight := 0
	resultChan := make(chan *promise.Deferred)

	for inFlight > 0 || len(p.tasks) > 0 {
		if len(p.tasks) > 0 {
			t := p.tasks[len(p.tasks)-1]
			p.tasks = p.tasks[:len(p.tasks)-1]
			go func() {
				t.fn()
				resultChan <- t.d
			}()
			inFlight++
			continue
		}
		d := <-resultChan
		inFlight--
		d.Resolve(nil)
	}
	close(resultChan)
	p.tasks = nil
}
