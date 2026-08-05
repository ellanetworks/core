// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"errors"
	"testing"
)

// A real INITIAL CONTEXT SETUP RESPONSE: MME-UE-S1AP-ID 0x020000bf,
// eNB-UE-S1AP-ID 1, one E-RAB.
const goldenICSResponse = "2009002500000300004005c0020000bf0008400200010033400f000032400a0a1f0a0123c601000908"

func TestInitialContextSetupResponseGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenICSResponse))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcInitialContextSetup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	resp, err := ParseInitialContextSetupResponse(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if deref(resp.MMEUES1APID) != 0x020000bf {
		t.Fatalf("MME-UE-S1AP-ID = %#x, want 0x020000bf", deref(resp.MMEUES1APID))
	}

	if deref(resp.ENBUES1APID) != 1 {
		t.Fatalf("eNB-UE-S1AP-ID = %d, want 1", deref(resp.ENBUES1APID))
	}

	if len(resp.ERABSetup) != 1 {
		t.Fatalf("ERABSetup len = %d, want 1", len(resp.ERABSetup))
	}

	if len(resp.ERABSetup[0].TransportLayerAddress) == 0 || resp.ERABSetup[0].GTPTEID == 0 {
		t.Fatalf("E-RAB item missing TLA/TEID: %+v", resp.ERABSetup[0])
	}

	// Semantic round-trip: re-encode and re-decode must reproduce the fields.
	b2, err := resp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu2, err := Unmarshal(b2)
	if err != nil {
		t.Fatal(err)
	}

	resp2, err := ParseInitialContextSetupResponse(pdu2.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatal(err)
	}

	if deref(resp2.MMEUES1APID) != deref(resp.MMEUES1APID) || deref(resp2.ENBUES1APID) != deref(resp.ENBUES1APID) ||
		len(resp2.ERABSetup) != len(resp.ERABSetup) {
		t.Fatalf("round-trip header mismatch")
	}

	for i := range resp.ERABSetup {
		a, b := resp.ERABSetup[i], resp2.ERABSetup[i]
		if a.ERABID != b.ERABID || a.GTPTEID != b.GTPTEID || !bytes.Equal(a.TransportLayerAddress, b.TransportLayerAddress) {
			t.Fatalf("E-RAB %d round-trip mismatch:\n  %+v\n  %+v", i, a, b)
		}
	}
}

func TestInitialContextSetupRoundTrips(t *testing.T) {
	qos := ERABLevelQoSParameters{
		QCI: 9,
		ARP: AllocationAndRetentionPriority{
			PriorityLevel:           15,
			PreemptionCapability:    PreemptionShallNotTrigger,
			PreemptionVulnerability: PreemptionNotPreemptable,
		},
	}

	tla := TransportLayerAddress{10, 45, 0, 1}

	var key SecurityKey
	for i := range key {
		key[i] = byte(i)
	}

	t.Run("Request", func(t *testing.T) {
		in := &InitialContextSetupRequest{
			MMEUES1APID:               0x020000bf,
			ENBUES1APID:               1,
			UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000000000, UL: 500000000},
			ERABToBeSetup: []ERABToBeSetupItemCtxtSUReq{{
				ERABID: 5, QoS: qos, TransportLayerAddress: tla, GTPTEID: 0x12345678,
				NASPDU: NASPDU{0x07, 0x42, 0x01},
			}},
			UESecurityCapabilities: UESecurityCapabilities{EncryptionAlgorithms: 0x8000, IntegrityProtectionAlgorithms: 0xc000},
			SecurityKey:            key,
			UERadioCapability:      []byte{0xaa, 0xbb, 0xcc, 0xdd},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseInitialContextSetupRequest(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID ||
			out.UEAggregateMaximumBitRate != in.UEAggregateMaximumBitRate ||
			out.UESecurityCapabilities != in.UESecurityCapabilities || out.SecurityKey != in.SecurityKey ||
			!bytes.Equal(out.UERadioCapability, in.UERadioCapability) || len(out.ERABToBeSetup) != 1 {
			t.Fatalf("scalar mismatch:\n  in  %+v\n  out %+v", in, out)
		}

		gi, go_ := in.ERABToBeSetup[0], out.ERABToBeSetup[0]
		if gi.ERABID != go_.ERABID || gi.GTPTEID != go_.GTPTEID || gi.QoS.QCI != go_.QoS.QCI ||
			gi.QoS.ARP != go_.QoS.ARP || !bytes.Equal(gi.TransportLayerAddress, go_.TransportLayerAddress) ||
			!bytes.Equal(gi.NASPDU, go_.NASPDU) {
			t.Fatalf("E-RAB item mismatch:\n  in  %+v\n  out %+v", gi, go_)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &InitialContextSetupResponse{
			MMEUES1APID: Ptr(MMEUES1APID(42)), ENBUES1APID: Ptr(ENBUES1APID(1)),
			ERABSetup: []ERABSetupItemCtxtSURes{{ERABID: 5, TransportLayerAddress: tla, GTPTEID: 0x12345678}},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseInitialContextSetupResponse(pdu.(*SuccessfulOutcome).Value)
		if err != nil {
			t.Fatal(err)
		}

		if deref(out.MMEUES1APID) != deref(in.MMEUES1APID) || deref(out.ENBUES1APID) != deref(in.ENBUES1APID) || len(out.ERABSetup) != 1 ||
			out.ERABSetup[0].ERABID != 5 || out.ERABSetup[0].GTPTEID != 0x12345678 ||
			!bytes.Equal(out.ERABSetup[0].TransportLayerAddress, tla) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		in := &InitialContextSetupFailure{
			MMEUES1APID: Ptr(MMEUES1APID(42)), ENBUES1APID: Ptr(ENBUES1APID(1)),
			Cause: Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 0}),
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		if b[0] != 0x40 || b[1] != 0x09 {
			t.Fatalf("envelope prefix = % x, want 40 09 (unsuccessfulOutcome / ICS)", b[:2])
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseInitialContextSetupFailure(pdu.(*UnsuccessfulOutcome).Value)
		if err != nil {
			t.Fatal(err)
		}

		if deref(out.MMEUES1APID) != deref(in.MMEUES1APID) || deref(out.ENBUES1APID) != deref(in.ENBUES1APID) ||
			deref(out.Cause) != deref(in.Cause) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})
}

// The mandatory reject-criticality IEs stop delivery when absent (§10.3.5); the
// MME cannot set up a context without them.
func TestInitialContextSetupRequestMissingRejectIE(t *testing.T) {
	partial := container(t,
		ieField{id: idMMEUES1APID, crit: CriticalityReject, raw: ieRaw(t, Ptr(MMEUES1APID(1)))},
		ieField{id: idENBUES1APID, crit: CriticalityReject, raw: ieRaw(t, Ptr(ENBUES1APID(2)))},
		ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, raw: ieRaw(t, &UEAggregateMaximumBitRate{})},
		ieField{id: idUESecurityCapabilities, crit: CriticalityReject, raw: ieRaw(t, &UESecurityCapabilities{})},
		ieField{id: idSecurityKey, crit: CriticalityReject, raw: ieRaw(t, Ptr(SecurityKey{}))},
	)

	// Without the E-RAB list the request is incomplete: it is mandatory and
	// reject criticality.
	if _, err := ParseInitialContextSetupRequest(partial); err == nil {
		t.Fatal("parse succeeded without the E-RAB To Be Setup list")
	} else {
		var ase *AbstractSyntaxError
		if !errors.As(err, &ase) {
			t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
		}

		if len(ase.IEs) != 1 || ase.IEs[0].IEID != idERABToBeSetupListCtxtSUReq ||
			ase.IEs[0].TypeOfError != TypeOfErrorMissing {
			t.Errorf("diagnostics = %+v, want one missing entry for the E-RAB To Be Setup list", ase.IEs)
		}
	}
}
