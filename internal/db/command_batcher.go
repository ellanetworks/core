// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"encoding/json"
	"fmt"
	"sync"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

const (
	commandBatchMaxCommands = 256
	commandBatchMaxBytes    = 512 << 10
	commandQueueLength      = 4096
	commandMaxInFlight      = 4
	commandFraming          = 64
)

var (
	errBatcherClosed  = fmt.Errorf("%w: the database is closing", ErrProposeTimeout)
	errCommandsPaused = fmt.Errorf("%w: a restore is in progress", ErrProposeTimeout)
	errWriteQueueFull = fmt.Errorf("%w: replicated write queue is full", ErrProposeTimeout)
)

type commandRequest struct {
	cmd    wireCommand
	size   int
	result commandResult
	index  uint64
	err    error
	done   chan struct{}
}

type commandBatcher struct {
	db        *Database
	queue     chan *commandRequest
	completed chan struct{}
	stop      chan struct{}
	stopped   chan struct{}
	mu        sync.RWMutex
	closed    bool
	last      int64
	inFlight  sync.WaitGroup
}

func (db *Database) commandBatcher() (*commandBatcher, error) {
	db.batcherMu.Lock()
	defer db.batcherMu.Unlock()

	if db.commandsPaused {
		return nil, errCommandsPaused
	}

	if db.batcher == nil {
		db.batcher = &commandBatcher{
			db:        db,
			queue:     make(chan *commandRequest, commandQueueLength),
			completed: make(chan struct{}, commandMaxInFlight),
			stop:      make(chan struct{}),
			stopped:   make(chan struct{}),
		}

		go db.batcher.run()
	}

	return db.batcher, nil
}

func (db *Database) pauseCommands() func() {
	db.batcherMu.Lock()
	db.commandsPaused = true
	b := db.batcher
	db.batcher = nil
	db.batcherMu.Unlock()

	if b != nil {
		b.close()
	}

	return func() {
		db.batcherMu.Lock()
		db.commandsPaused = false
		db.batcherMu.Unlock()
	}
}

func (b *commandBatcher) propose(cmd wireCommand) (commandResult, uint64, error) {
	req := &commandRequest{
		cmd:  cmd,
		size: len(cmd.Op) + len(cmd.Payload) + commandFraming,
		done: make(chan struct{}),
	}

	if err := b.enqueue(req); err != nil {
		return commandResult{}, 0, err
	}

	<-req.done

	if req.err != nil {
		return commandResult{}, 0, classifyProposeErr(req.err)
	}

	return req.result, req.index, nil
}

func (b *commandBatcher) enqueue(req *commandRequest) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return errBatcherClosed
	}

	select {
	case b.queue <- req:
		return nil
	default:
		return errWriteQueueFull
	}
}

func (b *commandBatcher) close() {
	b.mu.Lock()
	closed := b.closed
	b.closed = true
	b.mu.Unlock()

	if closed {
		return
	}

	close(b.stop)
	<-b.stopped

	for {
		select {
		case r := <-b.queue:
			finishCommands([]*commandRequest{r}, nil, 0, errBatcherClosed)
		default:
			b.inFlight.Wait()

			return
		}
	}
}

func (b *commandBatcher) run() {
	defer close(b.stopped)

	var (
		batch    []*commandRequest
		size     int
		carry    *commandRequest
		inFlight int
	)

	add := func(r *commandRequest) {
		if len(batch) > 0 && size+r.size > commandBatchMaxBytes {
			carry = r

			return
		}

		batch = append(batch, r)
		size += r.size
	}

	shutdown := func() {
		if carry != nil {
			batch = append(batch, carry)
		}

		finishCommands(batch, nil, 0, errBatcherClosed)
	}

	for {
		if len(batch) == 0 && carry != nil {
			r := carry
			carry = nil

			add(r)
		}

		if len(batch) == 0 {
			select {
			case r := <-b.queue:
				add(r)
			case <-b.completed:
				inFlight--

				continue
			case <-b.stop:
				return
			}
		}

	fill:
		for carry == nil && len(batch) < commandBatchMaxCommands {
			select {
			case r := <-b.queue:
				add(r)
			default:
				break fill
			}
		}

		full := carry != nil || len(batch) >= commandBatchMaxCommands

		if inFlight == 0 || (full && inFlight < commandMaxInFlight) {
			inFlight++

			b.send(batch)

			batch, size = nil, 0

			continue
		}

		if full {
			select {
			case <-b.completed:
				inFlight--
			case <-b.stop:
				shutdown()

				return
			}

			continue
		}

		select {
		case <-b.completed:
			inFlight--
		case r := <-b.queue:
			add(r)
		case <-b.stop:
			shutdown()

			return
		}
	}
}

func (b *commandBatcher) send(batch []*commandRequest) {
	b.last = b.db.nextLeaderTime(b.last)

	cmds := make([]wireCommand, len(batch))
	for i, r := range batch {
		cmds[i] = r.cmd
	}

	encoded := commandBatch{LeaderTime: b.last, Commands: cmds}

	var wait func() (*ellaraft.ProposeResult, error)

	if b.db.raftManager == nil {
		body, err := json.Marshal(encoded)
		if err != nil {
			finishCommands(batch, nil, 0, fmt.Errorf("encode command batch: %w", err))

			b.completed <- struct{}{}

			return
		}

		res, err := b.db.local.apply(b.db, body)
		wait = func() (*ellaraft.ProposeResult, error) { return res, err }
	} else {
		data, err := encodeCommandBatch(encoded)
		if err != nil {
			finishCommands(batch, nil, 0, err)

			b.completed <- struct{}{}

			return
		}

		wait = b.db.raftManager.StartApply(data, b.db.proposeTimeout)
	}

	b.inFlight.Add(1)

	go func() {
		defer b.inFlight.Done()
		defer func() { b.completed <- struct{}{} }()

		res, err := wait()
		if err != nil {
			finishCommands(batch, nil, 0, err)

			return
		}

		results, ok := res.Value.([]commandResult)
		if !ok || len(results) != len(batch) {
			finishCommands(batch, nil, 0, fmt.Errorf("unexpected apply response %T", res.Value))

			return
		}

		finishCommands(batch, results, res.Index, nil)
	}()
}

func finishCommands(batch []*commandRequest, results []commandResult, index uint64, err error) {
	for i, r := range batch {
		if err != nil {
			r.err = err
		} else {
			r.result = results[i]
			r.index = index
		}

		close(r.done)
	}
}
