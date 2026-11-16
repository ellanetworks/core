// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

func handoverENB(t *testing.T, enbID uint32, primary s1ap.PLMNIdentity, additional ...nas.PLMN) *ENB {
	t.Helper()

	bplmns, err := broadcastPLMNs(primary, additional)
	if err != nil {
		t.Fatalf("broadcastPLMNs: %v", err)
	}

	return &ENB{
		enbID: enbID, enbIDKind: ENBIDMacro, name: "ella", plmn: primary, bplmns: bplmns, tac: 1,
		selected: make(map[int64]s1ap.PLMNIdentity), nextENBUEID: 1, n3Addr: net.IPv4(10, 0, 0, 2).To4(),
	}
}

func handoverNotifyPLMNs(t *testing.T, e *ENB, enbUEID int64) (tai, cgi s1ap.PLMNIdentity) {
	t.Helper()

	b, err := e.buildHandoverNotify(enbUEID, 9)
	if err != nil {
		t.Fatalf("build Handover Notify: %v", err)
	}

	msg, err := s1ap.ParseHandoverNotify(initiatingValue(t, b, s1ap.ProcHandoverNotification))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.TAI == nil || msg.EUTRANCGI == nil {
		t.Fatalf("Handover Notify = %+v, want a TAI and an E-UTRAN CGI", msg)
	}

	return msg.TAI.PLMNIdentity, msg.EUTRANCGI.PLMNIdentity
}

func pathSwitchRequestPLMNs(t *testing.T, e *ENB, enbUEID int64) (tai, cgi s1ap.PLMNIdentity) {
	t.Helper()

	b, err := e.buildPathSwitchRequest(enbUEID, 9, 5, 0x10, s1ap.UESecurityCapabilities{})
	if err != nil {
		t.Fatalf("build Path Switch Request: %v", err)
	}

	msg, err := s1ap.ParsePathSwitchRequest(initiatingValue(t, b, s1ap.ProcPathSwitchRequest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.TAI == nil || msg.EUTRANCGI == nil {
		t.Fatalf("Path Switch Request = %+v, want a TAI and an E-UTRAN CGI", msg)
	}

	return msg.TAI.PLMNIdentity, msg.EUTRANCGI.PLMNIdentity
}

func TestHandoverTargetReportsTheSelectedPLMN(t *testing.T) {
	source := handoverENB(t, 0x19b, testHomePLMN, nas.PLMN{MCC: "310", MNC: "260"})
	target := handoverENB(t, 0x19c, testHomePLMN, nas.PLMN{MCC: "310", MNC: "260"})

	ue := source.NewUE("310260000000001", [16]byte{}, [16]byte{})
	if err := ue.SelectPLMN("310", "260"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	id := target.AllocateENBUEIDFor(ue)

	if tai, cgi := handoverNotifyPLMNs(t, target, id); tai != testOtherPLMN || cgi != testHomePLMN {
		t.Errorf("Handover Notify TAI PLMN = % x, E-UTRAN CGI PLMN = % x; want the selected 310/260 and the primary 001/01", tai, cgi)
	}

	if tai, cgi := pathSwitchRequestPLMNs(t, target, id); tai != testOtherPLMN || cgi != testHomePLMN {
		t.Errorf("Path Switch Request TAI PLMN = % x, E-UTRAN CGI PLMN = % x; want the selected 310/260 and the primary 001/01", tai, cgi)
	}
}

func TestHandoverTargetKeepsTheSourcePrimaryPLMN(t *testing.T) {
	source := handoverENB(t, 0x19b, testOtherPLMN)
	target := handoverENB(t, 0x19c, testHomePLMN, nas.PLMN{MCC: "310", MNC: "260"})

	id := target.AllocateENBUEIDFor(source.NewUE("310260000000001", [16]byte{}, [16]byte{}))

	if tai, _ := handoverNotifyPLMNs(t, target, id); tai != testOtherPLMN {
		t.Errorf("Handover Notify TAI PLMN = % x, want the source's 310/260", tai)
	}

	if tai, _ := pathSwitchRequestPLMNs(t, target, id); tai != testOtherPLMN {
		t.Errorf("Path Switch Request TAI PLMN = % x, want the source's 310/260", tai)
	}
}

func TestHandoverTargetNeverReportsAPLMNItDoesNotBroadcast(t *testing.T) {
	source := handoverENB(t, 0x19b, testHomePLMN, nas.PLMN{MCC: "310", MNC: "014"})
	target := handoverENB(t, 0x19c, testHomePLMN, nas.PLMN{MCC: "310", MNC: "260"})

	ue := source.NewUE("310014000000001", [16]byte{}, [16]byte{})
	if err := ue.SelectPLMN("310", "014"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	id := target.AllocateENBUEIDFor(ue)

	if tai, _ := handoverNotifyPLMNs(t, target, id); tai != testHomePLMN {
		t.Errorf("Handover Notify TAI PLMN = % x, want the target's primary 001/01", tai)
	}
}
