// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func testNOR() NotifyRequest {
	return NotifyRequest{
		IMSI:     testIMSI,
		Features: Features{List2: FeatureSMSInMME},
		Terminal: &TerminalInformation{IMEI: testIMEI, SoftwareVersion: testSoftware},
		PDNGateway: &MIP6AgentInfo{
			HomeAgentAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.9")},
			HomeAgentHost:      "pgw.example.org", HomeAgentRealm: homeRealm,
		},
		VisitedNetworkIdentifier:  "mnc093.mcc208.3gppnetwork.org",
		ContextIdentifier:         2,
		ServiceSelection:          "ims",
		AlertReason:               ptr(AlertReasonUEPresent),
		UESRVCCCapability:         ptr(UESRVCCSupported),
		Flags:                     NORFlagSingleRegistrationIndication | NORFlagRemovalOfMMERegistrationForSMS,
		HomogeneousIMSVoiceOverPS: ptr(IMSVoiceOverPSSupported),
		MaximumUEAvailabilityTime: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestNotifyRoundTrip(t *testing.T) {
	for name, r := range map[string]NotifyRequest{"full": testNOR(), "minimal": {IMSI: testIMSI}} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewNotifyRequest(mmeEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			got, err := ParseNotifyRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}

			n := Notify{Features: Features{List2: FeatureSMSInMME}}

			ans := must[*diameter.Message](t)(NewNotifyAnswer(req, hssIdentity, n))
			if back, err := ParseNotifyAnswer(roundTrip(t, ans)); err != nil || back != n {
				t.Fatalf("NOA = %+v, %v", back, err)
			}
		})
	}
}

func TestNotifyValidation(t *testing.T) {
	for name, mutate := range map[string]func(*NotifyRequest){
		"bad alert":       func(r *NotifyRequest) { r.AlertReason = ptr(AlertReason(2)) },
		"bad SRVCC":       func(r *NotifyRequest) { r.UESRVCCCapability = ptr(UESRVCCCapability(2)) },
		"bad homogeneous": func(r *NotifyRequest) { r.HomogeneousIMSVoiceOverPS = ptr(IMSVoiceOverPS(2)) },
		"reserved flag":   func(r *NotifyRequest) { r.Flags = 1 << 4 },
		"bad terminal":    func(r *NotifyRequest) { r.Terminal = &TerminalInformation{} },
		"bad gateway":     func(r *NotifyRequest) { r.PDNGateway = &MIP6AgentInfo{} },
		"sub-second time": func(r *NotifyRequest) { r.MaximumUEAvailabilityTime = time.Unix(1_800_000_000, 1) },
		"bad IMSI":        func(r *NotifyRequest) { r.IMSI = "" },
		"zoned gateway IP": func(r *NotifyRequest) {
			r.PDNGateway.HomeAgentAddresses = []netip.Addr{netip.MustParseAddr("fe80::1%eth0")}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := testNOR()
			mutate(&r)

			if _, err := NewNotifyRequest(mmeEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestParseNotifyRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewNotifyRequest(mmeEnvelope, testNOR()))

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"zero context":    {replace(base, vendorU32(AVPContextIdentifier, 0)), diameter.ResultInvalidAVPValue},
		"short context":   {replace(base, vendorOctets(AVPContextIdentifier, []byte{1})), diameter.ResultInvalidAVPLength},
		"bad alert":       {replace(base, vendorU32(AVPAlertReason, 3)), diameter.ResultInvalidAVPValue},
		"bad SRVCC":       {replace(base, optionalU32(AVPUESRVCCCapability, 3)), diameter.ResultInvalidAVPValue},
		"bad homogeneous": {replace(base, optionalU32(AVPHomogeneousSupportOfIMSVoiceOverPSSession, 3)), diameter.ResultInvalidAVPValue},
		"bad time":        {replace(base, vendorOctets(tgpp.AVPMaximumUEAvailabilityTime, []byte{1})), diameter.ResultInvalidAVPValue},
		"empty APN":       {replace(base, diameter.UTF8String(diameter.AVPServiceSelection, diameter.AVPFlagMandatory, 0, "")), diameter.ResultInvalidAVPValue},
		"empty visited":   {replace(base, diameter.OctetString(tgpp.AVPVisitedNetworkIdentifier, 0, tgpp.VendorID, nil)), diameter.ResultInvalidAVPValue},
		"bad terminal":    {replace(base, vendorGroup(AVPTerminalInformation, vendorOctets(AVPIMEI, []byte("1")))), diameter.ResultInvalidAVPValue},
		"empty gateway":   {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0)), diameter.ResultInvalidAVPValue},
		"unknown in gateway": {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
			diameter.Unsigned32(4242, diameter.AVPFlagMandatory, 0, 1))), diameter.ResultAVPUnsupported},
		"three gateways": {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
			diameter.Address(diameter.AVPMIPHomeAgentAddress, diameter.AVPFlagMandatory, 0, netip.MustParseAddr("192.0.2.1")),
			diameter.Address(diameter.AVPMIPHomeAgentAddress, diameter.AVPFlagMandatory, 0, netip.MustParseAddr("192.0.2.2")),
			diameter.Address(diameter.AVPMIPHomeAgentAddress, diameter.AVPFlagMandatory, 0, netip.MustParseAddr("192.0.2.3")))), diameter.ResultAVPOccursTooManyTimes},
		"empty host": {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
			diameter.Grouped(diameter.AVPMIPHomeAgentHost, diameter.AVPFlagMandatory, 0,
				diameter.UTF8String(diameter.AVPDestinationRealm, 0, 0, "r"), diameter.UTF8String(diameter.AVPDestinationHost, 0, 0, "")))), diameter.ResultInvalidAVPValue},
		"empty realm": {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
			diameter.Grouped(diameter.AVPMIPHomeAgentHost, diameter.AVPFlagMandatory, 0,
				diameter.UTF8String(diameter.AVPDestinationRealm, 0, 0, ""), diameter.UTF8String(diameter.AVPDestinationHost, 0, 0, "h")))), diameter.ResultInvalidAVPValue},
		"no realm": {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
			diameter.Grouped(diameter.AVPMIPHomeAgentHost, diameter.AVPFlagMandatory, 0, diameter.UTF8String(diameter.AVPDestinationHost, 0, 0, "h")))), diameter.ResultMissingAVP},
		"bad host group": {replace(base, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
			diameter.OctetString(diameter.AVPMIPHomeAgentHost, diameter.AVPFlagMandatory, 0, []byte{1}))), diameter.ResultInvalidAVPLength},
		"zero NOR-Flags": {replace(base, optionalU32(AVPNORFlags, 0)), 0},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseNotifyRequest(tt.req)
			if tt.want == 0 {
				if err != nil {
					t.Fatal(err)
				}

				return
			}

			if avpResult(t, err) != tt.want {
				t.Fatalf("err = %v, want %d", err, tt.want)
			}
		})
	}

	got, err := ParseNotifyRequest(replace(base, optionalU32(AVPNORFlags, uint32(NORFlagS6aS6dIndicator|1<<4|1<<20))))
	if err != nil || got.Flags != NORFlagS6aS6dIndicator {
		t.Fatalf("NOR-Flags = %+v, %v", got.Flags, err)
	}
}
