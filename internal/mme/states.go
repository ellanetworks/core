// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "sync/atomic"

// EMMState is the EPS Mobility Management state of a UE (TS 24.301,
// TS 23.401). It is orthogonal to ECMState (TS 23.401).
type EMMState uint8

const (
	EMMDeregistered EMMState = iota
	EMMRegistered
)

// ECMState is the EPS Connection Management state of a UE (TS 23.401).
// ECM-IDLE/ECM-CONNECTED correspond to the EMM-IDLE/EMM-CONNECTED modes of
// TS 24.301.
type ECMState uint8

const (
	ECMIdle ECMState = iota
	ECMConnected
)

// emmStateAtomic holds a UE's EMMState; see the MME concurrency model.
type emmStateAtomic struct{ v atomic.Uint32 }

func (a *emmStateAtomic) load() EMMState   { return EMMState(a.v.Load()) }
func (a *emmStateAtomic) store(s EMMState) { a.v.Store(uint32(s)) }

// ecmStateAtomic holds a UE's ECMState; see the MME concurrency model.
type ecmStateAtomic struct{ v atomic.Uint32 }

func (a *ecmStateAtomic) load() ECMState   { return ECMState(a.v.Load()) }
func (a *ecmStateAtomic) store(s ECMState) { a.v.Store(uint32(s)) }
