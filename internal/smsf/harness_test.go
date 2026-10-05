// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/smsf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

const (
	imsi          = "001010000000001"
	msisdn        = "15551230001"
	smsNumber     = "15550000010"
	serviceCentre = "15550000000"
	smscPeerID    = "a"
	smscHost      = "smsc.example.org"
	smscRealm     = "example.org"
	localNode     = "node-a"
	remoteNode    = "node-b"
	waitTimeout   = 10 * time.Second
)

var (
	loopback    = netip.MustParseAddr("127.0.0.1")
	localIdent  = diameternode.Identity{Host: "mmec41.mmegi8100.mme.epc.mnc001.mcc001.3gppnetwork.org", Realm: "epc.mnc001.mcc001.3gppnetwork.org"}
	remoteIdent = diameternode.Identity{Host: "mmec42.mmegi8100.mme.epc.mnc001.mcc001.3gppnetwork.org", Realm: "epc.mnc001.mcc001.3gppnetwork.org"}
)

func fastTimers() smsf.Timers {
	return smsf.Timers{
		TC1:                200 * time.Millisecond,
		Paging:             2 * time.Second,
		MoreMessages:       300 * time.Millisecond,
		MaxRetransmissions: 2,
		TR1N:               3 * time.Second,
		TR2N:               3 * time.Second,
		AlertTimeout:       2 * time.Second,
	}
}

type fakeStore struct {
	mu            sync.Mutex
	settings      db.SMSSettings
	settingsErr   error
	subscribers   map[string]db.Subscriber
	registrations map[[2]string]db.UERegistration
	regErr        error
	waiting       map[string]*db.SMSWaiting
	clearErr      error
	peers         []db.SMSCPeer
	peersErr      error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		settings: db.SMSSettings{Enabled: true, SMSNumber: smsNumber},
		peers:    []db.SMSCPeer{{ID: smscPeerID, Address: "192.0.2.10", Port: 3868, ServiceCentres: []string{serviceCentre}}},
		subscribers: map[string]db.Subscriber{
			imsi: {Imsi: imsi, Msisdn: msisdn},
		},
		registrations: make(map[[2]string]db.UERegistration),
		waiting:       make(map[string]*db.SMSWaiting),
	}
}

func (f *fakeStore) GetSMSWaiting(_ context.Context, imsi string) (*db.SMSWaiting, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	w, ok := f.waiting[imsi]
	if !ok {
		return nil, db.ErrNotFound
	}

	out := *w
	out.ServiceCentres = slices.Clone(w.ServiceCentres)

	return &out, nil
}

func (f *fakeStore) RecordSMSWaiting(_ context.Context, u db.SMSWaitingUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.subscribers[u.IMSI]; !ok {
		return db.ErrNotFound
	}

	w, ok := f.waiting[u.IMSI]
	if !ok {
		w = &db.SMSWaiting{IMSI: u.IMSI}
		f.waiting[u.IMSI] = w
	}

	w.MemoryFull = w.MemoryFull || u.MemoryFull

	if !slices.Contains(w.ServiceCentres, u.ServiceCentre) {
		w.ServiceCentres = append(w.ServiceCentres, u.ServiceCentre)
		slices.Sort(w.ServiceCentres)
	}

	return nil
}

func (f *fakeStore) ClearSMSMemoryFull(_ context.Context, imsi string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.clearErr != nil {
		return f.clearErr
	}

	if w, ok := f.waiting[imsi]; ok {
		w.MemoryFull = false
	}

	return nil
}

func (f *fakeStore) RemoveSMSWaitingCentre(_ context.Context, imsi, serviceCentre string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	w, ok := f.waiting[imsi]
	if !ok {
		return nil
	}

	w.ServiceCentres = slices.DeleteFunc(w.ServiceCentres, func(sc string) bool { return sc == serviceCentre })
	w.MemoryFull = false

	if len(w.ServiceCentres) == 0 {
		delete(f.waiting, imsi)
	}

	return nil
}

func (f *fakeStore) DeleteSMSWaiting(_ context.Context, imsi string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.waiting, imsi)

	return nil
}

func (f *fakeStore) ListSMSCPeers(context.Context) ([]db.SMSCPeer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.peers), f.peersErr
}

func (f *fakeStore) setPeers(peers ...db.SMSCPeer) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.peers = peers
}

func (f *fakeStore) setPeersErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.peersErr = err
}

func (f *fakeStore) GetSMSSettings(context.Context) (*db.SMSSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s := f.settings

	return &s, f.settingsErr
}

func (f *fakeStore) GetSubscriber(_ context.Context, imsi string) (*db.Subscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.subscribers[imsi]
	if !ok {
		return nil, db.ErrNotFound
	}

	return &s, nil
}

func (f *fakeStore) IMSIsWithMSISDN(_ context.Context, imsis []string) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	found := map[string]bool{}

	for _, imsi := range imsis {
		if s, ok := f.subscribers[imsi]; ok && s.Msisdn != "" {
			found[imsi] = true
		}
	}

	return found, nil
}

func (f *fakeStore) GetSubscriberByMSISDN(_ context.Context, msisdn string) (*db.Subscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, s := range f.subscribers {
		if s.Msisdn != "" && s.Msisdn == msisdn {
			return &s, nil
		}
	}

	return nil, db.ErrNotFound
}

func (f *fakeStore) GetUERegistration(_ context.Context, imsi, regType string) (*db.UERegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.regErr != nil {
		return nil, f.regErr
	}

	r, ok := f.registrations[[2]string{imsi, regType}]
	if !ok {
		return nil, db.ErrNotFound
	}

	return &r, nil
}

func (f *fakeStore) register(regType, nodeID string, purged bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.registrations[[2]string{imsi, regType}] = db.UERegistration{Imsi: imsi, Type: regType, NodeID: nodeID, Purged: purged, Version: 1}
}

func (f *fakeStore) registerVersion(regType, nodeID string, version int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.registrations[[2]string{imsi, regType}] = db.UERegistration{Imsi: imsi, Type: regType, NodeID: nodeID, Version: version}
}

func (f *fakeStore) setMSISDN(v string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s := f.subscribers[imsi]
	s.Msisdn = v
	f.subscribers[imsi] = s
}

type fakeDirectory map[string]diameternode.Identity

const unreadableNode = "node-unreadable"

func (d fakeDirectory) Identity(_ context.Context, nodeID string) (diameternode.Identity, error) {
	if nodeID == unreadableNode {
		return diameternode.Identity{}, errors.New("get operator: leader changed")
	}

	id, ok := d[nodeID]
	if !ok {
		return diameternode.Identity{}, smsf.ErrNotClusterMember
	}

	return id, nil
}

type fakeUE struct {
	mu       sync.Mutex
	err      error
	reached  int
	settled  int
	order    []string
	gate     chan struct{}
	downlink chan sms.CPMessage
}

func newFakeUE() *fakeUE {
	return &fakeUE{downlink: make(chan sms.CPMessage, 64)}
}

func (u *fakeUE) EnableUEReachability(ctx context.Context, _ string) error {
	u.mu.Lock()
	u.reached++
	err, gate := u.err, u.gate
	u.mu.Unlock()

	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return err
}

func (u *fakeUE) holdReachability() chan struct{} {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.gate = make(chan struct{})

	return u.gate
}

func (u *fakeUE) SignallingSettled(context.Context, string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.settled++
	u.order = append(u.order, "settled")
}

func (u *fakeUE) events() []string {
	u.mu.Lock()
	defer u.mu.Unlock()

	return slices.Clone(u.order)
}

func (u *fakeUE) settlements() int {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.settled
}

func (u *fakeUE) reachability() int {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.reached
}

func (u *fakeUE) SendSMS(_ context.Context, _ string, payload []byte) error {
	u.mu.Lock()
	err := u.err
	u.mu.Unlock()

	if err != nil {
		return err
	}

	m, parseErr := sms.ParseCP(payload)
	if parseErr != nil {
		return parseErr
	}

	u.mu.Lock()
	u.order = append(u.order, m.MessageType().String())
	u.mu.Unlock()

	u.downlink <- m

	return nil
}

func (u *fakeUE) fail(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.err = err
}

func (u *fakeUE) next(t *testing.T) sms.CPMessage {
	t.Helper()

	select {
	case m := <-u.downlink:
		return m
	case <-time.After(waitTimeout):
		t.Fatal("timed out waiting for a downlink CP message")
		return nil
	}
}

func (u *fakeUE) nextData(t *testing.T) (*sms.CPData, sms.RPMessage) {
	t.Helper()

	for {
		m := u.next(t)

		data, ok := m.(*sms.CPData)
		if !ok {
			continue
		}

		rp, err := sms.ParseRP(data.UserData, nas.DirectionDownlink)
		if err != nil {
			t.Fatalf("downlink RP: %v", err)
		}

		return data, rp
	}
}

func (u *fakeUE) expectNothing(t *testing.T, within time.Duration) {
	t.Helper()

	select {
	case m := <-u.downlink:
		t.Fatalf("unexpected downlink %s", m.MessageType())
	case <-time.After(within):
	}
}

func encode(t *testing.T, m interface{ MarshalBinary() ([]byte, error) }) []byte {
	t.Helper()

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	return b
}

var (
	localIdentity = diameter.Identity{OriginHost: localIdent.Host, OriginRealm: localIdent.Realm}
	smscIdentity  = diameter.Identity{OriginHost: smscHost, OriginRealm: smscRealm}
	errSMSCDown   = errors.New("fake SMSC: no route")
)

type fakeSMSC struct {
	mu         sync.Mutex
	smsf       *smsf.SMSF
	down       bool
	downPeers  map[string]bool
	targets    []string
	sessions   int
	ofrs       []sgd.MOForwardShortMessage
	alerts     []s6c.Alert
	answer     func(req *diameter.Message, id diameter.Identity) *diameter.Message
	failAlerts bool
}

func (f *fakeSMSC) Envelope(peer string) (tgpp.Envelope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.down || f.downPeers[peer] {
		return tgpp.Envelope{}, smsf.ErrSMSCUnavailable
	}

	f.sessions++

	return tgpp.Envelope{
		SessionID:        fmt.Sprintf("%s;1;%d", localIdent.Host, f.sessions),
		Origin:           localIdentity,
		DestinationHost:  smscHost,
		DestinationRealm: smscRealm,
	}, nil
}

func (f *fakeSMSC) LocalHost() (string, bool) {
	return localIdent.Host, true
}

func (f *fakeSMSC) Do(_ context.Context, peer string, req *diameter.Message) (*diameter.Message, error) {
	f.mu.Lock()
	down := f.down || f.downPeers[peer]
	f.targets = append(f.targets, peer)
	f.mu.Unlock()

	if down {
		return nil, errSMSCDown
	}

	switch req.CommandCode {
	case sgd.CommandMOForwardShortMessage:
		return f.moForwardShortMessage(req), nil
	case s6c.CommandAlertServiceCentre:
		return f.alertServiceCentre(req), nil
	default:
		return nil, fmt.Errorf("fake SMSC: unexpected command %d", req.CommandCode)
	}
}

func (f *fakeSMSC) moForwardShortMessage(req *diameter.Message) *diameter.Message {
	m, err := sgd.ParseMOForwardShortMessageRequest(req)
	if err != nil {
		return tgpp.NewErrorAnswer(req, smscIdentity, err)
	}

	f.mu.Lock()
	f.ofrs = append(f.ofrs, m)
	answer := f.answer
	f.mu.Unlock()

	if answer != nil {
		return answer(req, smscIdentity)
	}

	ans, _ := sgd.NewMOForwardShortMessageAnswer(req, smscIdentity, []byte{0x01, 0x00})

	return ans
}

func (f *fakeSMSC) alertServiceCentre(req *diameter.Message) *diameter.Message {
	a, err := s6c.ParseAlertServiceCentreRequest(req)
	if err != nil {
		return tgpp.NewErrorAnswer(req, smscIdentity, err)
	}

	f.mu.Lock()
	f.alerts = append(f.alerts, a)
	fail := f.failAlerts
	f.mu.Unlock()

	if fail {
		return tgpp.NewAnswer(req, smscIdentity, diameter.ResultUnableToComply)
	}

	return tgpp.NewAnswer(req, smscIdentity, diameter.ResultSuccess)
}

func (f *fakeSMSC) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.down = down
}

func (f *fakeSMSC) setPeerDown(peer string, down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.downPeers == nil {
		f.downPeers = make(map[string]bool)
	}

	f.downPeers[peer] = down
}

func (f *fakeSMSC) sentTo() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.targets)
}

func (f *fakeSMSC) setAnswer(answer func(req *diameter.Message, id diameter.Identity) *diameter.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.answer = answer
}

func (f *fakeSMSC) submitted() []sgd.MOForwardShortMessage {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]sgd.MOForwardShortMessage(nil), f.ofrs...)
}

func (f *fakeSMSC) setFailAlerts(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failAlerts = fail
}

func (f *fakeSMSC) alerted() []s6c.Alert {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]s6c.Alert(nil), f.alerts...)
}

func (f *fakeSMSC) forward(t *testing.T, tpdu []byte) (*diameter.Message, error) {
	t.Helper()

	return f.forwardMessage(t, sgd.MTForwardShortMessage{IMSI: imsi, ServiceCentreAddress: serviceCentre, SMRPUI: tpdu})
}

func (f *fakeSMSC) forwardMessage(t *testing.T, m sgd.MTForwardShortMessage) (*diameter.Message, error) {
	t.Helper()

	req, err := sgd.NewMTForwardShortMessageRequest(tgpp.Envelope{
		SessionID:        smscHost + ";1;1",
		Origin:           smscIdentity,
		DestinationHost:  localIdent.Host,
		DestinationRealm: localIdent.Realm,
	}, m)
	if err != nil {
		t.Fatalf("build TFR: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	return f.smsf.MTForwardShortMessage(ctx, localIdentity, req), nil
}

type env struct {
	smsc  *fakeSMSC
	store *fakeStore
	ue    *fakeUE
	smsf  *smsf.SMSF
}

func newEnv(t *testing.T) *env {
	t.Helper()

	return newEnvWithTimers(t, fastTimers())
}

func newEnvWithTimers(t *testing.T, timers smsf.Timers) *env {
	t.Helper()

	smsc := &fakeSMSC{}
	store := newFakeStore()
	ue := newFakeUE()
	smsc.smsf = smsf.New(store, fakeDirectory{localNode: localIdent, remoteNode: remoteIdent}, smsc, ue, zap.NewNop(), timers)

	return &env{smsc: smsc, store: store, ue: ue, smsf: smsc.smsf}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(waitTimeout)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(20 * time.Millisecond)
	}
}
