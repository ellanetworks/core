// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync"
	"syscall"
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
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

const (
	imsi          = "001010000000001"
	msisdn        = "15551230001"
	smsNumber     = "15550000010"
	serviceCentre = "15550000000"
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
		TC1Lost:            20 * time.Millisecond,
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
}

func newFakeStore(smsc netip.AddrPort) *fakeStore {
	return &fakeStore{
		settings: db.SMSSettings{SMSCAddress: smsc.Addr().String(), SMSCPort: int(smsc.Port()), SMSNumber: smsNumber},
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

type fakeSMSC struct {
	node *diameter.Node
	addr netip.AddrPort

	mu         sync.Mutex
	ofrs       []sgd.MOForwardShortMessage
	alerts     []s6c.Alert
	answer     func(req *diameter.Message, id diameter.Identity) *diameter.Message
	failAlerts bool
}

func requireSCTP(t *testing.T) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_SCTP)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("SCTP not available in CI: %v", err)
		}

		t.Skipf("SCTP not available: %v", err)
	}

	_ = syscall.Close(fd)
}

func startFakeSMSC(t *testing.T) *fakeSMSC {
	t.Helper()
	requireSCTP(t)

	f := &fakeSMSC{}
	mux := diameter.NewMux()

	mux.Handle(sgd.ApplicationID, sgd.CommandMOForwardShortMessage, diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		m, err := sgd.ParseMOForwardShortMessageRequest(req)
		if err != nil {
			return tgpp.NewErrorAnswer(req, c.LocalIdentity(), err)
		}

		f.mu.Lock()
		f.ofrs = append(f.ofrs, m)
		answer := f.answer
		f.mu.Unlock()

		if answer != nil {
			return answer(req, c.LocalIdentity())
		}

		ans, _ := sgd.NewMOForwardShortMessageAnswer(req, c.LocalIdentity(), []byte{0x01, 0x00})

		return ans
	}))

	mux.Handle(s6c.ApplicationID, s6c.CommandAlertServiceCentre, diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		a, err := s6c.ParseAlertServiceCentreRequest(req)
		if err != nil {
			return tgpp.NewErrorAnswer(req, c.LocalIdentity(), err)
		}

		f.mu.Lock()
		f.alerts = append(f.alerts, a)
		fail := f.failAlerts
		f.mu.Unlock()

		if fail {
			return tgpp.NewAnswer(req, c.LocalIdentity(), diameter.ResultUnableToComply)
		}

		return tgpp.NewAnswer(req, c.LocalIdentity(), diameter.ResultSuccess)
	}))

	node, err := diameter.New(diameter.Config{
		Identity: diameter.Identity{
			OriginHost: smscHost, OriginRealm: smscRealm,
			HostIPAddresses: []netip.Addr{loopback}, ProductName: "fake-smsc",
		},
		Handler:            mux,
		AcceptUnknownPeers: true,
		UnknownPeerApplications: []diameter.Application{
			{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
			{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
		},
	})
	if err != nil {
		t.Fatalf("new fake SMSC: %v", err)
	}

	var lc sctp.ListenConfig

	ln, err := lc.Listen(context.Background(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: loopback.AsSlice()}}})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = node.Serve(diameter.NewSCTPListener(ln, nil)) }()

	a, _ := ln.Addr().(*sctp.SCTPAddr)
	f.node = node
	f.addr = netip.AddrPortFrom(loopback, uint16(a.Port))

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = node.Shutdown(ctx)
		_ = ln.Close()
	})

	return f
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
		SessionID:        f.node.NewSessionID(),
		Origin:           f.node.Identity(),
		DestinationHost:  localIdent.Host,
		DestinationRealm: localIdent.Realm,
	}, m)
	if err != nil {
		t.Fatalf("build TFR: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	return f.node.DoHost(ctx, localIdent.Host, req)
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

	smsc := startFakeSMSC(t)
	store := newFakeStore(smsc.addr)

	nodeSettings := func(context.Context) (diameternode.NodeSettings, error) {
		return diameternode.NodeSettings{MCC: "001", MNC: "01", MMEGroupID: 0x8100, MMECode: 0x41}, nil
	}
	peers := func(context.Context) ([]diameternode.PeerConfig, error) {
		return []diameternode.PeerConfig{smsf.SMSCPeer(smsc.addr)}, nil
	}

	manager := diameternode.New(nodeSettings, peers, zap.NewNop())
	ue := newFakeUE()
	s := smsf.New(store, fakeDirectory{localNode: localIdent, remoteNode: remoteIdent}, manager, ue, zap.NewNop(), timers)
	s.Register(manager)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		manager.Run(ctx, make(chan struct{}))
		close(done)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	deadline := time.Now().Add(waitTimeout)

	for {
		peers := manager.Peers()
		if len(peers) == 1 && peers[0].State == diameter.PeerOpen {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("SMSC link did not come up")
		}

		time.Sleep(20 * time.Millisecond)
	}

	return &env{smsc: smsc, store: store, ue: ue, smsf: s}
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
