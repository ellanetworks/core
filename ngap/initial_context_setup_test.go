// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"
)

// Golden INITIAL CONTEXT SETUP PDUs. free5gc/ngap v1.1.3 and pycrate's NGAP
// module, two independent implementations, encode these identically.
const (
	goldenInitialContextSetupRequest           = "000e006b000008000a00020001005500020002006e000a0c3b9aca00301dcd6500001c00070000f11001004000470008000005002002aabb00000002000100770009100004000100004000005e0020000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	goldenInitialContextSetupRequestNoSessions = "000e006b000009000a00020001005500020002001c00070000f11001004000000005020101020300770009100004000100004000005e0020000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f0075400302010200264003027e000076400560010a010b"
	goldenInitialContextSetupResponse          = "200e0022000004000a400200010055400200020048400600000502ccdd0037400500000901ee"
	goldenInitialContextSetupFailure           = "400e001e000004000a400200010055400200020084400500000501ee000f40020000"
)

func goldContextGUAMI() GUAMI {
	return GUAMI{
		PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
		AMFRegionID:  1,
		AMFSetID:     1,
		AMFPointer:   0,
	}
}

func goldSecurityKey() SecurityKey {
	var k SecurityKey
	for i := range k {
		k[i] = byte(i)
	}

	return k
}

func goldUESecurityCapabilities() UESecurityCapabilities {
	return UESecurityCapabilities{
		NREncryptionAlgorithms:             0x8000,
		NRIntegrityProtectionAlgorithms:    0x4000,
		EUTRAEncryptionAlgorithms:          0x2000,
		EUTRAIntegrityProtectionAlgorithms: 0x1000,
	}
}

func goldInitialContextSetupRequest() *InitialContextSetupRequest {
	return &InitialContextSetupRequest{
		AMFUENGAPID:               1,
		RANUENGAPID:               2,
		UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{DL: 1000000000, UL: 500000000},
		GUAMI:                     goldContextGUAMI(),
		PDUSessionResourceSetup: PDUSessionResourceSetupListCxtReq{{
			PDUSessionID: 5,
			SNSSAI:       SNSSAI{SST: 1},
			Transfer:     TransferContainer{0xaa, 0xbb},
		}},
		AllowedNSSAI:           AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		UESecurityCapabilities: goldUESecurityCapabilities(),
		SecurityKey:            goldSecurityKey(),
	}
}

func TestInitialContextSetupGoldenEncodings(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Request", goldInitialContextSetupRequest(), goldenInitialContextSetupRequest},
		{
			"RequestWithoutSessions",
			&InitialContextSetupRequest{
				AMFUENGAPID:            1,
				RANUENGAPID:            2,
				GUAMI:                  goldContextGUAMI(),
				AllowedNSSAI:           AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1, SD: &SD{0x01, 0x02, 0x03}}}},
				UESecurityCapabilities: goldUESecurityCapabilities(),
				SecurityKey:            goldSecurityKey(),
				UERadioCapability:      UERadioCapability{0x01, 0x02},
				NASPDU:                 Ptr(NASPDU{0x7e, 0x00}),
				UERadioCapabilityForPaging: &UERadioCapabilityForPaging{
					NR:    &UERadioCapabilityForPagingOfNR{0x0a},
					EUTRA: &UERadioCapabilityForPagingOfEUTRA{0x0b},
				},
			},
			goldenInitialContextSetupRequestNoSessions,
		},
		{
			"Response",
			&InitialContextSetupResponse{
				AMFUENGAPID: Ptr(AMFUENGAPID(1)),
				RANUENGAPID: Ptr(RANUENGAPID(2)),
				PDUSessionResourceSetup: PDUSessionResourceSetupListCxtRes{{
					PDUSessionID: 5, Transfer: TransferContainer{0xcc, 0xdd},
				}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToSetupListCxtRes{{
					PDUSessionID: 9, Transfer: TransferContainer{0xee},
				}},
			},
			goldenInitialContextSetupResponse,
		},
		{
			"Failure",
			&InitialContextSetupFailure{
				AMFUENGAPID: Ptr(AMFUENGAPID(1)),
				RANUENGAPID: Ptr(RANUENGAPID(2)),
				PDUSessionResourceFailed: PDUSessionResourceFailedToSetupListCxtFail{{
					PDUSessionID: 5, Transfer: TransferContainer{0xee},
				}},
				Cause: Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}),
			},
			goldenInitialContextSetupFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustMarshalHex(t, tt.msg); got != tt.want {
				t.Errorf("encoded\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestInitialContextSetupRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := goldInitialContextSetupRequest()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcInitialContextSetup {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseInitialContextSetupRequest(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 || out.GUAMI != in.GUAMI ||
			out.SecurityKey != in.SecurityKey || out.UESecurityCapabilities != in.UESecurityCapabilities {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}

		if out.UEAggregateMaximumBitRate == nil || *out.UEAggregateMaximumBitRate != *in.UEAggregateMaximumBitRate {
			t.Fatalf("AMBR = %+v", out.UEAggregateMaximumBitRate)
		}

		if len(out.PDUSessionResourceSetup) != 1 {
			t.Fatalf("session list = %+v", out.PDUSessionResourceSetup)
		}

		item := out.PDUSessionResourceSetup[0]
		if item.PDUSessionID != 5 || item.SNSSAI.SST != 1 || string(item.Transfer) != "\xaa\xbb" {
			t.Fatalf("session item = %+v", item)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &InitialContextSetupResponse{
			AMFUENGAPID: Ptr(AMFUENGAPID(7)),
			RANUENGAPID: Ptr(RANUENGAPID(9)),
			PDUSessionResourceSetup: PDUSessionResourceSetupListCxtRes{
				{PDUSessionID: 1, Transfer: TransferContainer{0x01}},
				{PDUSessionID: 2, Transfer: TransferContainer{0x02}},
			},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcInitialContextSetup {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParseInitialContextSetupResponse(so.Value)
		if err != nil || deref(out.AMFUENGAPID) != 7 || deref(out.RANUENGAPID) != 9 {
			t.Fatalf("got %+v err %v", out, err)
		}

		if len(out.PDUSessionResourceSetup) != 2 || out.PDUSessionResourceSetup[1].PDUSessionID != 2 {
			t.Fatalf("session list = %+v", out.PDUSessionResourceSetup)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}
		in := &InitialContextSetupFailure{
			AMFUENGAPID: Ptr(AMFUENGAPID(7)),
			RANUENGAPID: Ptr(RANUENGAPID(9)),
			Cause:       &cause,
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		uo, ok := pdu.(*UnsuccessfulOutcome)
		if !ok || uo.ProcedureCode != ProcInitialContextSetup {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParseInitialContextSetupFailure(uo.Value)
		if err != nil || deref(out.Cause) != cause {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

// §9.2.2.1: the UE Aggregate Maximum Bit Rate "shall be present if the PDU
// Session Resource Setup List IE is present". The sender obligation is enforced
// both ways so neither a missing nor a stray IE reaches the wire.
func TestInitialContextSetupRequestAMBRCondition(t *testing.T) {
	t.Run("sessions without AMBR", func(t *testing.T) {
		m := goldInitialContextSetupRequest()
		m.UEAggregateMaximumBitRate = nil

		if _, err := m.Marshal(); err == nil {
			t.Fatal("encoded a session list with no UE Aggregate Maximum Bit Rate")
		}
	})

	t.Run("AMBR without sessions", func(t *testing.T) {
		m := goldInitialContextSetupRequest()
		m.PDUSessionResourceSetup = nil

		if _, err := m.Marshal(); err == nil {
			t.Fatal("encoded a UE Aggregate Maximum Bit Rate with no session list")
		}
	})

	t.Run("neither", func(t *testing.T) {
		m := goldInitialContextSetupRequest()
		m.PDUSessionResourceSetup = nil
		m.UEAggregateMaximumBitRate = nil

		if _, err := m.Marshal(); err != nil {
			t.Fatalf("a request carrying no PDU session is valid: %v", err)
		}
	})
}

// The four reject-criticality mandatory IEs stop delivery when absent
// (§10.3.5); the AMF cannot set up a context without them.
func TestInitialContextSetupRequestMissingRejectIE(t *testing.T) {
	full := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
		ieField{id: idGUAMI, crit: CriticalityReject, raw: ieRaw(t, Ptr(goldContextGUAMI()))},
		ieField{id: idUESecurityCapabilities, crit: CriticalityReject, raw: ieRaw(t, Ptr(goldUESecurityCapabilities()))},
		ieField{id: idSecurityKey, crit: CriticalityReject, raw: ieRaw(t, Ptr(goldSecurityKey()))},
	)

	// Without the Allowed NSSAI the request is incomplete: it is mandatory and
	// reject criticality.
	if _, err := ParseInitialContextSetupRequest(full); err == nil {
		t.Fatal("parse succeeded without the Allowed NSSAI")
	} else {
		var ase *AbstractSyntaxError
		if !errors.As(err, &ase) {
			t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
		}

		if len(ase.IEs) != 1 || ase.IEs[0].IEID != idAllowedNSSAI ||
			ase.IEs[0].TypeOfError != TypeOfErrorMissing {
			t.Errorf("diagnostics = %+v, want one missing entry for the AllowedNSSAI", ase.IEs)
		}
	}
}
