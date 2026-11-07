// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fakehss

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/core/internal/udm"
)

var ErrDown = errors.New("fake HSS down")

type Subscriber struct {
	K            string
	OPc          string
	SQN          string
	Subscription s6a.SubscriptionData
}

type Node struct {
	Local diameter.Identity
	HSS   diameter.Identity

	mu          sync.Mutex
	subscribers map[string]*Subscriber
	airError    error
	ulrError    error
	ulrAnswer   func(s6a.UpdateLocation) s6a.UpdateLocation
	purError    error
	puaFlags    s6a.PUAFlags
	norError    error
	gates       map[uint32]chan struct{}
	down        bool
	sessions    int
	requests    []*diameter.Message
}

func New(local, hss diameter.Identity) *Node {
	return &Node{Local: local, HSS: hss, subscribers: map[string]*Subscriber{}}
}

func (n *Node) Provision(imsi string, s Subscriber) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.subscribers[imsi] = &s
}

func (n *Node) FailAIR(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.airError = err
}

func (n *Node) FailULR(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.ulrError = err
}

func (n *Node) RewriteULA(f func(s6a.UpdateLocation) s6a.UpdateLocation) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.ulrAnswer = f
}

func (n *Node) FailPUR(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.purError = err
}

func (n *Node) SetPUAFlags(f s6a.PUAFlags) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.puaFlags = f
}

func (n *Node) FailNOR(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.norError = err
}

func (n *Node) Hold(command uint32) func() {
	gate := make(chan struct{})

	n.mu.Lock()
	if n.gates == nil {
		n.gates = map[uint32]chan struct{}{}
	}

	n.gates[command] = gate
	n.mu.Unlock()

	var once sync.Once

	return func() {
		once.Do(func() {
			n.mu.Lock()
			delete(n.gates, command)
			n.mu.Unlock()
			close(gate)
		})
	}
}

func (n *Node) SetDown(down bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.down = down
}

func (n *Node) Requests() []*diameter.Message {
	n.mu.Lock()
	defer n.mu.Unlock()

	return append([]*diameter.Message(nil), n.requests...)
}

func (n *Node) Identity() diameter.Identity {
	return n.Local
}

func (n *Node) NewSessionID() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.sessions++

	return fmt.Sprintf("%s;1;%d", n.Local.OriginHost, n.sessions)
}

func (n *Node) Send(ctx context.Context, req *diameter.Message, _ ...diameter.RequestOption) (*diameter.Message, error) {
	b, err := req.Marshal()
	if err != nil {
		return nil, err
	}

	wire, err := diameter.Unmarshal(b)
	if err != nil {
		return nil, err
	}

	n.mu.Lock()
	n.requests = append(n.requests, wire)
	down := n.down
	gate := n.gates[wire.CommandCode]
	n.mu.Unlock()

	if down {
		return nil, ErrDown
	}

	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	var ans *diameter.Message

	switch wire.CommandCode {
	case s6a.CommandAuthenticationInformation:
		ans, err = n.authenticationInformation(ctx, wire)
	case s6a.CommandUpdateLocation:
		ans, err = n.updateLocation(wire)
	case s6a.CommandPurgeUE:
		ans, err = n.purgeUE(wire)
	case s6a.CommandNotify:
		ans, err = n.notify(wire)
	default:
		ans = s6a.NewAnswer(wire, n.HSS, tgpp.Result{Code: diameter.ResultCommandUnsupported}, s6a.Features{})
	}

	if err != nil {
		return nil, err
	}

	out, err := ans.Marshal()
	if err != nil {
		return nil, err
	}

	return diameter.Unmarshal(out)
}

func (n *Node) authenticationInformation(ctx context.Context, req *diameter.Message) (*diameter.Message, error) {
	air, err := s6a.ParseAuthenticationInformationRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, n.HSS, err, s6a.Features{}), nil
	}

	n.mu.Lock()
	_, ok := n.subscribers[air.IMSI]
	failure := n.airError
	n.mu.Unlock()

	switch {
	case failure != nil:
		return s6a.NewErrorAnswer(req, n.HSS, failure, s6a.Features{}), nil
	case !ok:
		return s6a.NewErrorAnswer(req, n.HSS, &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}, s6a.Features{}), nil
	case air.EUTRAN == nil:
		return s6a.NewErrorAnswer(req, n.HSS, &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultAuthenticationDataUnavailable)}, s6a.Features{}), nil
	}

	var auts, rand string
	if r := air.EUTRAN.Resync; r != nil {
		auts, rand = hex.EncodeToString(r.AUTS), hex.EncodeToString(r.RAND)
	}

	plmn, err := air.VisitedPLMN.MarshalBinary()
	if err != nil {
		return nil, err
	}

	vec, err := udm.New(&credentials{node: n, imsi: air.IMSI}, nil).GenerateEPSVector(ctx, air.IMSI, plmn, auts, rand)
	if err != nil {
		return s6a.NewErrorAnswer(req, n.HSS, &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultAuthenticationDataUnavailable)}, s6a.Features{}), nil
	}

	return s6a.NewAuthenticationInformationAnswer(req, n.HSS, s6a.AuthenticationInformation{
		EUTRAN: []s6a.EUTRANVector{{ItemNumber: 1, RAND: vec.RAND[:], XRES: vec.XRES, AUTN: vec.AUTN[:], KASME: vec.KASME}},
	})
}

func (n *Node) updateLocation(req *diameter.Message) (*diameter.Message, error) {
	ulr, err := s6a.ParseUpdateLocationRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, n.HSS, err, s6a.Features{}), nil
	}

	n.mu.Lock()
	sub, ok := n.subscribers[ulr.IMSI]
	failure := n.ulrError
	rewrite := n.ulrAnswer
	n.mu.Unlock()

	switch {
	case failure != nil:
		return s6a.NewErrorAnswer(req, n.HSS, failure, s6a.Features{}), nil
	case !ok:
		return s6a.NewErrorAnswer(req, n.HSS, &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}, s6a.Features{}), nil
	}

	subscription := sub.Subscription
	ula := s6a.UpdateLocation{Flags: s6a.ULAFlagSeparationIndication, Features: ulr.Features, Subscription: &subscription}

	if rewrite != nil {
		ula = rewrite(ula)
	}

	return s6a.NewUpdateLocationAnswer(req, n.HSS, ula)
}

func (n *Node) purgeUE(req *diameter.Message) (*diameter.Message, error) {
	pur, err := s6a.ParsePurgeUERequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, n.HSS, err, s6a.Features{}), nil
	}

	n.mu.Lock()
	_, ok := n.subscribers[pur.IMSI]
	failure := n.purError
	flags := n.puaFlags
	n.mu.Unlock()

	switch {
	case failure != nil:
		return s6a.NewErrorAnswer(req, n.HSS, failure, s6a.Features{}), nil
	case !ok:
		return s6a.NewErrorAnswer(req, n.HSS, &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}, s6a.Features{}), nil
	}

	return s6a.NewPurgeUEAnswer(req, n.HSS, s6a.PurgeUE{Flags: flags})
}

func (n *Node) notify(req *diameter.Message) (*diameter.Message, error) {
	nor, err := s6a.ParseNotifyRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, n.HSS, err, s6a.Features{}), nil
	}

	n.mu.Lock()
	_, ok := n.subscribers[nor.IMSI]
	failure := n.norError
	n.mu.Unlock()

	switch {
	case failure != nil:
		return s6a.NewErrorAnswer(req, n.HSS, failure, s6a.Features{}), nil
	case !ok:
		return s6a.NewErrorAnswer(req, n.HSS, &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}, s6a.Features{}), nil
	}

	return s6a.NewNotifyAnswer(req, n.HSS, s6a.Notify{})
}

func (n *Node) envelope() tgpp.Envelope {
	return tgpp.Envelope{
		SessionID:        n.NewSessionID(),
		Origin:           n.HSS,
		DestinationHost:  n.Local.OriginHost,
		DestinationRealm: n.Local.OriginRealm,
	}
}

func roundTrip(req *diameter.Message, err error) (*diameter.Message, error) {
	if err != nil {
		return nil, err
	}

	b, err := req.Marshal()
	if err != nil {
		return nil, err
	}

	return diameter.Unmarshal(b)
}

func (n *Node) CancelLocationRequest(r s6a.CancelLocationRequest) (*diameter.Message, error) {
	return roundTrip(s6a.NewCancelLocationRequest(n.envelope(), r))
}

func (n *Node) InsertSubscriberDataRequest(r s6a.InsertSubscriberDataRequest) (*diameter.Message, error) {
	return roundTrip(s6a.NewInsertSubscriberDataRequest(n.envelope(), r))
}

func (n *Node) DeleteSubscriberDataRequest(r s6a.DeleteSubscriberDataRequest) (*diameter.Message, error) {
	return roundTrip(s6a.NewDeleteSubscriberDataRequest(n.envelope(), r))
}

func (n *Node) ResetRequest(r s6a.ResetRequest) (*diameter.Message, error) {
	return roundTrip(s6a.NewResetRequest(n.envelope(), r))
}

func Answer(ans *diameter.Message) (*diameter.Message, error) {
	return roundTrip(ans, nil)
}

type credentials struct {
	node *Node
	imsi string
}

func (c *credentials) AdvanceSequenceNumber(_ context.Context, imsi, resyncAuts, resyncRand string) (*udm.AdvancedCredentials, error) {
	c.node.mu.Lock()
	defer c.node.mu.Unlock()

	sub, ok := c.node.subscribers[imsi]
	if !ok {
		return nil, udm.ErrSubscriberUnknown
	}

	next, err := sqn.Next(sub.SQN, sub.OPc, sub.K, resyncAuts, resyncRand)
	if err != nil {
		return nil, err
	}

	sub.SQN = next

	return &udm.AdvancedCredentials{PermanentKey: sub.K, Opc: sub.OPc, SequenceNumber: next}, nil
}
