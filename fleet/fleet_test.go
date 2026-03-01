package fleet

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/db"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sampleStatus() client.EllaCoreStatus {
	return client.EllaCoreStatus{
		NetworkInterfaces: client.StatusNetworkInterfaces{
			N2:  client.N2Interface{Address: "10.0.0.1", Port: 38412},
			N3:  client.N3Interface{Name: "eth1", Address: "10.0.1.1"},
			N6:  client.N6Interface{Name: "eth2"},
			API: client.APIInterface{Address: "0.0.0.0", Port: 5000},
		},
		Radios: []client.Radio{
			{Name: "gnb-01", Address: "10.0.0.10"},
		},
		Subscribers: []client.SubscriberStatus{
			{Imsi: "001010000000001", IPAddress: "10.1.0.2", Registered: true},
		},
	}
}

func sampleMetrics() client.EllaCoreMetrics {
	return client.EllaCoreMetrics{
		UplinkBytesTotal:   1000,
		DownlinkBytesTotal: 2000,
		PDUSessionsTotal:   3,
	}
}

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeSyncer struct {
	calls    []*client.SyncParams
	response *client.SyncResponse
	err      error
}

func (f *fakeSyncer) Sync(_ context.Context, params *client.SyncParams) (*client.SyncResponse, error) {
	// Copy the params so callers' later mutations don't affect recorded calls.
	cp := *params

	if params.Status != nil {
		s := *params.Status
		cp.Status = &s
	}

	f.calls = append(f.calls, &cp)

	if f.err != nil {
		return nil, f.err
	}

	return f.response, nil
}

type fakeSyncDB struct {
	configApplied   *client.SyncConfig
	revisionUpdated *int64
	usageRows       []db.DailyUsage
	updateConfigErr error
}

func (f *fakeSyncDB) UpdateConfig(_ context.Context, cfg client.SyncConfig) error {
	if f.updateConfigErr != nil {
		return f.updateConfigErr
	}

	f.configApplied = &cfg

	return nil
}

func (f *fakeSyncDB) UpdateFleetConfigRevision(_ context.Context, rev int64) error {
	f.revisionUpdated = &rev
	return nil
}

func (f *fakeSyncDB) GetRawDailyUsage(_ context.Context, _, _ time.Time) ([]db.DailyUsage, error) {
	return f.usageRows, nil
}

func noConfigResponse() *client.SyncResponse {
	return &client.SyncResponse{Config: nil, ConfigRevision: 0}
}

func configResponse(rev int64) *client.SyncResponse {
	return &client.SyncResponse{
		Config: &client.SyncConfig{
			Operator: client.Operator{ID: client.OperatorID{Mcc: "001", Mnc: "01"}},
		},
		ConfigRevision: rev,
	}
}

func newTestRunner(fs *fakeSyncer, fdb *fakeSyncDB) *syncRunner {
	return &syncRunner{
		syncer:          fs,
		db:              fdb,
		statusProvider:  sampleStatus,
		metricsProvider: sampleMetrics,
		version:         "1.0.0-test",
		lastKnownRev:    10,
	}
}

// ---------------------------------------------------------------------------
// statusHash tests
// ---------------------------------------------------------------------------

func TestStatusHash_Deterministic(t *testing.T) {
	s := sampleStatus()
	h1 := statusHash(s)

	h2 := statusHash(s)
	if h1 != h2 {
		t.Fatal("statusHash produced different results for identical input")
	}
}

func TestStatusHash_DifferentInputs(t *testing.T) {
	s1 := sampleStatus()
	s2 := sampleStatus()
	s2.Subscribers = append(s2.Subscribers, client.SubscriberStatus{
		Imsi:       "001010000000002",
		IPAddress:  "10.1.0.3",
		Registered: false,
	})

	h1 := statusHash(s1)

	h2 := statusHash(s2)
	if h1 == h2 {
		t.Fatal("statusHash returned same hash for different inputs")
	}
}

// ---------------------------------------------------------------------------
// statusTracker tests
// ---------------------------------------------------------------------------

func TestStatusTracker_FirstCallAlwaysSendsStatus(t *testing.T) {
	var tracker statusTracker

	s := sampleStatus()

	got := tracker.Prepare(s)
	if got == nil {
		t.Fatal("expected status to be included on first call, got nil")
	}

	if got.Radios[0].Name != s.Radios[0].Name {
		t.Fatal("returned status does not match input")
	}
}

func TestStatusTracker_OmitsStatusWhenUnchanged(t *testing.T) {
	var tracker statusTracker

	s := sampleStatus()

	got := tracker.Prepare(s)
	if got == nil {
		t.Fatal("expected non-nil on first prepare")
	}

	tracker.Confirm(s)

	got = tracker.Prepare(s)
	if got != nil {
		t.Fatal("expected nil when status is unchanged, got non-nil")
	}
}

func TestStatusTracker_SendsStatusWhenChanged(t *testing.T) {
	var tracker statusTracker

	s := sampleStatus()

	tracker.Prepare(s)
	tracker.Confirm(s)

	s.Subscribers[0].IPAddress = "10.1.0.99"

	got := tracker.Prepare(s)
	if got == nil {
		t.Fatal("expected status to be included after change, got nil")
	}
}

func TestStatusTracker_ResendsOnFailure(t *testing.T) {
	var tracker statusTracker

	s := sampleStatus()

	tracker.Prepare(s)
	tracker.Confirm(s)

	s.Subscribers[0].Registered = false

	got := tracker.Prepare(s)
	if got == nil {
		t.Fatal("expected status to be included after change")
	}

	// Simulate sync failure: do NOT call Confirm.

	got = tracker.Prepare(s)
	if got == nil {
		t.Fatal("expected status to be resent after failed sync")
	}

	tracker.Confirm(s)

	got = tracker.Prepare(s)
	if got != nil {
		t.Fatal("expected nil after successful resend, got non-nil")
	}
}

func TestStatusTracker_OmitsAfterNoOpChange(t *testing.T) {
	var tracker statusTracker

	s := sampleStatus()

	tracker.Prepare(s)
	tracker.Confirm(s)

	s2 := sampleStatus()

	got := tracker.Prepare(s2)
	if got != nil {
		t.Fatal("expected nil when status content is identical, got non-nil")
	}
}

// ---------------------------------------------------------------------------
// syncRunner tests
// ---------------------------------------------------------------------------

func TestSyncRunner_FirstCycleIncludesStatus(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	runner := newTestRunner(fs, &fakeSyncDB{})

	runner.runOneCycle(context.Background())

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 sync call, got %d", len(fs.calls))
	}

	params := fs.calls[0]

	if params.Status == nil {
		t.Fatal("expected status to be included in first sync cycle")
	}

	if params.Version != "1.0.0-test" {
		t.Fatalf("expected version 1.0.0-test, got %s", params.Version)
	}

	if params.LastKnownRevision != 10 {
		t.Fatalf("expected revision 10, got %d", params.LastKnownRevision)
	}
}

func TestSyncRunner_UnchangedStatusOmittedOnSecondCycle(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	runner := newTestRunner(fs, &fakeSyncDB{})

	runner.runOneCycle(context.Background())
	runner.runOneCycle(context.Background())

	if len(fs.calls) != 2 {
		t.Fatalf("expected 2 sync calls, got %d", len(fs.calls))
	}

	if fs.calls[0].Status == nil {
		t.Fatal("first call should include status")
	}

	if fs.calls[1].Status != nil {
		t.Fatal("second call should omit unchanged status")
	}
}

func TestSyncRunner_ChangedStatusIncludedOnSecondCycle(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	fdb := &fakeSyncDB{}

	status := sampleStatus()
	runner := newTestRunner(fs, fdb)
	runner.statusProvider = func() client.EllaCoreStatus { return status }

	runner.runOneCycle(context.Background())

	// Change status before second cycle.
	status.Subscribers[0].IPAddress = "10.1.0.99"

	runner.runOneCycle(context.Background())

	if len(fs.calls) != 2 {
		t.Fatalf("expected 2 sync calls, got %d", len(fs.calls))
	}

	if fs.calls[1].Status == nil {
		t.Fatal("second call should include changed status")
	}

	if fs.calls[1].Status.Subscribers[0].IPAddress != "10.1.0.99" {
		t.Fatal("second call should carry the updated subscriber IP")
	}
}

func TestSyncRunner_AppliesConfigAndUpdatesRevision(t *testing.T) {
	fs := &fakeSyncer{response: configResponse(42)}
	fdb := &fakeSyncDB{}
	runner := newTestRunner(fs, fdb)

	runner.runOneCycle(context.Background())

	if fdb.configApplied == nil {
		t.Fatal("expected config to be applied to DB")
	}

	if fdb.configApplied.Operator.ID.Mcc != "001" {
		t.Fatalf("expected operator MCC 001, got %s", fdb.configApplied.Operator.ID.Mcc)
	}

	if fdb.revisionUpdated == nil || *fdb.revisionUpdated != 42 {
		t.Fatal("expected config revision 42 to be stored in DB")
	}

	if runner.lastKnownRev != 42 {
		t.Fatalf("expected runner revision to be updated to 42, got %d", runner.lastKnownRev)
	}
}

func TestSyncRunner_RevisionSentAfterConfigApply(t *testing.T) {
	fs := &fakeSyncer{response: configResponse(42)}
	fdb := &fakeSyncDB{}
	runner := newTestRunner(fs, fdb)
	runner.lastKnownRev = 10

	// First cycle: gets config revision 42.
	runner.runOneCycle(context.Background())

	// Second cycle should send revision 42.
	fs.response = noConfigResponse()

	runner.runOneCycle(context.Background())

	if len(fs.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(fs.calls))
	}

	if fs.calls[1].LastKnownRevision != 42 {
		t.Fatalf("expected second call to send revision 42, got %d", fs.calls[1].LastKnownRevision)
	}
}

func TestSyncRunner_NilConfigDoesNotUpdateRevision(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	fdb := &fakeSyncDB{}
	runner := newTestRunner(fs, fdb)

	runner.runOneCycle(context.Background())

	if fdb.configApplied != nil {
		t.Fatal("expected no config to be applied when response config is nil")
	}

	if fdb.revisionUpdated != nil {
		t.Fatal("expected revision not to be updated when response config is nil")
	}

	if runner.lastKnownRev != 10 {
		t.Fatalf("expected runner revision to remain 10, got %d", runner.lastKnownRev)
	}
}

func TestSyncRunner_FailedSyncCallsCallbackFalse(t *testing.T) {
	fs := &fakeSyncer{err: errors.New("network error")}
	fdb := &fakeSyncDB{}
	runner := newTestRunner(fs, fdb)

	var callbackResult *bool

	runner.onSync = func(_ context.Context, success bool) {
		callbackResult = &success
	}

	runner.runOneCycle(context.Background())

	if callbackResult == nil {
		t.Fatal("expected onSync callback to be called")
	}

	if *callbackResult {
		t.Fatal("expected onSync(false) on sync failure")
	}
}

func TestSyncRunner_SuccessfulSyncCallsCallbackTrue(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	runner := newTestRunner(fs, &fakeSyncDB{})

	var callbackResult *bool

	runner.onSync = func(_ context.Context, success bool) {
		callbackResult = &success
	}

	runner.runOneCycle(context.Background())

	if callbackResult == nil {
		t.Fatal("expected onSync callback to be called")
	}

	if !*callbackResult {
		t.Fatal("expected onSync(true) on sync success")
	}
}

func TestSyncRunner_FailedSyncDoesNotConfirmStatus(t *testing.T) {
	// First cycle succeeds, second fails, third should resend status.
	fs := &fakeSyncer{response: noConfigResponse()}
	fdb := &fakeSyncDB{}

	status := sampleStatus()
	runner := newTestRunner(fs, fdb)
	runner.statusProvider = func() client.EllaCoreStatus { return status }

	// Cycle 1: succeeds, status sent.
	runner.runOneCycle(context.Background())

	// Change status, then make sync fail.
	status.Radios = append(status.Radios, client.Radio{Name: "gnb-02", Address: "10.0.0.11"})
	fs.err = errors.New("timeout")

	runner.runOneCycle(context.Background())

	// Cycle 3: sync succeeds again — status should still be included
	// because the previous send was never confirmed.
	fs.err = nil
	fs.response = noConfigResponse()

	runner.runOneCycle(context.Background())

	if len(fs.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(fs.calls))
	}

	if fs.calls[2].Status == nil {
		t.Fatal("expected status to be resent after prior failed sync")
	}
}

func TestSyncRunner_IncludesUsageInParams(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	fdb := &fakeSyncDB{
		usageRows: []db.DailyUsage{
			{EpochDay: 20512, IMSI: "001010000000001", BytesUplink: 500, BytesDownlink: 1200},
			{EpochDay: 20513, IMSI: "001010000000002", BytesUplink: 300, BytesDownlink: 800},
		},
	}
	runner := newTestRunner(fs, fdb)

	runner.runOneCycle(context.Background())

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fs.calls))
	}

	usage := fs.calls[0].SubscriberUsage
	if len(usage) != 2 {
		t.Fatalf("expected 2 usage entries, got %d", len(usage))
	}

	if usage[0].IMSI != "001010000000001" || usage[0].UplinkBytes != 500 {
		t.Fatalf("unexpected first usage entry: %+v", usage[0])
	}

	if usage[1].IMSI != "001010000000002" || usage[1].DownlinkBytes != 800 {
		t.Fatalf("unexpected second usage entry: %+v", usage[1])
	}
}

func TestSyncRunner_MetricsAlwaysIncluded(t *testing.T) {
	fs := &fakeSyncer{response: noConfigResponse()}
	runner := newTestRunner(fs, &fakeSyncDB{})

	runner.runOneCycle(context.Background())
	runner.runOneCycle(context.Background())

	for i, call := range fs.calls {
		if call.Metrics.UplinkBytesTotal != 1000 {
			t.Fatalf("call %d: expected uplink_bytes_total 1000, got %d", i, call.Metrics.UplinkBytesTotal)
		}
	}
}

func TestSyncRunner_ConfigApplyErrorDoesNotUpdateRevision(t *testing.T) {
	fs := &fakeSyncer{response: configResponse(42)}
	fdb := &fakeSyncDB{updateConfigErr: errors.New("db error")}
	runner := newTestRunner(fs, fdb)

	runner.runOneCycle(context.Background())

	if fdb.revisionUpdated != nil {
		t.Fatal("revision should not be updated when config apply fails")
	}

	if runner.lastKnownRev != 10 {
		t.Fatalf("runner revision should remain 10 after config apply error, got %d", runner.lastKnownRev)
	}
}
