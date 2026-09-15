// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type releaseGuardTestSmf struct {
	*deregisterTestSmf
	relRspCalls []string
}

func (s *releaseGuardTestSmf) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, ref string) (bool, error) {
	s.relRspCalls = append(s.relRspCalls, ref)

	return false, nil
}

func releaseGuardFixture(t *testing.T) (*UeConn, *releaseGuardTestSmf) {
	t.Helper()

	smf := &releaseGuardTestSmf{deregisterTestSmf: &deregisterTestSmf{}}

	a := New(nil, nil, smf)

	radio := &Radio{}
	radio.BindAMFForTest(a)

	ueConn := NewUeConnForTest(radio, 1, 10)

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ueConn.AMFForTest().AttachUeConn(t.Context(), ue, ueConn)
	ueConn.SetN2SessionActive(1)

	t.Cleanup(ueConn.AbortN2Releases)

	return ueConn, smf
}

func TestUnansweredPDUSessionResourceReleaseCompletesLocally(t *testing.T) {
	ueConn, smf := releaseGuardFixture(t)

	ueConn.armN2Release(t.Context(), 1)

	g := ueConn.n2Releases.open[1]
	if g == nil {
		t.Fatal("sending a PDU Session Resource Release Command armed no supervision")
	}

	ueConn.expireN2Release(trace.SpanContext{}, 1, g)

	if !ueConn.N2SessionInactive(1) {
		t.Error("the connection still records AN resources for a release the NG-RAN node never answered")
	}

	if len(smf.relRspCalls) != 1 || smf.relRspCalls[0] != "ref-1" {
		t.Errorf("SMF release-response calls = %v, want one for ref-1 (TS 23.527 §5.3.2.1 step 7)", smf.relRspCalls)
	}
}

func TestAnsweredPDUSessionResourceReleaseDisarmsTheGuard(t *testing.T) {
	ueConn, smf := releaseGuardFixture(t)

	ueConn.armN2Release(t.Context(), 1)

	g := ueConn.n2Releases.open[1]
	if g == nil {
		t.Fatal("sending a PDU Session Resource Release Command armed no supervision")
	}

	ueConn.EndN2Release(1)
	ueConn.expireN2Release(trace.SpanContext{}, 1, g)

	if len(smf.relRspCalls) != 0 {
		t.Errorf("a disarmed guard still completed the release at the SMF: %v", smf.relRspCalls)
	}
}
