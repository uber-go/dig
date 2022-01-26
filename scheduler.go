package dig

// A scheduler queues work during resolution of params.
// constructorNode uses it to call its constructor function.
// This may happen in parallel with other calls (parallelScheduler) or
// synchronously, right when enqueued.
//
// Work is enqueued when building a paramList, but the user of scheduler
// must call flush() for asynchronous calls to proceed after the top-level
// paramList.BuildList() is called.
type scheduler interface {
	// schedule will call a the supplied func. The deferred will resolve
	// after the func is called. The func may be called before schedule
	// returns. The deferred will be resolved on the "main" goroutine, so
	// it's safe to mutate containerStore during its resolution. It will
	// always be resolved with a nil error.
	schedule(func()) *deferred

	// flush processes enqueued work. This may in turn enqueue more work;
	// flush will keep processing the work until it's empty. After flush is
	// called, every deferred returned from schedule will have been resolved.
	// Asynchronous deferred values returned from schedule are resolved on the
	// same goroutine as the one calling this method.
	//
	// The scheduler is ready for re-use after flush is called.
	flush()
}

// synchronousScheduler is stateless and calls funcs as soon as they are schedule. It produces
// the exact same results as the code before deferred was introduced.
type synchronousScheduler struct{}

// schedule calls func and returns an already-resolved deferred.
func (s synchronousScheduler) schedule(fn func()) *deferred {
	fn()
	return &alreadyResolved
}

// flush does nothing. All returned deferred values are already resolved.
func (s synchronousScheduler) flush() {

}

// task is used by parallelScheduler to remember which function to
// call and which deferred to notify afterwards.
type task struct {
	fn func()
	d  *deferred
}

// parallelScheduler processes enqueued work using a fixed-size worker pool.
// The pool is started and stopped during the call to flush.
type parallelScheduler struct {
	concurrency int
	tasks       []task
}

// schedule enqueues a task and returns an unresolved deferred. It will be
// resolved during flush.
func (p *parallelScheduler) schedule(fn func()) *deferred {
	d := &deferred{}
	p.tasks = append(p.tasks, task{fn, d})
	return d
}

// flush processes enqueued work. concurrency controls how many executor
// goroutines are started and thus the maximum number of calls that may
// proceed in parallel. The real level of concurrency may be lower for
// CPU-heavy workloads if Go doesn't assign these goroutines to OS threads.
func (p *parallelScheduler) flush() {
	inFlight := 0
	taskChan := make(chan task)
	resultChan := make(chan *deferred)

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
			p.tasks = p.tasks[0 : len(p.tasks)-1]
		case d := <-resultChan:
			inFlight--
			d.resolve(nil)
		}
	}

	close(taskChan)
	close(resultChan)

	p.tasks = nil
}

// unboundedScheduler starts a goroutine per task. Maximum concurrency is
// controlled by Go's allocation of OS threads to goroutines.
type unboundedScheduler struct {
	tasks []task
}

// schedule enqueues a task and returns an unresolved deferred. It will be
// resolved during flush.
func (p *unboundedScheduler) schedule(fn func()) *deferred {
	d := &deferred{}
	p.tasks = append(p.tasks, task{fn, d})
	return d
}

// flush processes enqueued work with unlimited concurrency. The actual limit
// is up to Go's allocation of OS resources to goroutines.
func (p *unboundedScheduler) flush() {
	inFlight := 0
	resultChan := make(chan *deferred)

	for inFlight > 0 || len(p.tasks) > 0 {
		if len(p.tasks) > 0 {
			t := p.tasks[len(p.tasks)-1]
			p.tasks = p.tasks[0 : len(p.tasks)-1]

			go func() {
				t.fn()
				resultChan <- t.d
			}()

			inFlight++
			continue
		}

		select {
		case d := <-resultChan:
			inFlight--
			d.resolve(nil)
		}
	}

	close(resultChan)

	p.tasks = nil
}
