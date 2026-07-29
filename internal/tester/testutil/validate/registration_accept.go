// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package validate

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
)

type ExpectedSlice struct {
	Sst int32
	Sd  string
}

type RegistrationAcceptOpts struct {
	NASMsg         []byte
	UE             *ue.UE
	Sst            int32
	Sd             string
	Mcc            string
	Mnc            string
	ExpectedSlices []ExpectedSlice // If set, validates all allowed NSSAI entries
}

func RegistrationAccept(opts *RegistrationAcceptOpts) error {
	msg, err := testutil.ExpectNAS[*fgs.RegistrationAccept](opts.NASMsg)
	if err != nil {
		return err
	}

	if msg.RegistrationResult != 1 {
		return fmt.Errorf("registration result 5GS not the expected value")
	}

	if msg.GUTI == nil || msg.GUTI.GUTI == nil {
		return fmt.Errorf("REGISTRATION ACCEPT carries no 5G-GUTI")
	}

	guti := *msg.GUTI.GUTI

	prefix := opts.Mcc + opts.Mnc
	if !strings.HasPrefix(buildGUTI5G(guti), prefix) {
		return fmt.Errorf("GUTI5G PLMN not the expected value, got: %s, want prefix: %s", buildGUTI5G(guti), prefix)
	}

	if len(msg.AllowedNSSAI) == 0 {
		return fmt.Errorf("allowed NSSAI is missing")
	}

	parsedSlices := allowedNSSAI(msg.AllowedNSSAI)

	expected := opts.ExpectedSlices
	if len(expected) == 0 {
		expected = []ExpectedSlice{{Sst: opts.Sst, Sd: opts.Sd}}
	}

	if len(parsedSlices) != len(expected) {
		return fmt.Errorf("allowed NSSAI count mismatch: got %d, want %d", len(parsedSlices), len(expected))
	}

	for i, exp := range expected {
		if parsedSlices[i].Sst != exp.Sst {
			return fmt.Errorf("allowed NSSAI[%d] SST not the expected value, got: %d, want: %d", i, parsedSlices[i].Sst, exp.Sst)
		}

		if parsedSlices[i].Sd != exp.Sd {
			return fmt.Errorf("allowed NSSAI[%d] SD not the expected value, got: %s, want: %s", i, parsedSlices[i].Sd, exp.Sd)
		}
	}

	if msg.T3512 == nil {
		return fmt.Errorf("T3512 value is nil")
	}

	d, ok := msg.T3512.Duration()
	if !ok {
		return fmt.Errorf("T3512 carries no duration: %s", msg.T3512)
	}

	if timerInSeconds := int(d / time.Second); timerInSeconds != 3600 {
		return fmt.Errorf("T3512 timer in seconds not the expected value, got: %d, want: 3600", timerInSeconds)
	}

	return nil
}

func buildGUTI5G(g fgs.GUTI) string {
	amfID := nasToAmfId(g.AMFRegionID, g.AMFSetID, g.AMFPointer)
	tmsi := hex.EncodeToString(g.TMSI[:])

	return fmt.Sprintf("%s%s%s%s", g.PLMN.MCC, g.PLMN.MNC, amfID, tmsi)
}

func nasToAmfId(regionID uint8, setID uint16, pointer uint8) string {
	b, err := fgs.AMFIdentifier{RegionID: regionID, SetID: setID, Pointer: pointer}.MarshalBinary()
	if err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}

// parseAllowedNSSAI decodes the Allowed NSSAI IE value into its slices.
func allowedNSSAI(list fgs.NSSAI) []ExpectedSlice {
	out := make([]ExpectedSlice, 0, len(list))
	for _, s := range list {
		sd := ""
		if s.SD != nil {
			sd = fmt.Sprintf("%02x%02x%02x", s.SD[0], s.SD[1], s.SD[2])
		}

		out = append(out, ExpectedSlice{Sst: int32(s.SST), Sd: sd})
	}

	return out
}
