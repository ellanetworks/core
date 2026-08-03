// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

// BuildPaging assembles the Paging message for a UE (TS 38.413 §9.2.4.1). The
// TAI list for paging is the UE's registration area. Mirrors the MME's
// buildPaging.
func (amf *AMF) BuildPaging(guami *models.Guami, ue *UeContext) (*ngap.Paging, error) {
	guti, err := amf.PagingGuti(guami, ue)
	if err != nil {
		return nil, fmt.Errorf("paging: build 5G-GUTI: %w", err)
	}

	taiList, err := areaToNGAPTAIs(ue.RegistrationArea)
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	stmsi, err := fiveGSTMSIFor(guti)
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	return &ngap.Paging{
		FiveGSTMSI:       stmsi,
		TAIListForPaging: taiList,
		// Replay the gNB-reported paging capability so it can apply paging
		// optimisations (TS 38.413 §9.3.1.68); omitted when none was reported.
		UERadioCapabilityForPaging: radioCapabilityForPaging(ue.RadioCapabilityForPaging),
	}, nil
}

// fiveGSTMSIFor splits the paging 5G-GUTI into the 5G-S-TMSI the gNB matches
// against: the AMF Set ID and Pointer that addressed the UE, plus its 5G-TMSI
// (TS 23.003 §2.10).
func fiveGSTMSIFor(guti etsi.GUTI5G) (*ngap.FiveGSTMSI, error) {
	_, setID, pointer, err := util.AMFIDToNGAP(guti.Amfid)
	if err != nil {
		return nil, fmt.Errorf("split AMF id: %w", err)
	}

	tmsi, err := strconv.ParseUint(guti.Tmsi.String(), 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse 5G-TMSI: %w", err)
	}

	return &ngap.FiveGSTMSI{
		AMFSetID:   setID,
		AMFPointer: pointer,
		FiveGTMSI:  ngap.FiveGTMSI(tmsi),
	}, nil
}

// areaToNGAPTAIs converts the UE's registration area into the TAI List for
// Paging. Mirrors the MME's areaToS1APTAIs; the 5G TAC is three octets where
// LTE's is two (TS 38.413 §9.3.3.10).
func areaToNGAPTAIs(area []models.Tai) ([]ngap.TAI, error) {
	if len(area) == 0 {
		return nil, fmt.Errorf("empty registration area")
	}

	out := make([]ngap.TAI, 0, len(area))

	for _, t := range area {
		if t.PlmnID == nil {
			return nil, fmt.Errorf("registration-area TAI with no PLMN")
		}

		plmn, err := util.PLMNToNGAP(*t.PlmnID)
		if err != nil {
			return nil, fmt.Errorf("encode PLMN: %w", err)
		}

		tac, err := strconv.ParseUint(t.Tac, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("parse TAC %q: %w", t.Tac, err)
		}

		out = append(out, ngap.TAI{PLMNIdentity: plmn, TAC: ngap.TAC(tac)})
	}

	return out, nil
}

// radioCapabilityForPaging maps the stored capability onto the IE, returning nil
// when the gNB reported none so the optional IE is omitted.
func radioCapabilityForPaging(c *models.UERadioCapabilityForPaging) *ngap.UERadioCapabilityForPaging {
	if c == nil {
		return nil
	}

	out := &ngap.UERadioCapabilityForPaging{}

	if nr, err := hex.DecodeString(c.NR); err == nil && len(nr) > 0 {
		out.NR = ngap.Ptr(ngap.UERadioCapabilityForPagingOfNR(nr))
	}

	if eutra, err := hex.DecodeString(c.EUTRA); err == nil && len(eutra) > 0 {
		out.EUTRA = ngap.Ptr(ngap.UERadioCapabilityForPagingOfEUTRA(eutra))
	}

	if out.NR == nil && out.EUTRA == nil {
		return nil
	}

	return out
}
