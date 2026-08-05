// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"
)

const (
	goldenHandoverCancel            = "000a0015000003000a00020001005500020002000f40020140"
	goldenHandoverCancelAcknowledge = "200a000f000002000a40020001005540020002"
)

func TestHandoverCancelGolden(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverCancelled}

	in := HandoverCancel{AMFUENGAPID: 1, RANUENGAPID: 2, Cause: &cause}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverCancel {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverCancel)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverCancel {
		t.Fatalf("decoded %T, want an initiating HandoverCancel", pdu)
	}

	out, err := ParseHandoverCancel(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 || out.Cause == nil || *out.Cause != cause {
		t.Fatalf("round trip %+v, want 1/2 and %v", out, cause)
	}
}

func TestHandoverCancelAcknowledgeGolden(t *testing.T) {
	amfID, ranID := AMFUENGAPID(1), RANUENGAPID(2)

	in := HandoverCancelAcknowledge{AMFUENGAPID: &amfID, RANUENGAPID: &ranID}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverCancelAcknowledge {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverCancelAcknowledge)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverCancel {
		t.Fatalf("decoded %T, want a successful HandoverCancel outcome", pdu)
	}

	out, err := ParseHandoverCancelAcknowledge(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 1 || out.RANUENGAPID == nil || *out.RANUENGAPID != 2 {
		t.Fatalf("round trip %+v, want 1/2", out)
	}

	if out.CriticalityDiagnostics != nil {
		t.Errorf("CriticalityDiagnostics = %+v, want nil", out.CriticalityDiagnostics)
	}
}

// Cause is mandatory with ignore criticality, so §10.3.5 lets the procedure
// continue without it; the AMF still has to see that it was missing.
func TestHandoverCancelMissingCause(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	out, err := ParseHandoverCancel(value)
	if err != nil {
		t.Fatalf("an absent ignore-criticality IE must not reject the procedure: %v", err)
	}

	if out.Cause != nil {
		t.Errorf("Cause = %+v, want nil", out.Cause)
	}

	ies := out.meta().diagnostics.IEs
	if len(ies) != 1 || ies[0].ID != idCause || ies[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for Cause", ies)
	}
}

func TestHandoverCancelMissingRANUENGAPID(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
	)

	_, err := ParseHandoverCancel(value)
	if err == nil {
		t.Fatal("parsed without the RAN UE NGAP ID")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}
}
