// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

// FivegmmContext is the per-registration 5GMM state for one AmfUe. It
// outlives NAS connection drops within a registration; it is replaced
// when an initial registration succeeds for an already-registered UE.
type FivegmmContext struct {
	parent *AmfUe

	ctx    context.Context
	cancel context.CancelFunc

	active atomic.Pointer[ActiveNasConnection]

	// NAS security context per TS 33.501.
	SecurityContextAvailable bool
	UESecurityCapability     *nasType.UESecurityCapability
	NgKsi                    models.NgKsi
	KnasInt                  [16]uint8
	KnasEnc                  [16]uint8
	Kgnb                     []uint8
	NH                       []uint8
	NCC                      uint8
	ULCount                  security.Count
	DLCount                  security.Count
	CipheringAlg             uint8
	IntegrityAlg             uint8
	Kamf                     string
	ABBA                     []uint8

	Ambr                       *models.Ambr
	AllowedNssai               []models.Snssai
	RegistrationArea           []models.Tai
	UeRadioCapability          string
	UeRadioCapabilityForPaging *models.UERadioCapabilityForPaging
	UESpecificDRX              uint8
	SmContextList              map[uint8]*SmContext

	T3502Value time.Duration
	T3512Value time.Duration

	MobileReachableTimer        *Timer
	ImplicitDeregistrationTimer *Timer
}

func newFivegmmContext(parent *AmfUe) *FivegmmContext {
	ctx, cancel := context.WithCancel(context.Background())

	return &FivegmmContext{
		parent:           parent,
		ctx:              ctx,
		cancel:           cancel,
		SmContextList:    make(map[uint8]*SmContext),
		RegistrationArea: make([]models.Tai, 0),
	}
}

func (fc *FivegmmContext) Ctx() context.Context {
	return fc.ctx
}

func (fc *FivegmmContext) Parent() *AmfUe {
	return fc.parent
}

func (fc *FivegmmContext) ActiveConnection() *ActiveNasConnection {
	return fc.active.Load()
}

func (fc *FivegmmContext) close() {
	if conn := fc.active.Swap(nil); conn != nil {
		conn.stopTimers()
		conn.cancel()
	}

	fc.stopIdleTimers()
	fc.cancel()
}

// stopIdleTimers stops the mobile reachable and implicit deregistration
// timers that fire while the UE is Registered-but-idle.
func (fc *FivegmmContext) stopIdleTimers() {
	if fc.MobileReachableTimer != nil {
		fc.MobileReachableTimer.Stop()
		fc.MobileReachableTimer = nil
	}

	if fc.ImplicitDeregistrationTimer != nil {
		fc.ImplicitDeregistrationTimer.Stop()
		fc.ImplicitDeregistrationTimer = nil
	}
}

func NewFivegmmContextForTest(parent *AmfUe) *FivegmmContext {
	return newFivegmmContext(parent)
}
