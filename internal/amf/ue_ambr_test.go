// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
)

type ueAMBRCapture struct {
	mu   sync.Mutex
	sent [][]byte
}

func (c *ueAMBRCapture) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	c.mu.Lock()
	c.sent = append(c.sent, append([]byte(nil), b...))
	c.mu.Unlock()

	return len(b), nil
}

func (c *ueAMBRCapture) modifications(t *testing.T) []*ngap.UEContextModificationRequest {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()

	var out []*ngap.UEContextModificationRequest

	for _, b := range c.sent {
		pdu, err := ngap.Unmarshal(b)
		if err != nil {
			t.Fatalf("unmarshal NGAP: %v", err)
		}

		im, ok := pdu.(*ngap.InitiatingMessage)
		if !ok || im.ProcedureCode != ngap.ProcUEContextModification {
			continue
		}

		req, err := ngap.ParseUEContextModificationRequest(im.Value)
		if err != nil {
			t.Fatalf("parse UE Context Modification Request: %v", err)
		}

		out = append(out, req)
	}

	return out
}

type ueAMBRDB struct {
	fakeDBInstance
	profile db.Profile
}

func (d *ueAMBRDB) GetSubscriber(_ context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{Imsi: imsi, ProfileID: d.profile.ID}, nil
}

func (d *ueAMBRDB) GetProfileByID(context.Context, string) (*db.Profile, error) {
	p := d.profile

	return &p, nil
}

func ambrOf(ul, dl string) *models.Ambr {
	return &models.Ambr{Uplink: models.MustParseBitRate(ul), Downlink: models.MustParseBitRate(dl)}
}

func ueAMBRConnectedUE(t *testing.T, dbInstance amf.DBer) (*amf.AMF, *amf.UeContext, *amf.UeConn, *ueAMBRCapture) {
	t.Helper()

	capture := &ueAMBRCapture{}
	amfInstance := amf.New(dbInstance, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000071", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
	})

	radio := &amf.Radio{Conn: capture}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1)
	ueConn.AMFForTest().AttachUeConn(t.Context(), ue, ueConn)

	ue.SetAmbr(ambrOf("100 Mbps", "200 Mbps"))

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatal(err)
	}

	ueConn.MarkICSCompleted()

	if err := ueConn.SendPDUSessionResourceSetupRequest(context.Background(), models.MustParseBitRate("100 Mbps"), models.MustParseBitRate("200 Mbps"), nil, ngap.PDUSessionResourceSetupListSUReq{{
		PDUSessionID: 1,
		SNSSAI:       ngap.SNSSAI{SST: 1},
		Transfer:     ngap.TransferContainer{0x00},
	}}); err != nil {
		t.Fatal(err)
	}

	return amfInstance, ue, ueConn, capture
}

func assertNGAPUEAMBR(t *testing.T, got *ngap.UEAggregateMaximumBitRate, dl, ul uint64) {
	t.Helper()

	if got == nil {
		t.Fatal("UE-AMBR IE absent")
	}

	if uint64(got.DL) != dl || uint64(got.UL) != ul {
		t.Fatalf("UE-AMBR = %d/%d bit/s (DL/UL), want %d/%d", got.DL, got.UL, dl, ul)
	}
}

func TestAMFSyncUEAMBRIsSilentWhenTheRANHoldsTheValue(t *testing.T) {
	amfInstance, ue, _, capture := ueAMBRConnectedUE(t, nil)

	amfInstance.SyncUEAMBR(context.Background(), ue)

	if n := len(capture.modifications(t)); n != 0 {
		t.Fatalf("sent %d UE Context Modification Requests for an unchanged UE-AMBR", n)
	}
}

func TestAMFSyncUEAMBRSignalsASubscriptionChange(t *testing.T) {
	amfInstance, ue, ueConn, capture := ueAMBRConnectedUE(t, nil)

	ue.SetAmbr(ambrOf("30 Mbps", "60 Mbps"))
	amfInstance.SyncUEAMBR(context.Background(), ue)

	mods := capture.modifications(t)
	if len(mods) != 1 {
		t.Fatalf("expected one UE Context Modification Request, got %d", len(mods))
	}

	if mods[0].AMFUENGAPID != ngap.AMFUENGAPID(ueConn.AmfUeNgapID) || mods[0].RANUENGAPID != ngap.RANUENGAPID(ueConn.RanUeNgapID()) {
		t.Fatalf("UE NGAP IDs = %d/%d, want %d/%d", mods[0].AMFUENGAPID, mods[0].RANUENGAPID, ueConn.AmfUeNgapID, ueConn.RanUeNgapID())
	}

	assertNGAPUEAMBR(t, mods[0].UEAggregateMaximumBitRate, 60_000_000, 30_000_000)

	amfInstance.SyncUEAMBR(context.Background(), ue)

	if n := len(capture.modifications(t)); n != 1 {
		t.Fatalf("re-signalled a UE-AMBR the RAN already holds (%d requests)", n)
	}
}

func TestAMFSyncUEAMBRWaitsForInitialContextSetup(t *testing.T) {
	amfInstance, ue, ueConn, capture := ueAMBRConnectedUE(t, nil)

	ueConn.MarkICSPending()
	ue.SetAmbr(ambrOf("30 Mbps", "60 Mbps"))
	amfInstance.SyncUEAMBR(context.Background(), ue)

	if n := len(capture.modifications(t)); n != 0 {
		t.Fatalf("sent %d UE Context Modification Requests while the Initial Context Setup was pending", n)
	}

	ueConn.MarkICSCompleted()
	amfInstance.ReconcileSessionsForUE(context.Background(), ue)

	if n := len(capture.modifications(t)); n != 1 {
		t.Fatalf("expected one UE Context Modification Request once the context was set up, got %d", n)
	}
}

func TestAMFSyncUEAMBRResendsAfterAFailure(t *testing.T) {
	amfInstance, ue, ueConn, capture := ueAMBRConnectedUE(t, nil)

	ueConn.ForgetUEAMBR()
	amfInstance.SyncUEAMBR(context.Background(), ue)

	mods := capture.modifications(t)
	if len(mods) != 1 {
		t.Fatalf("expected the UE-AMBR to be re-signalled after a failure, got %d requests", len(mods))
	}

	assertNGAPUEAMBR(t, mods[0].UEAggregateMaximumBitRate, 200_000_000, 100_000_000)
}

func TestAMFSyncUEAMBRSkipsARANWithoutAUEAMBR(t *testing.T) {
	amfInstance, ue, ueConn, capture := ueAMBRConnectedUE(t, nil)

	ue.DeleteSmContext(1)
	ueConn.ForgetUEAMBR()
	amfInstance.SyncUEAMBR(context.Background(), ue)

	if n := len(capture.modifications(t)); n != 0 {
		t.Fatalf("signalled a UE-AMBR to a RAN that holds none and has no PDU session (%d requests)", n)
	}
}

func TestAMFReconcileUEAMBRAppliesTheProfile(t *testing.T) {
	dbInstance := &ueAMBRDB{profile: db.Profile{ID: "p", UeAmbrUplink: "20 Mbps", UeAmbrDownlink: "40 Mbps"}}
	amfInstance, ue, _, capture := ueAMBRConnectedUE(t, dbInstance)

	amfInstance.ReconcileUEAMBR(context.Background())

	mods := capture.modifications(t)
	if len(mods) != 1 {
		t.Fatalf("expected one UE Context Modification Request, got %d", len(mods))
	}

	assertNGAPUEAMBR(t, mods[0].UEAggregateMaximumBitRate, 40_000_000, 20_000_000)

	if got := ue.Ambr(); got == nil || got.Uplink.Bps() != 20_000_000 || got.Downlink.Bps() != 40_000_000 {
		t.Fatalf("UE-AMBR = %+v, want the profile's 20/40 Mbps", got)
	}
}
