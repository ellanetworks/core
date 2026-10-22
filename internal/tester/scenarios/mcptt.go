// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	"github.com/ellanetworks/ims/sip"
	"github.com/ellanetworks/ims/sip/sdp"
	"github.com/ellanetworks/ims/sip/ua"
	"github.com/ellanetworks/ims/testue"
	"github.com/ellanetworks/ims/ue"
)

const (
	MCPTTICSI      = "urn:urn-7:3gpp-service.ims.icsi.mcptt"
	MCPTTService   = "mcptt"
	MCPTTPSIDomain = "mcptt." + IMSHomeDomain

	MCInternetPolicyName = "mcinternetpolicy"

	mcpttFeatureTag      = "+g.3gpp.mcptt"
	mcpttAudioPort       = 50000
	mcpttControlPort     = 50002
	mcpttClientFloorSSRC = 1001
)

func MCPTTPSI(user string) string {
	return "sip:" + user + "@" + MCPTTPSIDomain
}

func MCPTTSubscriber(imsi, msisdn string) SubscriberSpec {
	s := DefaultSubscriberWith(imsi, MCProfileName)
	s.MSISDN = msisdn

	return s
}

func MCPTTFixture(env Env, subs ...SubscriberSpec) FixtureSpec {
	fx := MCFixture(env, "")
	fx.Policies = append(fx.Policies, PolicySpec{
		Name:                MCInternetPolicyName,
		ProfileName:         MCProfileName,
		SliceName:           DefaultSliceName,
		DataNetworkName:     DefaultDNN,
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 15,
	})
	fx.Subscribers = subs
	fx.PCSCFAddresses = IMSServerAddresses

	return fx
}

func mcpttFeatures() sip.FeatureSet {
	return sip.FeatureSet{sip.NewFeature(mcpttFeatureTag)}
}

func mcpttICSIFeature() sip.FeatureSet {
	return sip.FeatureSet{sip.NewFeature("+g.3gpp.icsi-ref", MCPTTICSI)}
}

func mcpttService() ue.Service {
	return ue.Service{
		Name:     MCPTTService,
		ICSI:     MCPTTICSI,
		Features: mcpttFeatures(),
		AcceptContact: []sip.CallerPreference{
			{Features: mcpttFeatures(), Require: true, Explicit: true},
			{Features: mcpttICSIFeature(), Require: true, Explicit: true},
		},
		Allow: []string{"INVITE", "UPDATE"},
	}
}

type MCPTTClient struct {
	u         *imsUE
	sub       SubscriberSpec
	transport sip.Transport
}

func NewMCPTTClient(ep IMSEndpoint, transport sip.Transport) (*MCPTTClient, error) {
	u, err := newIMSUEConfigured(ep.Subscriber, ep.PCSCF, ep.Local, transport, 0, func(cfg *testue.Config) {
		cfg.NoMMTel = true
		cfg.NoSMS = true
		cfg.Services = []ue.Service{mcpttService()}
	})
	if err != nil {
		return nil, err
	}

	return &MCPTTClient{u: u, sub: ep.Subscriber, transport: transport}, nil
}

func (c *MCPTTClient) Close() { c.u.close() }

func (c *MCPTTClient) Register(ctx context.Context) error {
	if err := c.u.register(ctx, c.sub, c.transport); err != nil {
		return err
	}

	contact, err := c.u.RegisterContact()
	if err != nil {
		return fmt.Errorf("registered contact: %w", err)
	}

	fs, err := contact.Features()

	switch {
	case err != nil:
		return fmt.Errorf("feature tags of the registered contact %s: %w", contact, err)
	case !fs.Has("g.3gpp.mcptt"):
		return fmt.Errorf("the registered contact %s has no g.3gpp.mcptt feature tag (TS 24.379 §7.2.1)", contact)
	case !containsFold(fs.Tokens("g.3gpp.icsi-ref"), MCPTTICSI):
		return fmt.Errorf("the registered contact %s has no g.3gpp.icsi-ref with the MCPTT ICSI (TS 24.379 §7.2.1)", contact)
	}

	return nil
}

func (c *MCPTTClient) Deregister(ctx context.Context) error {
	return c.u.deregister(ctx, c.transport)
}

func containsFold(vs []string, want string) bool {
	for _, v := range vs {
		if strings.EqualFold(v, want) {
			return true
		}
	}

	return false
}

type MCPTTCall struct {
	media mcpttMedia

	mu       sync.Mutex
	session  *ua.Session
	response *sip.Response
	failure  error
	release  func()

	settleOnce sync.Once
	settled    chan struct{}
	endOnce    sync.Once
	done       chan struct{}
}

func (c *MCPTTClient) Call(ctx context.Context, target, priority string) (*MCPTTCall, error) {
	uri, err := sip.ParseURI(target)
	if err != nil {
		return nil, fmt.Errorf("target %q: %w", target, err)
	}

	req := sip.NewRequest("INVITE", uri)

	if err := c.u.Originate(req, MCPTTService); err != nil {
		return nil, fmt.Errorf("originate the MCPTT INVITE: %w", err)
	}

	req.Header.Add("To", "<"+uri.String()+">")
	req.Header.Add("Call-ID", c.u.NewCallID())

	if priority != "" {
		req.Header.Add("Resource-Priority", priority)
	}

	call := &MCPTTCall{
		media: mcpttMedia{
			addr: c.u.Local(), id: sdpSessionID(req.Header.CallID()), version: 1,
			audio: mcpttAudioPort, control: mcpttControlPort, pt: "96", rtpmap: "AMR-WB/16000/1",
			fmtp: "mode-change-capability=2;max-red=0", floorSSRC: mcpttClientFloorSSRC,
		},
		release: c.u.HoldSAs(),
		settled: make(chan struct{}),
		done:    make(chan struct{}),
	}

	req.SetBody(sdp.ContentType, call.media.sdp())

	if _, err := c.u.Agent().Invite(req, call.handler()); err != nil {
		call.release()
		return nil, fmt.Errorf("send the MCPTT INVITE: %w", err)
	}

	select {
	case <-call.settled:
	case <-ctx.Done():
		return nil, fmt.Errorf("the MCPTT INVITE to %s got no final response: %w", target, ctx.Err())
	}

	call.mu.Lock()
	defer call.mu.Unlock()

	if call.failure != nil {
		return nil, fmt.Errorf("MCPTT INVITE to %s: %w", target, call.failure)
	}

	return call, nil
}

func sdpSessionID(callID string) uint64 {
	var id uint64

	for _, b := range []byte(callID) {
		id = id*31 + uint64(b)
	}

	return id >> 2
}

func (c *MCPTTCall) handler() ua.Handler {
	return ua.Handler{
		Established: func(s *ua.Session, m sip.Message) {
			c.mu.Lock()
			c.session = s

			if res, ok := m.(*sip.Response); ok {
				c.response = res.Clone()
			}

			c.mu.Unlock()
			c.settle()
		},
		Offer: func(_ *ua.Session, o *ua.Offer) { o.Reject() },
		Request: func(_ *ua.Session, r *ua.Request) {
			if r.Refresh() {
				_ = r.AnswerUnchanged(ua.Change{})
				return
			}

			_ = r.Reject(488)
		},
		Ended: func(*ua.Session, ua.End) { c.end() },
		Failed: func(_ *ua.Outgoing, res *sip.Response, err error) {
			c.mu.Lock()

			switch {
			case err != nil:
				c.failure = err
			case res != nil:
				c.failure = &ue.ResponseError{Response: res.Clone()}
			default:
				c.failure = errors.New("the INVITE failed")
			}

			c.mu.Unlock()
			c.settle()
			c.end()
		},
	}
}

func (c *MCPTTCall) settle() {
	c.settleOnce.Do(func() {
		c.release()
		close(c.settled)
	})
}

func (c *MCPTTCall) end() {
	c.endOnce.Do(func() { close(c.done) })
}

func (c *MCPTTCall) Response() *sip.Response {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.response
}

func (c *MCPTTCall) Done() <-chan struct{} { return c.done }

func (c *MCPTTCall) Reprioritize(ctx context.Context, priority string) (*sip.Response, error) {
	c.mu.Lock()
	s := c.session
	c.media.version++
	offer := c.media.sdp()
	c.mu.Unlock()

	if s == nil {
		return nil, errors.New("re-INVITE of a call that is not established")
	}

	done := make(chan outcome, 1)
	change := ua.Change{
		Header: sip.Header{{Name: "Resource-Priority", Value: priority}},
		Bodies: []sip.BodyPart{ua.SDP(offer)},
	}

	if err := s.Reinvite(change, func(res *sip.Response, err error) { done <- outcome{res, err} }); err != nil {
		return nil, fmt.Errorf("send the re-INVITE: %w", err)
	}

	select {
	case o := <-done:
		switch {
		case o.err != nil:
			return nil, fmt.Errorf("re-INVITE: %w", o.err)
		case !o.res.IsSuccess():
			return o.res, fmt.Errorf("re-INVITE: %w", &ue.ResponseError{Response: o.res})
		}

		return o.res, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("the re-INVITE got no final response: %w", ctx.Err())
	}
}

func (c *MCPTTCall) Bye(ctx context.Context) error {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()

	if s == nil {
		return errors.New("BYE of a call that is not established")
	}

	done := make(chan error, 1)

	if err := s.Bye(ua.Change{}, func(_ *sip.Response, err error) { done <- err }); err != nil {
		return fmt.Errorf("send the BYE: %w", err)
	}

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("BYE: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("the BYE got no response: %w", ctx.Err())
	}

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("the call did not end after its BYE: %w", ctx.Err())
	}
}

type outcome struct {
	res *sip.Response
	err error
}

type mcpttMedia struct {
	addr      netip.Addr
	id        uint64
	version   int
	audio     uint16
	control   uint16
	pt        string
	rtpmap    string
	fmtp      string
	floorSSRC uint32
}

func (m mcpttMedia) sdp() []byte {
	ip := sdp.IP4
	if m.addr.Is6() {
		ip = sdp.IP6
	}

	lines := []string{
		"v=0",
		"o=- " + strconv.FormatUint(m.id, 10) + " " + strconv.Itoa(m.version) + " IN " + ip + " " + m.addr.String(),
		"s=-",
		"c=IN " + ip + " " + m.addr.String(),
		"t=0 0",
		"m=audio " + strconv.Itoa(int(m.audio)) + " RTP/AVP " + m.pt,
		"i=speech",
	}

	if m.rtpmap != "" {
		lines = append(lines, "a=rtpmap:"+m.pt+" "+m.rtpmap)
	}

	if m.fmtp != "" {
		lines = append(lines, "a=fmtp:"+m.pt+" "+m.fmtp)
	}

	lines = append(lines,
		"a=ptime:20",
		"a=maxptime:240",
		"a=sendrecv",
		"m=application "+strconv.Itoa(int(m.control))+" udp MCPTT",
		"a=fmtp:MCPTT mc_floor_ssrc="+strconv.FormatUint(uint64(m.floorSSRC), 10),
	)

	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}
