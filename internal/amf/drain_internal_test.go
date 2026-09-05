// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

type drainCaptureConn struct {
	mu   sync.Mutex
	sent [][]byte
}

func (c *drainCaptureConn) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	c.mu.Lock()
	c.sent = append(c.sent, append([]byte(nil), b...))
	c.mu.Unlock()

	return len(b), nil
}

func (c *drainCaptureConn) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.sent)
}

func (c *drainCaptureConn) snapshot() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([][]byte(nil), c.sent...)
}

func trackDrainTestRadio(a *AMF, conn NGAPWriter) *Radio {
	r := trackPreSetupDrainTestRadio(a, conn)

	capacity := a.RelativeCapacity()

	a.mu.Lock()
	defer a.mu.Unlock()

	r.RanPresent = RanPresentGNbID
	r.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "000102"}}
	r.advertisedCapacity = &capacity

	return r
}

func trackPreSetupDrainTestRadio(a *AMF, conn NGAPWriter) *Radio {
	r := &Radio{Conn: conn, amf: a, Log: logger.AmfLog}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.reg.Track(conn, r)

	return r
}

func drainTestRanNodeID() ngap.GlobalRANNodeID {
	return ngap.GlobalRANNodeID{
		Kind:         ngap.RANNodeIDGNB,
		PLMNIdentity: ngap.PLMNIdentity{0x00, 0xf1, 0x10},
		Value:        0x000102,
		Bits:         24,
	}
}

func parseAMFConfigUpdate(t *testing.T, pdu []byte) *ngap.AMFConfigurationUpdate {
	t.Helper()

	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcAMFConfigurationUpdate {
		t.Fatalf("got %T, want AMF Configuration Update", msg)
	}

	out, err := ngap.ParseAMFConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return out
}

func TestSetEligibleAdvertisesAZeroWeightFactor(t *testing.T) {
	a := New(nil, nil, nil)

	cc := &drainCaptureConn{}
	trackDrainTestRadio(a, cc)

	a.setRelativeCapacity(DrainedRelativeCapacity)

	if notified := a.notifyRelativeCapacity(context.Background(), nil); notified != 1 {
		t.Fatalf("notified %d gNBs, want 1", notified)
	}

	sent := cc.snapshot()
	if len(sent) != 1 {
		t.Fatalf("gNB received %d messages, want 1", len(sent))
	}

	update := parseAMFConfigUpdate(t, sent[0])
	if update.RelativeAMFCapacity == nil || *update.RelativeAMFCapacity != 0 {
		t.Fatalf("RelativeAMFCapacity = %v, want 0", update.RelativeAMFCapacity)
	}
}

func TestUnchangedCapacityIsNotReadvertised(t *testing.T) {
	a := New(nil, nil, nil)

	cc := &drainCaptureConn{}
	trackDrainTestRadio(a, cc)

	a.setRelativeCapacity(DrainedRelativeCapacity)

	for range 5 {
		a.notifyRelativeCapacity(context.Background(), nil)
	}

	if got := cc.count(); got != 1 {
		t.Fatalf("%d updates for one capacity change, want 1: reconciling re-advertises an unchanged capacity", got)
	}
}

func TestFailureClearsTheAdvertisedCapacitySoTheBackstopRetries(t *testing.T) {
	a := New(nil, nil, nil)

	cc := &drainCaptureConn{}
	radio := trackDrainTestRadio(a, cc)

	a.setRelativeCapacity(DrainedRelativeCapacity)
	a.notifyRelativeCapacity(context.Background(), nil)
	a.ConfigUpdateFailed(context.Background(), radio, 0)
	a.notifyRelativeCapacity(context.Background(), nil)

	if got := cc.count(); got != 2 {
		t.Fatalf("%d updates, want 2: a rejected update was not re-sent on the next reconcile", got)
	}
}

func TestTimeToWaitHoldsOffTheNextUpdate(t *testing.T) {
	a := New(nil, nil, nil)

	cc := &drainCaptureConn{}
	radio := trackDrainTestRadio(a, cc)

	a.setRelativeCapacity(DrainedRelativeCapacity)
	a.notifyRelativeCapacity(context.Background(), nil)
	a.ConfigUpdateFailed(context.Background(), radio, time.Minute)
	a.notifyRelativeCapacity(context.Background(), nil)

	if got := cc.count(); got != 1 {
		t.Fatalf("%d updates, want 1: the AMF reinitiated inside the Time To Wait", got)
	}
}

func TestConfigUpdateStateClearsOnDisconnect(t *testing.T) {
	a := New(nil, nil, nil)

	cc := &drainCaptureConn{}
	radio := trackDrainTestRadio(a, cc)

	a.mu.Lock()
	radio.retryNotBefore = time.Now().Add(time.Hour)
	a.mu.Unlock()

	a.DisconnectRadio(context.Background(), radio)

	a.mu.RLock()
	defer a.mu.RUnlock()

	if radio.advertisedCapacity != nil || !radio.retryNotBefore.IsZero() {
		t.Fatal("a dropped association kept its configuration-update state")
	}
}

func TestDrainDoesNotReleaseUEs(t *testing.T) {
	var a any = New(nil, nil, nil)

	if _, ok := a.(interface {
		Offload(ctx context.Context, batch int) int
	}); ok {
		t.Fatal("the AMF offloads its UEs on drain; TS 23.501 §5.21.2.2 leaves that to the 5G-AN")
	}
}

type drainTestDB struct {
	operator *db.Operator
	nodeID   int
}

func (d *drainTestDB) GetOperator(context.Context) (*db.Operator, error) {
	return d.operator, nil
}

func (d *drainTestDB) GetSubscriber(context.Context, string) (*db.Subscriber, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) GetDataNetworkByID(context.Context, string) (*db.DataNetwork, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) GetNetworkSliceByID(context.Context, string) (*db.NetworkSlice, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) ListNetworkSlicesByIDs(context.Context, []string) ([]db.NetworkSlice, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) GetProfileByID(context.Context, string) (*db.Profile, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) GetPolicyByProfileAndSlice(context.Context, string, string) (*db.Policy, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) ListAllNetworkSlices(context.Context) ([]db.NetworkSlice, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) ListPoliciesByProfile(context.Context, string) ([]db.Policy, error) {
	return nil, errNotImplementedInDrainTest
}

func (d *drainTestDB) NodeID() int { return d.nodeID }

var errNotImplementedInDrainTest = errors.New("not implemented in drain test")

func drainTestAMF() *AMF {
	return New(&drainTestDB{
		operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`, AmfRegionID: 2, AmfSetID: 1},
		nodeID:   3,
	}, nil, nil)
}

func TestResumeReadvertisesTheServedGUAMI(t *testing.T) {
	a := drainTestAMF()

	cc := &drainCaptureConn{}
	trackDrainTestRadio(a, cc)

	a.SetEligible(context.Background(), false)
	a.SetEligible(context.Background(), true)

	sent := cc.snapshot()

	resume := parseAMFConfigUpdate(t, sent[len(sent)-1])
	if resume.RelativeAMFCapacity == nil || *resume.RelativeAMFCapacity != DefaultRelativeCapacity {
		t.Fatalf("RelativeAMFCapacity = %v, want %d", resume.RelativeAMFCapacity, DefaultRelativeCapacity)
	}

	if len(resume.ServedGUAMIList) != 1 {
		t.Fatalf("resume carried %d served GUAMIs, want 1: the gNB keeps the GUAMI unavailable", len(resume.ServedGUAMIList))
	}

	want, err := util.GUAMIToNGAP(*mustOperatorGUAMI(t, a))
	if err != nil {
		t.Fatalf("encode expected GUAMI: %v", err)
	}

	if resume.ServedGUAMIList[0].GUAMI != want {
		t.Fatalf("served GUAMI = %+v, want %+v", resume.ServedGUAMIList[0].GUAMI, want)
	}
}

func TestDrainDoesNotReadvertiseTheServedGUAMI(t *testing.T) {
	a := drainTestAMF()

	cc := &drainCaptureConn{}
	trackDrainTestRadio(a, cc)

	a.SetEligible(context.Background(), false)

	drainUpdate := parseAMFConfigUpdate(t, cc.snapshot()[0])
	if len(drainUpdate.ServedGUAMIList) != 0 {
		t.Fatal("the drain update re-advertised the GUAMI it is about to mark unavailable")
	}
}

func mustOperatorGUAMI(t *testing.T, a *AMF) *models.Guami {
	t.Helper()

	info, err := a.OperatorInfo(context.Background())
	if err != nil {
		t.Fatalf("OperatorInfo: %v", err)
	}

	return info.Guami
}

func TestDrainSkipsAssociationsWithoutNGSetup(t *testing.T) {
	a := drainTestAMF()

	cc := &drainCaptureConn{}
	trackPreSetupDrainTestRadio(a, cc)

	if notified := a.SetEligible(context.Background(), false); notified != 0 {
		t.Fatalf("notified %d gNBs, want 0", notified)
	}

	if got := cc.count(); got != 0 {
		t.Fatalf("sent %d messages before NG Setup completed; TS 38.413 §8.7.1.1 makes NG Setup the first NGAP procedure on the association", got)
	}
}

func statusIndications(t *testing.T, sent [][]byte) int {
	t.Helper()

	count := 0

	for _, pdu := range sent {
		msg, err := ngap.Unmarshal(pdu)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if im, ok := msg.(*ngap.InitiatingMessage); ok && im.ProcedureCode == ngap.ProcAMFStatusIndication {
			count++
		}
	}

	return count
}

func TestGUAMIUnavailableIsSentOncePerDrain(t *testing.T) {
	a := drainTestAMF()

	cc := &drainCaptureConn{}
	trackDrainTestRadio(a, cc)

	for range 5 {
		a.SetEligible(context.Background(), false)
	}

	if got := statusIndications(t, cc.snapshot()); got != 1 {
		t.Fatalf("%d AMF Status Indications for one drain, want 1: reconciling repeats a request to release the NGAP UE-TNLA bindings (TS 23.501 §5.21.2.2.2)", got)
	}
}

func TestGUAMIUnavailableIsResentAfterResume(t *testing.T) {
	a := drainTestAMF()

	cc := &drainCaptureConn{}
	trackDrainTestRadio(a, cc)

	a.SetEligible(context.Background(), false)
	a.SetEligible(context.Background(), true)
	a.SetEligible(context.Background(), false)

	if got := statusIndications(t, cc.snapshot()); got != 2 {
		t.Fatalf("%d AMF Status Indications across two drains, want 2", got)
	}
}

func TestGUAMIUnavailableIsResentAfterNGSetup(t *testing.T) {
	a := drainTestAMF()

	cc := &drainCaptureConn{}
	radio := trackDrainTestRadio(a, cc)

	a.SetEligible(context.Background(), false)

	a.ClaimRanID(radio, drainTestRanNodeID(), DefaultRelativeCapacity)

	a.SetEligible(context.Background(), false)

	if got := statusIndications(t, cc.snapshot()); got != 2 {
		t.Fatalf("%d AMF Status Indications, want 2: NG Setup erases application level configuration data (TS 38.413 §8.7.1.1), so the drained GUAMI must be re-announced", got)
	}
}

func TestSetupAcceptRecordsTheCapacityTheResponseCarried(t *testing.T) {
	a := drainTestAMF()

	a.setRelativeCapacity(DrainedRelativeCapacity)

	cc := &drainCaptureConn{}
	radio := trackPreSetupDrainTestRadio(a, cc)

	a.setRelativeCapacity(DefaultRelativeCapacity)

	a.ClaimRanID(radio, drainTestRanNodeID(), DefaultRelativeCapacity)

	a.setRelativeCapacity(DrainedRelativeCapacity)

	if n := a.notifyRelativeCapacity(context.Background(), nil); n != 1 {
		t.Fatalf("notified %d gNBs, want 1: de-duplication used a capacity the gNB was never sent", n)
	}

	update := parseAMFConfigUpdate(t, cc.snapshot()[0])
	if update.RelativeAMFCapacity == nil || *update.RelativeAMFCapacity != DrainedRelativeCapacity {
		t.Fatalf("RelativeAMFCapacity = %v, want %d", update.RelativeAMFCapacity, DrainedRelativeCapacity)
	}
}
