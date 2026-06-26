// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "sync/atomic"

// EMMState is the EPS Mobility Management state of a UE (TS 24.301, TS 23.401).
// The ECM state is derived from whether the UE holds an S1-connection (ue.s1).
type EMMState uint8

const (
	EMMDeregistered EMMState = iota
	EMMRegistered
)

// emmStateAtomic holds a UE's EMMState; see the MME concurrency model.
type emmStateAtomic struct{ v atomic.Uint32 }

func (a *emmStateAtomic) load() EMMState   { return EMMState(a.v.Load()) }
func (a *emmStateAtomic) store(s EMMState) { a.v.Store(uint32(s)) }
