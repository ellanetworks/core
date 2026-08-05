// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

const (
	goldenHandoverRequiredTransferMin = "00"
	// directForwardingPathAvailability present: preamble bit, then the
	// single-root extensible ENUMERATED's extension bit and index.
	goldenHandoverRequiredTransferDPA = "40"

	goldenHandoverCommandTransferMin  = "00"
	goldenHandoverCommandTransferFull = "600f80c0a80101000000010002"

	goldenHandoverRequestAcknowledgeTransferMin  = "0007c0c0a80101000000010001"
	goldenHandoverRequestAcknowledgeTransferFull = "6807c0c0a801010000000101f00a00000100000002040404040007c0c0a8010100000001"

	goldenPathSwitchRequestTransferMin  = "001fc0a80101000000010002"
	goldenPathSwitchRequestTransferFull = "601fc0a80101000000010080000080"

	goldenPathSwitchRequestAcknowledgeTransferMin  = "00"
	goldenPathSwitchRequestAcknowledgeTransferFull = "601fc0a80101000000010000"

	// An all-zero encoding pins almost nothing, so the other CHOICE groups are
	// pinned alongside the radio-network one.
	goldenPathSwitchRequestSetupFailedRadioNetwork  = "0000"
	goldenPathSwitchRequestSetupFailedNAS           = "10"
	goldenPathSwitchRequestSetupFailedTransport     = "08"
	goldenPathSwitchRequestSetupFailedRadioNetwork3 = "0030"
)

func hoTunnel(a byte, teid GTPTEID) UPTransportLayerInformation {
	addr := TransportLayerAddress{192, 168, 1, 1}
	if a != 192 {
		addr = TransportLayerAddress{10, 0, 0, 1}
	}

	return UPTransportLayerInformation{GTPTunnel: GTPTunnel{TransportLayerAddress: addr, GTPTEID: teid}}
}

func TestHandoverAndPathSwitchTransferGoldens(t *testing.T) {
	tun := hoTunnel(192, 1)
	tun2 := hoTunnel(10, 2)
	dpa := DirectForwardingPathAvailable
	reused := DLNGUTNLInformationReusedTrue
	accepted := DataForwardingAcceptedTrue

	secResult := SecurityResult{
		IntegrityProtectionResult:       IntegrityProtectionPerformed,
		ConfidentialityProtectionResult: ConfidentialityProtectionNotPerformed,
	}
	secIndication := SecurityIndication{
		IntegrityProtectionIndication:       IntegrityProtectionRequired,
		ConfidentialityProtectionIndication: ConfidentialityProtectionRequired,
	}

	for _, c := range []struct {
		name   string
		in     per.Marshaler
		golden string
	}{
		{"HandoverRequiredMin", &HandoverRequiredTransfer{}, goldenHandoverRequiredTransferMin},
		{"HandoverRequiredDPA", &HandoverRequiredTransfer{
			DirectForwardingPathAvailability: &dpa,
		}, goldenHandoverRequiredTransferDPA},

		{"HandoverCommandMin", &HandoverCommandTransfer{}, goldenHandoverCommandTransferMin},
		{"HandoverCommandFull", &HandoverCommandTransfer{
			DLForwardingUPTNLInformation: &tun,
			QosFlowToBeForwarded:         QosFlowToBeForwardedList{{QosFlowIdentifier: 1}},
		}, goldenHandoverCommandTransferFull},

		{"HandoverRequestAcknowledgeMin", &HandoverRequestAcknowledgeTransfer{
			DLNGUUPTNLInformation: tun,
			QosFlowSetupResponse:  QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
		}, goldenHandoverRequestAcknowledgeTransferMin},
		{"HandoverRequestAcknowledgeFull", &HandoverRequestAcknowledgeTransfer{
			DLNGUUPTNLInformation:        tun,
			DLForwardingUPTNLInformation: &tun2,
			SecurityResult:               &secResult,
			QosFlowSetupResponse: QosFlowListWithDataForwarding{{
				QosFlowIdentifier: 1, DataForwardingAccepted: &accepted,
			}},
			DataForwardingResponseDRB: DataForwardingResponseDRBList{{
				DRBID: 1, DLForwardingUPTNLInformation: &tun,
			}},
		}, goldenHandoverRequestAcknowledgeTransferFull},

		{"PathSwitchRequestMin", &PathSwitchRequestTransfer{
			DLNGUUPTNLInformation: tun,
			QosFlowAccepted:       QosFlowAcceptedList{{QosFlowIdentifier: 1}},
		}, goldenPathSwitchRequestTransferMin},
		{"PathSwitchRequestFull", &PathSwitchRequestTransfer{
			DLNGUUPTNLInformation:     tun,
			DLNGUTNLInformationReused: &reused,
			UserPlaneSecurityInformation: &UserPlaneSecurityInformation{
				SecurityResult: secResult, SecurityIndication: secIndication,
			},
			QosFlowAccepted: QosFlowAcceptedList{{QosFlowIdentifier: 1}},
		}, goldenPathSwitchRequestTransferFull},

		{"PathSwitchRequestAcknowledgeMin", &PathSwitchRequestAcknowledgeTransfer{}, goldenPathSwitchRequestAcknowledgeTransferMin},
		{"PathSwitchRequestAcknowledgeFull", &PathSwitchRequestAcknowledgeTransfer{
			ULNGUUPTNLInformation: &tun,
			SecurityIndication:    &secIndication,
		}, goldenPathSwitchRequestAcknowledgeTransferFull},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := c.in.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(perBytes(w)); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}
		})
	}
}

func TestPathSwitchRequestSetupFailedTransferGolden(t *testing.T) {
	for _, c := range []struct {
		name   string
		cause  Cause
		golden string
	}{
		{"RadioNetworkUnspecified", Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}, goldenPathSwitchRequestSetupFailedRadioNetwork},
		{"NASNormalRelease", Cause{Group: CauseGroupNAS, Value: CauseNASNormalRelease}, goldenPathSwitchRequestSetupFailedNAS},
		{"TransportUnavailable", Cause{Group: CauseGroupTransport, Value: CauseTransportResourceUnavailable}, goldenPathSwitchRequestSetupFailedTransport},
		{"RadioNetworkNGRANGenerated", Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkReleaseDueToNGRANGeneratedReason}, goldenPathSwitchRequestSetupFailedRadioNetwork3},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := PathSwitchRequestSetupFailedTransfer{Cause: c.cause}

			b, err := in.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(b); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}

			out, err := ParsePathSwitchRequestSetupFailedTransfer(b)
			if err != nil {
				t.Fatal(err)
			}

			if out.Cause != c.cause {
				t.Fatalf("round trip %+v, want %+v", out.Cause, c.cause)
			}
		})
	}
}

// The three transfers the SMF decodes must survive a round trip with every
// optional set, or a field silently drops on the way back.
func TestHandoverTransfersRoundTrip(t *testing.T) {
	tun := hoTunnel(192, 1)
	tun2 := hoTunnel(10, 2)
	accepted := DataForwardingAcceptedTrue

	ack := HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation:        tun,
		DLForwardingUPTNLInformation: &tun2,
		SecurityResult: &SecurityResult{
			IntegrityProtectionResult:       IntegrityProtectionPerformed,
			ConfidentialityProtectionResult: ConfidentialityProtectionNotPerformed,
		},
		QosFlowSetupResponse: QosFlowListWithDataForwarding{{
			QosFlowIdentifier: 5, DataForwardingAccepted: &accepted,
		}},
		QosFlowFailedToSetup: QosFlowListWithCause{{
			QosFlowIdentifier: 6,
			Cause:             Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
		}},
		DataForwardingResponseDRB: DataForwardingResponseDRBList{{
			DRBID: 32, DLForwardingUPTNLInformation: &tun, ULForwardingUPTNLInformation: &tun2,
		}},
	}

	b, err := ack.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverRequestAcknowledgeTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.DLForwardingUPTNLInformation == nil || out.SecurityResult == nil ||
		len(out.QosFlowSetupResponse) != 1 || out.QosFlowSetupResponse[0].DataForwardingAccepted == nil ||
		len(out.QosFlowFailedToSetup) != 1 || len(out.DataForwardingResponseDRB) != 1 {
		t.Fatalf("round trip dropped a field: %+v", out)
	}

	drb := out.DataForwardingResponseDRB[0]
	if drb.DRBID != 32 || drb.DLForwardingUPTNLInformation == nil || drb.ULForwardingUPTNLInformation == nil {
		t.Fatalf("DRB item = %+v, want DRB-ID 32 with both tunnels", drb)
	}
}

// DRB-ID is (1..32, ...): zero is below the lower bound.
func TestDRBIDRejectsZero(t *testing.T) {
	w := per.NewWriter()
	if err := DRBID(0).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encoded a DRB-ID of 0")
	}
}
