// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/ngap"
)

var (
	hoHomePLMN  = ngap.PLMNIdentity{0x00, 0xf1, 0x10}
	hoOtherPLMN = ngap.PLMNIdentity{0x13, 0x00, 0x62}
)

func multiPLMNGnodeB(gnbID, mcc, mnc string, additional ...BroadcastPLMN) *GnodeB {
	return &GnodeB{GnbID: gnbID, MCC: mcc, MNC: mnc, TAC: "000001", AdditionalPLMNs: additional}
}

func handoverNotifyTAIPLMN(t *testing.T, g *GnodeB, ranUENGAPID int64) (tai, cgi ngap.PLMNIdentity) {
	t.Helper()

	pdu, err := g.handoverNotifyPDU(&HandoverNotifyOpts{AMFUENGAPID: 9, RANUENGAPID: ranUENGAPID})
	if err != nil {
		t.Fatalf("build Handover Notify: %v", err)
	}

	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcHandoverNotification {
		t.Fatalf("got %T, want a Handover Notify", msg)
	}

	notify, err := ngap.ParseHandoverNotify(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if notify.UserLocationInformation == nil || notify.UserLocationInformation.NR == nil {
		t.Fatalf("location = %+v, want NR", notify.UserLocationInformation)
	}

	nr := notify.UserLocationInformation.NR

	return nr.TAI.PLMNIdentity, nr.NRCGI.PLMNIdentity
}

func pathSwitchRequestTAIPLMN(t *testing.T, g *GnodeB, ranUENGAPID int64) (tai, cgi ngap.PLMNIdentity) {
	t.Helper()

	pdu, err := g.pathSwitchRequestPDU(&PathSwitchRequestOpts{
		RANUENGAPID: ranUENGAPID, SourceAMFUENGAPID: 9, N3GnbIp: netip.MustParseAddr("10.0.0.2"),
		PDUSessions: [16]*PDUSessionInformation{{PDUSessionID: 1, DLTEID: 0x10}},
	})
	if err != nil {
		t.Fatalf("build Path Switch Request: %v", err)
	}

	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcPathSwitchRequest {
		t.Fatalf("got %T, want a Path Switch Request", msg)
	}

	req, err := ngap.ParsePathSwitchRequest(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if req.UserLocationInformation == nil || req.UserLocationInformation.NR == nil {
		t.Fatalf("location = %+v, want NR", req.UserLocationInformation)
	}

	nr := req.UserLocationInformation.NR

	return nr.TAI.PLMNIdentity, nr.NRCGI.PLMNIdentity
}

func TestHandoverTargetReportsTheSelectedPLMN(t *testing.T) {
	source := multiPLMNGnodeB("000102", "001", "01", BroadcastPLMN{Mcc: "310", Mnc: "260"})
	target := multiPLMNGnodeB("000103", "001", "01", BroadcastPLMN{Mcc: "310", Mnc: "260"})

	if err := source.SelectPLMN(1, "310", "260"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	if err := target.InheritSelectedPLMN(7, source, 1); err != nil {
		t.Fatalf("InheritSelectedPLMN: %v", err)
	}

	if tai, cgi := handoverNotifyTAIPLMN(t, target, 7); tai != hoOtherPLMN || cgi != hoHomePLMN {
		t.Errorf("Handover Notify TAI PLMN = % x, NR CGI PLMN = % x; want the selected 310/260 and the primary 001/01", tai, cgi)
	}

	if tai, cgi := pathSwitchRequestTAIPLMN(t, target, 7); tai != hoOtherPLMN || cgi != hoHomePLMN {
		t.Errorf("Path Switch Request TAI PLMN = % x, NR CGI PLMN = % x; want the selected 310/260 and the primary 001/01", tai, cgi)
	}

	if tai, _ := handoverNotifyTAIPLMN(t, target, 8); tai != hoHomePLMN {
		t.Errorf("another UE TAI PLMN = % x, want the primary 001/01", tai)
	}
}

func TestHandoverTargetKeepsTheSourcePrimaryPLMN(t *testing.T) {
	source := multiPLMNGnodeB("000102", "310", "260")
	target := multiPLMNGnodeB("000103", "001", "01", BroadcastPLMN{Mcc: "310", Mnc: "260"})

	if err := target.InheritSelectedPLMN(7, source, 1); err != nil {
		t.Fatalf("InheritSelectedPLMN: %v", err)
	}

	if tai, _ := handoverNotifyTAIPLMN(t, target, 7); tai != hoOtherPLMN {
		t.Errorf("Handover Notify TAI PLMN = % x, want the source's 310/260", tai)
	}

	if tai, _ := pathSwitchRequestTAIPLMN(t, target, 7); tai != hoOtherPLMN {
		t.Errorf("Path Switch Request TAI PLMN = % x, want the source's 310/260", tai)
	}
}

func TestInheritSelectedPLMNRefusesAPLMNTheTargetDoesNotBroadcast(t *testing.T) {
	source := multiPLMNGnodeB("000102", "001", "01", BroadcastPLMN{Mcc: "310", Mnc: "014"})
	target := multiPLMNGnodeB("000103", "001", "01", BroadcastPLMN{Mcc: "310", Mnc: "260"})

	if err := source.SelectPLMN(1, "310", "014"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	if err := target.InheritSelectedPLMN(7, source, 1); err == nil {
		t.Fatal("InheritSelectedPLMN accepted a PLMN the target does not broadcast")
	}

	if tai, _ := handoverNotifyTAIPLMN(t, target, 7); tai != hoHomePLMN {
		t.Errorf("Handover Notify TAI PLMN = % x, want the target's primary 001/01", tai)
	}
}
