// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"
)

const (
	goldenUEContextModificationRequest  = "0028001d000003000a00020001005500020002006e400a0c0bebc2003005f5e100"
	goldenUEContextModificationResponse = "20280022000003000a400200010055400200020079400f4000f110123456789000f110000001"
	goldenUEContextModificationFailure  = "40280015000003000a40020001005540020002000f40020000"
)

func goldUEContextModificationRequest() *UEContextModificationRequest {
	return &UEContextModificationRequest{
		AMFUENGAPID:               1,
		RANUENGAPID:               2,
		UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{DL: 200_000_000, UL: 100_000_000},
	}
}

func goldUEContextModificationResponse() *UEContextModificationResponse {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}

	return &UEContextModificationResponse{
		AMFUENGAPID: Ptr(AMFUENGAPID(1)),
		RANUENGAPID: Ptr(RANUENGAPID(2)),
		UserLocationInformation: &UserLocationInformation{
			Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
			TAI: TAI{PLMNIdentity: plmn, TAC: 1},
		},
	}
}

func goldUEContextModificationFailure() *UEContextModificationFailure {
	return &UEContextModificationFailure{
		AMFUENGAPID: Ptr(AMFUENGAPID(1)),
		RANUENGAPID: Ptr(RANUENGAPID(2)),
		Cause:       Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}),
	}
}

func TestUEContextModificationGoldenEncodings(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Request", goldUEContextModificationRequest(), goldenUEContextModificationRequest},
		{"Response", goldUEContextModificationResponse(), goldenUEContextModificationResponse},
		{"Failure", goldUEContextModificationFailure(), goldenUEContextModificationFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustMarshalHex(t, tt.msg); got != tt.want {
				t.Errorf("encoded\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestUEContextModificationRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := goldUEContextModificationRequest()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcUEContextModification {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextModificationRequest(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 || out.UEAggregateMaximumBitRate == nil ||
			*out.UEAggregateMaximumBitRate != *in.UEAggregateMaximumBitRate {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("Request without UE-AMBR", func(t *testing.T) {
		in := &UEContextModificationRequest{AMFUENGAPID: 1, RANUENGAPID: 2}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseUEContextModificationRequest(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.UEAggregateMaximumBitRate != nil {
			t.Fatalf("UE-AMBR = %+v, want absent", out.UEAggregateMaximumBitRate)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := goldUEContextModificationResponse()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcUEContextModification {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextModificationResponse(so.Value)
		if err != nil {
			t.Fatal(err)
		}

		if deref(out.AMFUENGAPID) != 1 || deref(out.RANUENGAPID) != 2 || out.UserLocationInformation == nil ||
			out.UserLocationInformation.CellIdentity != in.UserLocationInformation.CellIdentity {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		in := goldUEContextModificationFailure()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		uo, ok := pdu.(*UnsuccessfulOutcome)
		if !ok || uo.ProcedureCode != ProcUEContextModification {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextModificationFailure(uo.Value)
		if err != nil {
			t.Fatal(err)
		}

		if deref(out.AMFUENGAPID) != 1 || deref(out.RANUENGAPID) != 2 || deref(out.Cause) != deref(in.Cause) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})
}
