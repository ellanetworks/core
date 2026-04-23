package validate

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"

	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasType"
)

type ExpectedSlice struct {
	Sst int32
	Sd  string
}

type RegistrationAcceptOpts struct {
	NASMsg         *nas.Message
	UE             *ue.UE
	Sst            int32
	Sd             string
	Mcc            string
	Mnc            string
	ExpectedSlices []ExpectedSlice // If set, validates all allowed NSSAI entries
}

func RegistrationAccept(opts *RegistrationAcceptOpts) error {
	if opts.NASMsg == nil {
		return fmt.Errorf("NAS message is nil")
	}

	if opts.NASMsg.GmmMessage == nil {
		return fmt.Errorf("NAS message is not a GMM message")
	}

	if opts.NASMsg.GmmMessage.GetMessageType() != nas.MsgTypeRegistrationAccept {
		return fmt.Errorf("NAS message type is not Registration Accept (%d), got (%d)", nas.MsgTypeRegistrationAccept, opts.NASMsg.GmmMessage.GetMessageType())
	}

	if opts.NASMsg.RegistrationAccept == nil {
		return fmt.Errorf("NAS Registration Accept message is nil")
	}

	if reflect.ValueOf(opts.NASMsg.RegistrationAccept.ExtendedProtocolDiscriminator).IsZero() {
		return fmt.Errorf("extended protocol is missing")
	}

	if opts.NASMsg.RegistrationAccept.GetExtendedProtocolDiscriminator() != 126 {
		return fmt.Errorf("extended protocol not the expected value")
	}

	if opts.NASMsg.RegistrationAccept.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if opts.NASMsg.RegistrationAccept.GetSecurityHeaderType() != 0 {
		return fmt.Errorf("security header type not the expected value")
	}

	if reflect.ValueOf(opts.NASMsg.RegistrationAccept.RegistrationAcceptMessageIdentity).IsZero() {
		return fmt.Errorf("message type is missing")
	}

	if opts.NASMsg.RegistrationAcceptMessageIdentity.GetMessageType() != 66 {
		return fmt.Errorf("message type not the expected value")
	}

	if reflect.ValueOf(opts.NASMsg.RegistrationAccept.RegistrationResult5GS).IsZero() {
		return fmt.Errorf("registration result 5GS is missing")
	}

	if opts.NASMsg.GetRegistrationResultValue5GS() != 1 {
		return fmt.Errorf("registration result 5GS not the expected value")
	}

	if opts.NASMsg.RegistrationAccept.GUTI5G == nil {
		return fmt.Errorf("GUTI5G is nil")
	}

	guti5GStr := buildGUTI5G(*opts.NASMsg.RegistrationAccept.GUTI5G)

	prefix := opts.Mcc + opts.Mnc
	if !strings.HasPrefix(guti5GStr, prefix) {
		return fmt.Errorf("GUTI5G PLMN not the expected value, got: %s, want prefix: %s", guti5GStr, prefix)
	}

	snssaiBytes := opts.NASMsg.RegistrationAccept.AllowedNSSAI.GetSNSSAIValue()

	if len(snssaiBytes) == 0 {
		return fmt.Errorf("allowed NSSAI is missing")
	}

	parsedSlices, err := parseAllowedNSSAI(snssaiBytes)
	if err != nil {
		return fmt.Errorf("could not parse allowed NSSAI: %v", err)
	}

	// Build expected slice list from either ExpectedSlices or single Sst/Sd.
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

	if opts.NASMsg.T3512Value == nil {
		return fmt.Errorf("T3512 value is nil")
	}

	timerInSeconds := utils.NasToGPRSTimer3(opts.NASMsg.T3512Value.Octet)
	if timerInSeconds != 3600 {
		return fmt.Errorf("T3512 timer in seconds not the expected value, got: %d, want: 3600", timerInSeconds)
	}

	return nil
}

func buildGUTI5G(gutiNas nasType.GUTI5G) string {
	mcc1 := gutiNas.GetMCCDigit1()
	mcc2 := gutiNas.GetMCCDigit2()
	mcc3 := gutiNas.GetMCCDigit3()
	mnc1 := gutiNas.GetMNCDigit1()
	mnc2 := gutiNas.GetMNCDigit2()
	mnc3 := gutiNas.GetMNCDigit3()

	amfRegionID := gutiNas.GetAMFRegionID()
	amfSetID := gutiNas.GetAMFSetID()
	amfPointer := gutiNas.GetAMFPointer()
	amfID := nasToAmfId(amfRegionID, amfSetID, amfPointer)

	tmsi := hex.EncodeToString(gutiNas.Octet[7:11])

	if mnc3 == 0x0F {
		return fmt.Sprintf("%d%d%d%d%d%s%s", mcc1, mcc2, mcc3, mnc1, mnc2, amfID, tmsi)
	}

	return fmt.Sprintf("%d%d%d%d%d%d%s%s", mcc1, mcc2, mcc3, mnc1, mnc2, mnc3, amfID, tmsi)
}

func nasToAmfId(regionID uint8, setID uint16, pointer uint8) string {
	setID &= 0x03FF // 10 bits
	pointer &= 0x3F // 6 bits

	b0 := regionID
	b1 := uint8(setID >> 2)
	b2 := uint8((setID&0x3)<<6) | (pointer & 0x3F)

	return fmt.Sprintf("%02x%02x%02x", b0, b1, b2)
}

// parseAllowedNSSAI parses the NAS AllowedNSSAI value bytes into a list of slices.
// Each entry is: [length, SST, (SD[0], SD[1], SD[2])?]
func parseAllowedNSSAI(data []byte) ([]ExpectedSlice, error) {
	var result []ExpectedSlice

	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			return nil, fmt.Errorf("unexpected end of allowed NSSAI data")
		}

		length := int(data[offset])
		offset++

		if offset+length > len(data) {
			return nil, fmt.Errorf("allowed NSSAI entry length %d exceeds remaining data at offset %d", length, offset)
		}

		sst := int32(data[offset])
		sd := ""

		if length >= 4 {
			sd = fmt.Sprintf("%02x%02x%02x", data[offset+1], data[offset+2], data[offset+3])
		}

		result = append(result, ExpectedSlice{Sst: sst, Sd: sd})
		offset += length
	}

	return result, nil
}
