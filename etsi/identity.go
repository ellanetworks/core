package etsi

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

var maxMsbValue = big.NewInt(0x003FFFFF)

// GUTI represents a 5G Globally Unique Temporary Identity,
// as defined by 3GPP.
type GUTI struct {
	mcc   string
	mnc   string
	Amfid string
	Tmsi  TMSI
}

var InvalidGUTI GUTI = GUTI{Tmsi: InvalidTMSI}

func NewGUTI(mcc string, mnc string, amfid string, tmsi TMSI) (GUTI, error) {
	if len(mcc) != 3 {
		return InvalidGUTI, fmt.Errorf("invalid mcc: %s", mcc)
	}

	_, err := strconv.ParseUint(mcc, 10, 16)
	if err != nil {
		return InvalidGUTI, fmt.Errorf("invalid mcc: %s", mcc)
	}

	if len(mnc) < 2 || len(mnc) > 3 {
		return InvalidGUTI, fmt.Errorf("invalid mnc: %s", mnc)
	}

	_, err = strconv.ParseUint(mnc, 10, 16)
	if err != nil {
		return InvalidGUTI, fmt.Errorf("invalid mnc: %s", mnc)
	}

	if len(amfid) != 6 {
		return InvalidGUTI, fmt.Errorf("invalid amfid: %s", amfid)
	}

	_, err = hex.DecodeString(amfid)
	if err != nil {
		return InvalidGUTI, fmt.Errorf("invalid amfid: %s", amfid)
	}

	if tmsi == InvalidTMSI {
		return InvalidGUTI, fmt.Errorf("invalid tmsi: %s", tmsi.String())
	}

	return GUTI{mcc: mcc, mnc: mnc, Amfid: strings.ToLower(amfid), Tmsi: tmsi}, nil
}

func NewGUTIFromBytes(buf []byte) (GUTI, error) {
	if len(buf) != 11 {
		return InvalidGUTI, fmt.Errorf("invalid GUTI length")
	}

	mcc, mnc, err := plmnIDToMccMncString(buf[1:4])
	if err != nil {
		return InvalidGUTI, fmt.Errorf("invalid PLMN: %v", err)
	}

	amfID := hex.EncodeToString(buf[4:7])
	tmsi5G := binary.BigEndian.Uint32(buf[7:])

	tmsi, err := NewTMSI(tmsi5G)
	if err != nil {
		return InvalidGUTI, err
	}

	return GUTI{mcc: mcc, mnc: mnc, Amfid: amfID, Tmsi: tmsi}, nil
}

func (g *GUTI) String() string {
	return fmt.Sprintf("%s%s%s%s", g.mcc, g.mnc, g.Amfid, &g.Tmsi)
}

// TMSI represents a 5G Temporary Mobile Subscriber's Identity,
// as define by 3GPP.
type TMSI struct {
	tmsi uint32
}

var InvalidTMSI TMSI = TMSI{math.MaxUint32}

// NewTMSI creates a TMSI instance from a uint32 value.
// It returns an error if the resulting TMSI is invalid.
func NewTMSI(v uint32) (TMSI, error) {
	if v == math.MaxUint32 {
		return TMSI{v}, fmt.Errorf("invalid TMSI")
	}

	return TMSI{v}, nil
}

// String returns the TMSI has an hexadecimal string
func (t *TMSI) String() string {
	return fmt.Sprintf("%08x", t.tmsi)
}

// TmsiAllocator allocates and frees TMSI. It keeps
// track internally of allocated TMSI. Generated TMSIs
// are round-robined over the 10 least significant bits to spread
// paging load, as define in TS 23.501. The TMSIs are
// otherwise allocated randomly to ensure privacy.
type TmsiAllocator struct {
	allocated map[TMSI]bool
	nextLsb   uint32

	sync.Mutex
}

// NewTMSIAllocator returns a new allocator, that will allocate unique TMSI
// on demand.
func NewTMSIAllocator() *TmsiAllocator {
	ta := TmsiAllocator{
		allocated: make(map[TMSI]bool),
		nextLsb:   0,
	}

	return &ta
}

// Allocate returns a valid TMSI, or an error if the provided context
// expires.
func (ta *TmsiAllocator) Allocate(ctx context.Context) (TMSI, error) {
	for {
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
}

// Free returns the TMSI to the pool, allowing it to be reallocated.
func (ta *TmsiAllocator) Free(t TMSI) {
	ta.Lock()
	defer ta.Unlock()

	delete(ta.allocated, t)
}

func (ta *TmsiAllocator) tryAllocate(t TMSI) bool {
	if ta.allocated[t] {
		return false
	}

	ta.allocated[t] = true
	ta.nextLsb++

	return true
}

func plmnIDToMccMncString(buf []byte) (mcc string, mnc string, err error) {
	mccDigit1 := buf[0] & 0x0f
	mccDigit2 := (buf[0] & 0xf0) >> 4
	mccDigit3 := (buf[1] & 0x0f)

	mncDigit1 := (buf[2] & 0x0f)
	mncDigit2 := (buf[2] & 0xf0) >> 4
	mncDigit3 := (buf[1] & 0xf0) >> 4

	if mccDigit1 > 9 || mccDigit2 > 9 || mccDigit3 > 9 {
		return "", "", fmt.Errorf("invalid mcc")
	}

	// Last digit of MNC is set to `f` if MNC is only 2 digits
	if mncDigit3 > 9 && mncDigit3 != 15 {
		return "", "", fmt.Errorf("invalid mnc")
	}

	if mncDigit1 > 9 || mncDigit2 > 9 {
		return "", "", fmt.Errorf("invalid mnc")
	}

	tmpBytes := []byte{(mccDigit1 << 4) | mccDigit2, (mccDigit3 << 4) | mncDigit1, (mncDigit2 << 4) | mncDigit3}

	plmnID := hex.EncodeToString(tmpBytes)
	if plmnID[5] == 'f' {
		plmnID = plmnID[:5] // get plmnID[0~4]
	}

	return plmnID[:3], plmnID[3:], nil
}
