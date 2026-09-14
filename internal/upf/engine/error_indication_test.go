// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type recordingReportHandler struct {
	reports []*models.ErrorIndicationReport
}

func (r *recordingReportHandler) HandleDownlinkDataReport(context.Context, *models.DownlinkDataReport) error {
	return nil
}

func (r *recordingReportHandler) HandleErrorIndicationReport(_ context.Context, report *models.ErrorIndicationReport) error {
	r.reports = append(r.reports, report)

	return nil
}

func (r *recordingReportHandler) HandleUsageReports(context.Context, []*models.UsageReport) error {
	return nil
}

func (r *recordingReportHandler) SendFlowReports(context.Context, []*models.FlowReportRequest) error {
	return nil
}

func addSessionWithFAR(eng *SessionEngine, seid uint64, farID uint32, far ebpf.FarInfo) {
	sess := NewSession(seid)
	sess.PutFar(farID, far)
	eng.sessions[seid] = sess
}

func gtpUFAR(teid uint32, peer netip.Addr) ebpf.FarInfo {
	ohc := uint8(1)
	if peer.Is6() {
		ohc = 2
	}

	return ebpf.FarInfo{
		Action:              2,
		OuterHeaderCreation: ohc,
		TeID:                teid,
		RemoteIP:            ebpf.IPToIn6Addr(peer),
	}
}

// TS 29.281 §7.3.1: the reported TEID and peer address name the FAR that was
// transmitting into the tunnel, not a PDR's local F-TEID.
func TestFindFARByRemoteFTEIDMatchesTheTransmittingFAR(t *testing.T) {
	eng := newTestEngine()

	peer := netip.MustParseAddr("10.0.0.1")
	other := netip.MustParseAddr("10.0.0.9")

	addSessionWithFAR(eng, 1, 2, gtpUFAR(0x1111, other))
	addSessionWithFAR(eng, 7, 3, gtpUFAR(0x2222, peer))

	match, ok := eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x2222, Addr: peer})
	if !ok {
		t.Fatal("the Error Indication did not resolve to any session")
	}

	if match.SEID != 7 || match.FARID != 3 {
		t.Errorf("resolved to SEID %d FAR %d, want SEID 7 FAR 3", match.SEID, match.FARID)
	}
}

func TestFindFARByRemoteFTEIDNeedsBothTheTEIDAndThePeer(t *testing.T) {
	eng := newTestEngine()

	peer := netip.MustParseAddr("10.0.0.1")
	addSessionWithFAR(eng, 1, 1, gtpUFAR(0x1111, peer))

	if _, ok := eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x1111, Addr: netip.MustParseAddr("10.0.0.2")}); ok {
		t.Error("matched a FAR whose peer address differs; a reused TEID would hit the wrong session")
	}

	if _, ok := eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x2222, Addr: peer}); ok {
		t.Error("matched a FAR whose TEID differs")
	}
}

func TestFindFARByRemoteFTEIDMatchesAnIPv6Peer(t *testing.T) {
	eng := newTestEngine()

	peer := netip.MustParseAddr("2001:db8::aa")
	addSessionWithFAR(eng, 4, 6, gtpUFAR(0x3333, peer))

	match, ok := eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x3333, Addr: peer})
	if !ok {
		t.Fatal("an IPv6 peer address did not resolve")
	}

	if match.SEID != 4 || match.FARID != 6 {
		t.Errorf("resolved to SEID %d FAR %d, want SEID 4 FAR 6", match.SEID, match.FARID)
	}
}

func TestFindFARByRemoteFTEIDIgnoresAFARThatDoesNotEncapsulate(t *testing.T) {
	eng := newTestEngine()

	peer := netip.MustParseAddr("10.0.0.1")
	far := gtpUFAR(0x1111, peer)
	far.OuterHeaderCreation = 0

	addSessionWithFAR(eng, 1, 1, far)

	if _, ok := eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x1111, Addr: peer}); ok {
		t.Error("matched a FAR with no outer header creation; it forwards no GTP-U tunnel")
	}
}

// The FAR map is keyed by the SMF's own FAR ID, so the report lets the SMF tell
// a dead access tunnel from a dead handover forwarding tunnel: the reactions
// differ (TS 23.527 §5.3.2 and TS 23.007 §21.7 against TS 23.502 §4.9.1.3.3).
func TestFindFARByRemoteFTEIDTellsAccessFromForwarding(t *testing.T) {
	const (
		farIDDownlink   = 2
		farIDForwarding = 3
	)

	eng := newTestEngine()

	sourceRAN := netip.MustParseAddr("10.0.0.1")
	targetRAN := netip.MustParseAddr("10.0.0.2")

	sess := NewSession(11)
	sess.PutFar(farIDDownlink, gtpUFAR(0x1111, sourceRAN))
	sess.PutFar(farIDForwarding, gtpUFAR(0x2222, targetRAN))
	eng.sessions[11] = sess

	match, ok := eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x2222, Addr: targetRAN})
	if !ok {
		t.Fatal("the forwarding tunnel did not resolve")
	}

	if match.SEID != 11 || match.FARID != farIDForwarding {
		t.Errorf("resolved to SEID %d FAR %d, want SEID 11 FAR %d", match.SEID, match.FARID, farIDForwarding)
	}

	match, ok = eng.FindFARByRemoteFTEID(models.FTEID{TEID: 0x1111, Addr: sourceRAN})
	if !ok {
		t.Fatal("the access tunnel did not resolve")
	}

	if match.SEID != 11 || match.FARID != farIDDownlink {
		t.Errorf("resolved to SEID %d FAR %d, want SEID 11 FAR %d", match.SEID, match.FARID, farIDDownlink)
	}
}

// TS 29.244 §7.5.8.4: the report names the remote F-TEID.
func TestSendErrorIndicationReportNamesTheRemoteFTEID(t *testing.T) {
	eng := newTestEngine()

	peer := netip.MustParseAddr("10.0.0.1")
	addSessionWithFAR(eng, 9, 5, gtpUFAR(0x4444, peer))

	smf := &recordingReportHandler{}
	if err := eng.SendErrorIndicationReport(context.Background(), smf, models.FTEID{TEID: 0x4444, Addr: peer}); err != nil {
		t.Fatalf("SendErrorIndicationReport: %v", err)
	}

	if len(smf.reports) != 1 {
		t.Fatalf("SMF got %d reports, want 1", len(smf.reports))
	}

	got := smf.reports[0]
	if got.SEID != 9 || got.FARID != 5 || got.RemoteFTEID != (models.FTEID{TEID: 0x4444, Addr: peer}) {
		t.Errorf("report = %+v, want SEID 9, FAR 5, TEID 0x4444, peer %s", got, peer)
	}
}

func TestSendErrorIndicationReportOnAnUnknownTunnelTellsTheSMFNothing(t *testing.T) {
	eng := newTestEngine()

	smf := &recordingReportHandler{}

	err := eng.SendErrorIndicationReport(context.Background(), smf,
		models.FTEID{TEID: 0x4444, Addr: netip.MustParseAddr("10.0.0.1")})
	if err == nil {
		t.Fatal("an Error Indication for a tunnel no session forwards into was reported as resolved")
	}

	if len(smf.reports) != 0 {
		t.Errorf("SMF got %d reports for an unresolved Error Indication, want 0", len(smf.reports))
	}
}
