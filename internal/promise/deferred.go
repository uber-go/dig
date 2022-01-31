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

package promise

type Observer func(error)

// Deferred is an observable future result that may fail.
// Its zero value is unresolved and has no observers.
// It can be resolved once, at which point every observer will be called.
type Deferred struct {
	observers []Observer
	settled   bool
	err       error
}

// Resolved reports whether this Deferred has resolved,
// and if so, with what error.
//
// err is undefined if the Deferred has not yet resolved.
func (d *Deferred) Resolved() (resolved bool, err error) {
	return d.settled, d.err
}

// Done is a Deferred that has already been resolved with a nil error.
var Done = &Deferred{settled: true}

// Fail returns a Deferred that has resolved with the given error.
func Fail(err error) *Deferred {
	return &Deferred{settled: true, err: err}
}

// Observe registers an observer to receive a callback when this deferred is
// resolved.
// It will be called at most one time.
// If this deferred is already resolved, the observer is called immediately,
// before Observe returns.
func (d *Deferred) Observe(obs Observer) {
	if d.settled {
		obs(d.err)
	} else {
		d.observers = append(d.observers, obs)
	}
}

// Resolve sets the status of this deferred and notifies all observers.
// This is a no-op if the Deferred has already resolved.
func (d *Deferred) Resolve(err error) {
	if d.settled {
		return
	}

	d.settled = true
	d.err = err
	for _, obs := range d.observers {
		obs(err)
	}
	d.observers = nil
}

// Then returns a new Deferred that resolves with the same error as this
// Deferred or the eventual result of the Deferred returned by res.
func (d *Deferred) Then(res func() *Deferred) *Deferred {
	// Shortcut: if we're settled...
	if d.settled {
		if d.err == nil {
			// ...successfully, then return the other deferred
			return res()
		}

		// ...with an error, then return us
		return d
	}

	d2 := new(Deferred)
	d.Observe(func(err error) {
		if err != nil {
			d2.Resolve(err)
		} else {
			res().Observe(d2.Resolve)
		}
	})
	return d2
}

// Catch maps any error from this deferred using the supplied function.
// The supplied function is only called if this deferred is resolved with an
// error.
// If the supplied function returns a nil error, the new deferred will resolve
// successfully.
func (d *Deferred) Catch(rej func(error) error) *Deferred {
	d2 := new(Deferred)
	d.Observe(func(err error) {
		if err != nil {
			err = rej(err)
		}
		d2.Resolve(err)
	})
	return d2
}

// WhenAll returns a new Deferred that resolves when all the supplied deferreds
// resolve.
// It resolves with the first error reported by any deferred, or nil if they
// all succeed.
func WhenAll(others ...*Deferred) *Deferred {
	if len(others) == 0 {
		return Done
	}

	d := new(Deferred)
	count := len(others)

	onResolved := func(err error) {
		if d.settled {
			return
		}

		if err != nil {
			d.Resolve(err)
		}

		count--
		if count == 0 {
			d.Resolve(nil)
		}
	}

	for _, other := range others {
		other.Observe(onResolved)
	}

	return d
}
