// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package radioreg

import "time"

type Record[K comparable] interface {
	comparable

	IDKey() (K, bool)
	DisconnectedAt() time.Time
	SetDisconnectedAt(time.Time)
}

type Registry[C comparable, K comparable, R Record[K]] struct {
	ByConn map[C]R

	byID map[K]R

	ttl time.Duration
	max int
	now func() time.Time
}

func New[C comparable, K comparable, R Record[K]](ttl time.Duration, maxOffline int, now func() time.Time) *Registry[C, K, R] {
	return &Registry[C, K, R]{
		ByConn: make(map[C]R),
		byID:   make(map[K]R),
		ttl:    ttl,
		max:    maxOffline,
		now:    now,
	}
}

func Connected[K comparable, R Record[K]](r R) bool {
	return r.DisconnectedAt().IsZero()
}

func (reg *Registry[C, K, R]) Track(conn C, radio R) {
	reg.ByConn[conn] = radio
}

func (reg *Registry[C, K, R]) Radio(conn C) (R, bool) {
	radio, ok := reg.ByConn[conn]

	return radio, ok
}

func (reg *Registry[C, K, R]) Claim(key K, radio R) R {
	var none R

	prev, ok := reg.byID[key]

	reg.byID[key] = radio

	if !ok || prev == radio {
		return none
	}

	return prev
}

func (reg *Registry[C, K, R]) Unclaim(key K) {
	delete(reg.byID, key)
}

func (reg *Registry[C, K, R]) ClaimedBy(key K) (R, bool) {
	radio, ok := reg.byID[key]

	return radio, ok
}

func (reg *Registry[C, K, R]) FindConnected(key K) (R, bool) {
	var none R

	radio, ok := reg.byID[key]
	if !ok || !Connected[K](radio) {
		return none, false
	}

	return radio, true
}

func (reg *Registry[C, K, R]) Disconnect(conn C, radio R) {
	delete(reg.ByConn, conn)

	key, ok := radio.IDKey()
	if !ok {
		return
	}

	if held, ok := reg.byID[key]; !ok || held != radio {
		return
	}

	radio.SetDisconnectedAt(reg.now())

	reg.EvictOffline()
}

func (reg *Registry[C, K, R]) Connected() []R {
	out := make([]R, 0, len(reg.ByConn))
	for _, radio := range reg.ByConn {
		out = append(out, radio)
	}

	return out
}

func (reg *Registry[C, K, R]) CountConnected() int {
	return len(reg.ByConn)
}

func (reg *Registry[C, K, R]) Has(match func(R) bool) bool {
	reg.EvictOffline()

	if reg.HasConnected(match) {
		return true
	}

	for _, radio := range reg.byID {
		if !Connected[K](radio) && match(radio) {
			return true
		}
	}

	return false
}

func (reg *Registry[C, K, R]) HasConnected(match func(R) bool) bool {
	for _, radio := range reg.ByConn {
		if match(radio) {
			return true
		}
	}

	return false
}

func (reg *Registry[C, K, R]) All() []R {
	reg.EvictOffline()

	out := make([]R, 0, len(reg.ByConn))
	for _, radio := range reg.ByConn {
		out = append(out, radio)
	}

	for _, radio := range reg.byID {
		if !Connected[K](radio) {
			out = append(out, radio)
		}
	}

	return out
}

func (reg *Registry[C, K, R]) CountOffline() int {
	reg.EvictOffline()

	count := 0

	for _, radio := range reg.byID {
		if !Connected[K](radio) {
			count++
		}
	}

	return count
}

func (reg *Registry[C, K, R]) Forget(match func(R) bool) (online bool, forgotten int) {
	reg.EvictOffline()

	if reg.HasConnected(match) {
		return true, 0
	}

	for key, radio := range reg.byID {
		if !Connected[K](radio) && match(radio) {
			delete(reg.byID, key)

			forgotten++
		}
	}

	return false, forgotten
}

func (reg *Registry[C, K, R]) EvictOffline() {
	now := reg.now()
	offline := 0

	for key, radio := range reg.byID {
		if Connected[K](radio) {
			continue
		}

		if now.Sub(radio.DisconnectedAt()) >= reg.ttl {
			delete(reg.byID, key)

			continue
		}

		offline++
	}

	for ; offline > reg.max; offline-- {
		var (
			victim K
			oldest time.Time
			found  bool
		)

		for key, radio := range reg.byID {
			if Connected[K](radio) {
				continue
			}

			if !found || radio.DisconnectedAt().Before(oldest) {
				victim, oldest, found = key, radio.DisconnectedAt(), true
			}
		}

		if !found {
			return
		}

		delete(reg.byID, victim)
	}
}

func (reg *Registry[C, K, R]) SetRetention(ttl time.Duration, maxOffline int, now func() time.Time) {
	reg.ttl = ttl
	reg.max = maxOffline
	reg.now = now
}
