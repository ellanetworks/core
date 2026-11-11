// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package serial

import (
	"sync"
)

type Lane struct {
	mu    sync.Mutex
	cond  *sync.Cond
	busy  bool
	park  chan struct{}
	queue []func()
}

func (l *Lane) Run(f func()) {
	l.mu.Lock()

	if l.busy {
		l.queue = append(l.queue, f)
		l.mu.Unlock()

		return
	}

	l.busy = true
	l.mu.Unlock()

	l.loop(f)
}

func (l *Lane) loop(f func()) {
	for {
		f()

		l.mu.Lock()

		if p := l.park; p != nil {
			l.park = nil
			l.mu.Unlock()
			close(p)

			return
		}

		if len(l.queue) == 0 {
			l.busy = false
			l.idleLocked()
			l.mu.Unlock()

			return
		}

		f = l.queue[0]
		l.queue[0] = nil
		l.queue = l.queue[1:]
		l.mu.Unlock()
	}
}

func (l *Lane) Await(job, resume func()) {
	l.mu.Lock()

	switch {
	case !l.busy:
		l.busy = true
		l.mu.Unlock()

		go func() {
			job()
			l.loop(resume)
		}()
	case l.park == nil:
		p := make(chan struct{})
		l.park = p
		l.mu.Unlock()

		go func() {
			job()
			<-p
			l.loop(resume)
		}()
	default:
		l.mu.Unlock()
		job()
		resume()
	}
}

func (l *Lane) idleLocked() {
	if l.cond != nil {
		l.cond.Broadcast()
	}
}

func (l *Lane) Settle() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cond == nil {
		l.cond = sync.NewCond(&l.mu)
	}

	for l.busy {
		l.cond.Wait()
	}
}

type KeyedQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	jobs     map[string][]func()
	inFlight int
}

func (q *KeyedQueue) Go(key string, job func()) {
	q.mu.Lock()

	if q.jobs == nil {
		q.jobs = map[string][]func(){}
	}

	pending, running := q.jobs[key]
	q.jobs[key] = append(pending, job)
	q.inFlight++
	q.mu.Unlock()

	if !running {
		go q.work(key)
	}
}

func (q *KeyedQueue) Do(key string, job func()) {
	done := make(chan struct{})

	q.Go(key, func() {
		defer close(done)

		job()
	})

	<-done
}

func (q *KeyedQueue) work(key string) {
	for {
		q.mu.Lock()

		pending := q.jobs[key]
		if len(pending) == 0 {
			delete(q.jobs, key)
			q.mu.Unlock()

			return
		}

		job := pending[0]
		pending[0] = nil
		q.jobs[key] = pending[1:]
		q.mu.Unlock()

		job()

		q.mu.Lock()

		q.inFlight--
		if q.inFlight == 0 && q.cond != nil {
			q.cond.Broadcast()
		}

		q.mu.Unlock()
	}
}

func (q *KeyedQueue) Settle() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.cond == nil {
		q.cond = sync.NewCond(&q.mu)
	}

	for q.inFlight > 0 {
		q.cond.Wait()
	}
}
