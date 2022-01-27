package dig

type observer func(error)

// A deferred is an observable future result that may fail. Its zero value is unresolved and has no observers. It can
// be resolved once, at which point every observer will be called.
type deferred struct {
	observers []observer
	settled   bool
	err       error
}

// alreadyResolved is a deferred that has already been resolved with a nil error.
var alreadyResolved = deferred{settled: true}

// failedDeferred returns a deferred that is resolved with the given error.
func failedDeferred(err error) *deferred {
	return &deferred{settled: true, err: err}
}

// observe registers an observer to receive a callback when this deferred is resolved. It will be called at most one
// time. If this deferred is already resolved, the observer is called immediately, before observe returns.
func (d *deferred) observe(obs observer) {
	if d.settled {
		obs(d.err)
		return
	}

	d.observers = append(d.observers, obs)
}

// resolve sets the status of this deferred and notifies all observers if it's not already resolved.
func (d *deferred) resolve(err error) {
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

// then returns a new deferred that is either resolved with the same error as this deferred or the eventual result of
// the deferred returned by res.
func (d *deferred) then(res func() *deferred) *deferred {
	// Shortcut: if we're settled...
	if d.settled {
		if d.err == nil {
			// ...successfully, then return the other deferred
			return res()
		} else {
			// ...with an error, then return us
			return d
		}
	}

	d2 := new(deferred)
	d.observe(func(err error) {
		if err != nil {
			d2.resolve(err)
		} else {
			res().observe(d2.resolve)
		}
	})
	return d2
}

// catch maps any error from this deferred using the supplied function. The supplied function is only called if this
// deferred is resolved with an error. If the supplied function returns a nil error, the new deferred will resolve
// successfully.
func (d *deferred) catch(rej func(error) error) *deferred {
	d2 := new(deferred)
	d.observe(func(err error) {
		if err != nil {
			err = rej(err)
		}
		d2.resolve(err)
	})
	return d2
}

// whenAll returns a new deferred that resolves when all the supplied deferreds resolve. It resolves with the first
// error reported by any deferred, or nil if they all succeed.
func whenAll(others ...*deferred) *deferred {
	if len(others) == 0 {
		return &alreadyResolved
	}

	d := new(deferred)
	count := len(others)

	onResolved := func(err error) {
		if d.settled {
			return
		}

		if err != nil {
			d.resolve(err)
		}

		count--
		if count == 0 {
			d.resolve(nil)
		}
	}

	for _, other := range others {
		other.observe(onResolved)
	}

	return d
}
