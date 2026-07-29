// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

var maxMsbValue = big.NewInt(0x003FFFFF)

// ErrTMSIPoolExhausted is returned when Allocate cannot find a free TMSI within
// maxAllocateAttempts.
var ErrTMSIPoolExhausted = errors.New("TMSI pool exhausted")

// maxAllocateAttempts bounds the collision-retry loop so a saturated pool fails
// deterministically instead of spinning. Far above any realistic occupancy of the
// ~2^32 TMSI space, so it only fires on true exhaustion.
const maxAllocateAttempts = 1 << 16

// GUTI5G is a 5G Globally Unique Temporary Identity (5G-GUTI).
type GUTI5G struct {
	mcc   string
	mnc   string
	Amfid string
	Tmsi  TMSI
}

var InvalidGUTI5G GUTI5G = GUTI5G{Tmsi: InvalidTMSI}

func NewGUTI5G(mcc string, mnc string, amfid string, tmsi TMSI) (GUTI5G, error) {
	if len(mcc) != 3 {
		return InvalidGUTI5G, fmt.Errorf("invalid mcc: %s", mcc)
	}

	_, err := strconv.ParseUint(mcc, 10, 16)
	if err != nil {
		return InvalidGUTI5G, fmt.Errorf("invalid mcc: %s", mcc)
	}

	if len(mnc) < 2 || len(mnc) > 3 {
		return InvalidGUTI5G, fmt.Errorf("invalid mnc: %s", mnc)
	}

	_, err = strconv.ParseUint(mnc, 10, 16)
	if err != nil {
		return InvalidGUTI5G, fmt.Errorf("invalid mnc: %s", mnc)
	}

	if len(amfid) != 6 {
		return InvalidGUTI5G, fmt.Errorf("invalid amfid: %s", amfid)
	}

	_, err = hex.DecodeString(amfid)
	if err != nil {
		return InvalidGUTI5G, fmt.Errorf("invalid amfid: %s", amfid)
	}

	if tmsi == InvalidTMSI {
		return InvalidGUTI5G, fmt.Errorf("invalid tmsi: %s", tmsi.String())
	}

	return GUTI5G{mcc: mcc, mnc: mnc, Amfid: strings.ToLower(amfid), Tmsi: tmsi}, nil
}

func NewGUTI5GFromBytes(buf []byte) (GUTI5G, error) {
	id, err := fgs.ParseMobileIdentity(buf)
	if err != nil {
		return InvalidGUTI5G, err
	}

	return NewGUTI5GFromNAS(id)
}

// NewGUTI5GFromNAS adopts a decoded 5G-GUTI 5GS mobile identity.
func NewGUTI5GFromNAS(id fgs.MobileIdentity) (GUTI5G, error) {
	if id.GUTI == nil {
		return InvalidGUTI5G, fmt.Errorf("mobile identity %s is not a 5G-GUTI", id.Type())
	}

	amf, err := fgs.AMFIdentifier{
		RegionID: id.GUTI.AMFRegionID, SetID: id.GUTI.AMFSetID, Pointer: id.GUTI.AMFPointer,
	}.MarshalBinary()
	if err != nil {
		return InvalidGUTI5G, err
	}

	tmsi, err := NewTMSI(binary.BigEndian.Uint32(id.GUTI.TMSI[:]))
	if err != nil {
		return InvalidGUTI5G, err
	}

	return GUTI5G{mcc: id.GUTI.PLMN.MCC, mnc: id.GUTI.PLMN.MNC, Amfid: hex.EncodeToString(amf), Tmsi: tmsi}, nil
}

func (g *GUTI5G) String() string {
	return fmt.Sprintf("%s%s%s%s", g.mcc, g.mnc, g.Amfid, &g.Tmsi)
}

// MobileIdentity renders the GUTI as a 5G-GUTI 5GS mobile identity
// (TS 24.501 §9.11.3.4). It is the inverse of NewGUTI5GFromNAS.
func (g GUTI5G) MobileIdentity() (fgs.MobileIdentity, error) {
	if len(g.mcc) != 3 || (len(g.mnc) != 2 && len(g.mnc) != 3) {
		return fgs.MobileIdentity{}, fmt.Errorf("invalid PLMN in GUTI: mcc %q mnc %q", g.mcc, g.mnc)
	}

	raw, err := hex.DecodeString(g.Amfid)
	if err != nil || len(raw) != 3 {
		return fgs.MobileIdentity{}, fmt.Errorf("invalid AMF ID %q in GUTI", g.Amfid)
	}

	amf, err := fgs.ParseAMFIdentifier(raw)
	if err != nil {
		return fgs.MobileIdentity{}, err
	}

	out := fgs.GUTI{
		PLMN:        nas.PLMN{MCC: g.mcc, MNC: g.mnc},
		AMFRegionID: amf.RegionID, AMFSetID: amf.SetID, AMFPointer: amf.Pointer,
	}
	binary.BigEndian.PutUint32(out.TMSI[:], g.Tmsi.Uint32())

	return fgs.GUTIIdentity(out), nil
}

// TMSI is a 5G Temporary Mobile Subscriber Identity.
type TMSI struct {
	tmsi uint32
}

var InvalidTMSI TMSI = TMSI{math.MaxUint32}

// NewTMSI wraps v as a TMSI. math.MaxUint32 is the reserved invalid sentinel and
// yields an error.
func NewTMSI(v uint32) (TMSI, error) {
	if v == math.MaxUint32 {
		return TMSI{v}, fmt.Errorf("invalid TMSI")
	}

	return TMSI{v}, nil
}

// String returns the TMSI as a hexadecimal string.
func (t TMSI) String() string {
	return fmt.Sprintf("%08x", t.tmsi)
}

func (t TMSI) Uint32() uint32 {
	return t.tmsi
}

// TmsiAllocator allocates and frees TMSIs. Generated TMSIs are round-robined over
// the 10 least significant bits to spread paging load (TS 23.501), and are
// otherwise random for privacy.
type TmsiAllocator struct {
	allocated map[TMSI]bool
	nextLsb   uint32

	sync.Mutex
}

func NewTMSIAllocator() *TmsiAllocator {
	ta := TmsiAllocator{
		allocated: make(map[TMSI]bool),
		nextLsb:   0,
	}

	return &ta
}

// Allocate returns a fresh unique TMSI, ctx.Err() if the context expires first, or
// ErrTMSIPoolExhausted if no free TMSI is found within maxAllocateAttempts.
func (ta *TmsiAllocator) Allocate(ctx context.Context) (TMSI, error) {
	for attempt := 0; attempt < maxAllocateAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return InvalidTMSI, ctx.Err()
		default:
		}

		msb, err := rand.Int(rand.Reader, maxMsbValue)
		if err != nil {
			continue
		}

		ta.Lock()

		lsb := ta.nextLsb

		t, err := NewTMSI(uint32(msb.Int64()<<10) + lsb)
		if err != nil {
			ta.Unlock()
			continue
		}

		if !ta.tryAllocate(t) {
			ta.Unlock()
			continue
		}

		ta.Unlock()

		return t, nil
	}

	return InvalidTMSI, ErrTMSIPoolExhausted
}

// Free returns the TMSI to the pool.
func (ta *TmsiAllocator) Free(t TMSI) {
	ta.Lock()
	defer ta.Unlock()

	delete(ta.allocated, t)
}

// IsAllocated reports whether t is currently held by the allocator.
func (ta *TmsiAllocator) IsAllocated(t TMSI) bool {
	ta.Lock()
	defer ta.Unlock()

	return ta.allocated[t]
}

func (ta *TmsiAllocator) tryAllocate(t TMSI) bool {
	if ta.allocated[t] {
		return false
	}

	ta.allocated[t] = true
	ta.nextLsb++

	return true
}
