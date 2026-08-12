// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"
)

func pathSwitchULI() UserLocationInformation {
	return UserLocationInformation{
		Kind:         UserLocationNR,
		PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39},
		CellIdentity: 1,
		TAI:          TAI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, TAC: 1},
	}
}

const goldenPathSwitchRequest = "001900430000050055000200020064000200010079400f4002f839000000001002" +
	"f83900000100774009100008000000000000004c00100000050c001fc0a80101000000010002"

func TestPathSwitchRequestGolden(t *testing.T) {
	transfer, err := (&PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1}, GTPTEID: 1,
		}},
		QosFlowAccepted: QosFlowAcceptedList{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	uli := pathSwitchULI()
	caps := UESecurityCapabilities{NREncryptionAlgorithms: 0x8000, NRIntegrityProtectionAlgorithms: 0x8000}

	in := PathSwitchRequest{
		RANUENGAPID: 2, SourceAMFUENGAPID: 1,
		UserLocationInformation:              &uli,
		UESecurityCapabilities:               &caps,
		PDUSessionResourceToBeSwitchedDLList: PDUSessionResourceToBeSwitchedDLList{{PDUSessionID: 5, Transfer: transfer}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenPathSwitchRequest {
		t.Fatalf("encoded %s, want %s", got, goldenPathSwitchRequest)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcPathSwitchRequest {
		t.Fatalf("decoded %T, want an initiating PathSwitchRequest", pdu)
	}

	out, err := ParsePathSwitchRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.RANUENGAPID != 2 || out.SourceAMFUENGAPID != 1 ||
		out.UserLocationInformation == nil || out.UserLocationInformation.TAI.TAC != 1 ||
		len(out.PDUSessionResourceToBeSwitchedDLList) != 1 || out.PDUSessionResourceToBeSwitchedDLList[0].PDUSessionID != 5 {
		t.Fatalf("round trip %+v", out)
	}

	if _, err := ParsePathSwitchRequestTransfer(out.PDUSessionResourceToBeSwitchedDLList[0].Transfer); err != nil {
		t.Fatalf("nested transfer: %v", err)
	}
}

const goldenPathSwitchRequestAcknowledge = "20190043000005000a40020001005540020002005d0021080000000000000000" +
	"000000000000000000000000000000000000000000000000004d40050000050100000000020001"

func TestPathSwitchRequestAcknowledgeGolden(t *testing.T) {
	transfer, err := (&PathSwitchRequestAcknowledgeTransfer{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	amfID, ranID := AMFUENGAPID(1), RANUENGAPID(2)

	in := PathSwitchRequestAcknowledge{
		AMFUENGAPID: &amfID, RANUENGAPID: &ranID,
		SecurityContext:                SecurityContext{NextHopChainingCount: 1},
		PDUSessionResourceSwitchedList: PDUSessionResourceSwitchedList{{PDUSessionID: 5, Transfer: transfer}},
		AllowedNSSAI:                   AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenPathSwitchRequestAcknowledge {
		t.Fatalf("encoded %s, want %s", got, goldenPathSwitchRequestAcknowledge)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcPathSwitchRequest {
		t.Fatalf("decoded %T, want a successful PathSwitchRequest outcome", pdu)
	}

	out, err := ParsePathSwitchRequestAcknowledge(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.SecurityContext.NextHopChainingCount != 1 || len(out.PDUSessionResourceSwitchedList) != 1 ||
		out.PDUSessionResourceSwitchedList[0].PDUSessionID != 5 || len(out.AllowedNSSAI) != 1 {
		t.Fatalf("round trip %+v", out)
	}
}

const goldenPathSwitchRequestFailure = "40190019000003000a40020001005540020002004540060000050201a0"

// Where TS 36.413 names one Cause for the whole message, NGAP reports per
// session: each released session carries its own cause in a
// PathSwitchRequestUnsuccessfulTransfer.
func TestPathSwitchRequestFailureGolden(t *testing.T) {
	transfer, err := (&PathSwitchRequestUnsuccessfulTransfer{
		Cause: Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnknownPDUSessionID},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	amfID, ranID := AMFUENGAPID(1), RANUENGAPID(2)

	in := PathSwitchRequestFailure{
		AMFUENGAPID: &amfID, RANUENGAPID: &ranID,
		PDUSessionResourceReleased: PDUSessionResourceReleasedListPSFail{{PDUSessionID: 5, Transfer: transfer}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenPathSwitchRequestFailure {
		t.Fatalf("encoded %s, want %s", got, goldenPathSwitchRequestFailure)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcPathSwitchRequest {
		t.Fatalf("decoded %T, want an unsuccessful PathSwitchRequest outcome", pdu)
	}

	out, err := ParsePathSwitchRequestFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.PDUSessionResourceReleased) != 1 || out.PDUSessionResourceReleased[0].PDUSessionID != 5 {
		t.Fatalf("round trip %+v", out)
	}

	released, err := ParsePathSwitchRequestUnsuccessfulTransfer(out.PDUSessionResourceReleased[0].Transfer)
	if err != nil {
		t.Fatal(err)
	}

	if released.Cause.Group != CauseGroupRadioNetwork || released.Cause.Value != CauseRadioNetworkUnknownPDUSessionID {
		t.Errorf("nested cause %+v", released.Cause)
	}
}

// The AMF must not send a PATH SWITCH REQUEST ACKNOWLEDGE that omits an IE
// §9.2.3.11 makes mandatory (§9.1.1 binds the sender).
func TestPathSwitchRequestAcknowledgeRefusesMissingMandatoryIEs(t *testing.T) {
	amfID, ranID := AMFUENGAPID(1), RANUENGAPID(2)
	full := func() PathSwitchRequestAcknowledge {
		return PathSwitchRequestAcknowledge{
			AMFUENGAPID: &amfID, RANUENGAPID: &ranID,
			PDUSessionResourceSwitchedList: PDUSessionResourceSwitchedList{{PDUSessionID: 5, Transfer: TransferContainer{0x00}}},
			AllowedNSSAI:                   AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		}
	}

	complete := full()
	if _, err := complete.Marshal(); err != nil {
		t.Fatalf("the complete message must encode: %v", err)
	}

	for _, tt := range []struct {
		name string
		omit func(*PathSwitchRequestAcknowledge)
	}{
		{"PDUSessionResourceSwitchedList", func(m *PathSwitchRequestAcknowledge) { m.PDUSessionResourceSwitchedList = nil }},
		{"AllowedNSSAI", func(m *PathSwitchRequestAcknowledge) { m.AllowedNSSAI = nil }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := full()
			tt.omit(&m)

			if _, err := m.Marshal(); err == nil {
				t.Errorf("encoded a PATH SWITCH REQUEST ACKNOWLEDGE with no %s", tt.name)
			}
		})
	}
}

// §9.2.3.10 makes the released list mandatory in PATH SWITCH REQUEST FAILURE,
// so answering a rejected PATH SWITCH REQUEST with that message needs the
// sessions it asked to switch. They are recovered from the IEs the failed
// decode still collected.
func TestPathSwitchSessionsSurviveRejection(t *testing.T) {
	transfer, err := (&PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1}, GTPTEID: 1,
		}},
		QosFlowAccepted: QosFlowAcceptedList{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	uli := pathSwitchULI()
	caps := UESecurityCapabilities{NREncryptionAlgorithms: 0x8000, NRIntegrityProtectionAlgorithms: 0x8000}
	sessions := PDUSessionResourceToBeSwitchedDLList{{PDUSessionID: 5, Transfer: transfer}}

	fields := []ieField{
		{id: IDRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(2)},
		{id: IDSourceAMFUENGAPID, crit: CriticalityReject, val: AMFUENGAPID(1)},
		{id: IDUserLocationInformation, crit: CriticalityIgnore, val: &uli},
		{id: IDUESecurityCapabilities, crit: CriticalityIgnore, val: &caps},
		{id: IDPDUSessionResourceToBeSwitchedDLList, crit: CriticalityReject, val: sessions},
	}

	// §10.3.6: the same IE twice is a falsely constructed message.
	value := container(t, append(fields, fields[0])...)

	_, err = ParsePathSwitchRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("err = %v, want an AbstractSyntaxError", err)
	}

	got := ase.PathSwitchSessions()
	if len(got) != 1 || got[0].PDUSessionID != 5 {
		t.Fatalf("PathSwitchSessions = %+v, want one item for PDU session 5", got)
	}

	amfID, ranID := ase.UEIDs()
	if deref(amfID) != 1 || deref(ranID) != 2 {
		t.Fatalf("UEIDs = %v/%v, want 1/2", amfID, ranID)
	}
}

// Without the list the failure message cannot be built, and §10.3.5 sends an
// Error Indication instead.
func TestPathSwitchSessionsAbsent(t *testing.T) {
	uli := pathSwitchULI()

	value := container(t,
		ieField{id: IDRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(2)},
		ieField{id: IDSourceAMFUENGAPID, crit: CriticalityReject, val: AMFUENGAPID(1)},
		ieField{id: IDUserLocationInformation, crit: CriticalityIgnore, val: &uli},
	)

	_, err := ParsePathSwitchRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("err = %v, want an AbstractSyntaxError", err)
	}

	if got := ase.PathSwitchSessions(); got != nil {
		t.Fatalf("PathSwitchSessions = %+v, want nil", got)
	}
}
