// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/ims/sip"
	"github.com/ellanetworks/ims/sip/sdp"
	"github.com/ellanetworks/ims/sip/transaction"
	"github.com/ellanetworks/ims/sip/ua"
)

const (
	MCPTTServerPort = 5090

	mcpttServerAudioPort    = 52000
	mcpttServerControlPort  = 52002
	mcpttServerFloorSSRC    = 2002
	mcpttServerShutdownTime = 5 * time.Second
)

type MCPTTServer struct {
	agent *ua.Agent
	local netip.AddrPort

	mu       sync.Mutex
	changed  chan struct{}
	priority string
	received []*sip.Request
}

func StartMCPTTServer(ctx context.Context, env Env) (*MCPTTServer, error) {
	addr, err := netip.ParseAddr(env.FirstGNB().N2Address)
	if err != nil {
		return nil, fmt.Errorf("MCPTT server address %q: %w", env.FirstGNB().N2Address, err)
	}

	s := &MCPTTServer{local: netip.AddrPortFrom(addr, MCPTTServerPort), changed: make(chan struct{})}

	params, err := append(mcpttICSIFeature(), mcpttFeatures()...).Params()
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})).With("component", "mcptt-server")

	agent, err := ua.New(ua.Config{
		Transaction: transaction.Config{Logger: logger},
		Prepare:     s.prepare,
		Contact:     sip.Address{URI: s.uri(), Params: params},
		Allow:       append(slices.Clone(ua.Methods), "REGISTER"),
		OnInvite:    s.onInvite,
		OnRequest:   s.onRequest,
		Logger:      logger,
	})
	if err != nil {
		return nil, fmt.Errorf("MCPTT server agent: %w", err)
	}

	s.agent = agent

	if _, err := agent.Layer().Listen(ctx, s.local); err != nil {
		_ = agent.Shutdown(ctx)
		return nil, fmt.Errorf("MCPTT server: listen on %s: %w", s.local, err)
	}

	return s, nil
}

func (s *MCPTTServer) uri() sip.URI {
	return sip.URI{Scheme: "sip", Host: sip.FormatHost(s.local.Addr()), Port: s.local.Port()}
}

func (s *MCPTTServer) URI() string { return s.uri().String() }

func (s *MCPTTServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), mcpttServerShutdownTime)
	defer cancel()

	_ = s.agent.Shutdown(ctx)
}

func (s *MCPTTServer) SetResourcePriority(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.priority = v
}

func (s *MCPTTServer) Requests(method string) []*sip.Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*sip.Request

	for _, r := range s.received {
		if r.Method == method {
			out = append(out, r)
		}
	}

	return out
}

func (s *MCPTTServer) AwaitRequest(ctx context.Context, method string, n int) (*sip.Request, error) {
	for {
		s.mu.Lock()
		changed := s.changed
		s.mu.Unlock()

		if rs := s.Requests(method); len(rs) >= n {
			return rs[n-1], nil
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return nil, fmt.Errorf("the MCPTT server received %d %s requests, want %d: %w", len(s.Requests(method)), method, n, ctx.Err())
		}
	}
}

func (s *MCPTTServer) record(req *sip.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.received = append(s.received, req.Clone())
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *MCPTTServer) prepare(req *sip.Request) error {
	target := req.URI

	switch route, err := req.Header.TopRoute(); {
	case err == nil:
		target = route.URI
	case !errors.Is(err, sip.ErrMissingHeader):
		return err
	}

	transport, dst, err := sip.Destination(target)
	if err != nil {
		return err
	}

	req.Flow = sip.Flow{Transport: transport, Local: s.local, Remote: dst}
	req.Header.Prepend("Via", sip.NewVia(transport, s.local).String())

	return nil
}

func (s *MCPTTServer) onRequest(r *ua.Request) {
	req := r.Message()
	s.record(req)

	switch req.Method {
	case "REGISTER":
		_ = r.Respond(ua.Reply{Code: 200})
	case "OPTIONS":
		_ = r.Respond(ua.Reply{Code: 200, Header: s.agent.Capabilities()})
	default:
		_ = r.Reject(405, sip.Field{Name: "Allow", Value: s.agent.Capabilities().Get("Allow")})
	}
}

func (s *MCPTTServer) onInvite(r *ua.Request) ua.Handler {
	s.record(r.Message())

	h := ua.Handler{
		Request: s.inDialog,
		Received: func(_ *ua.Session, m sip.Message) {
			if req, ok := m.(*sip.Request); ok {
				s.record(req)
			}
		},
		Cancelled: func(_ *ua.Session, _ *ua.Request, cancel *sip.Request) { s.record(cancel) },
	}

	r.Session().SetHandler(h)

	offer, ok := r.Offer()
	if !ok {
		_ = r.Reject(488)
		return h
	}

	answer, err := mcpttAnswer(s.local.Addr(), offer)
	if err != nil {
		_ = r.Reject(488, sip.Field{Name: "Warning", Value: `399 ` + sip.FormatHost(s.local.Addr()) + ` "` + err.Error() + `"`})
		return h
	}

	r.Session().SetValue(answer)

	_ = r.Respond(ua.Reply{Code: 200, Header: s.responseHeader(r.Message()), Bodies: []sip.BodyPart{ua.SDP(answer.sdp())}})

	return h
}

func (s *MCPTTServer) inDialog(session *ua.Session, r *ua.Request) {
	req := r.Message()

	if r.Refresh() {
		_ = r.AnswerUnchanged(ua.Change{Header: s.responseHeader(req)})
		return
	}

	offer, ok := r.Offer()
	if !ok {
		_ = r.Respond(ua.Reply{Code: 200, Header: s.responseHeader(req)})
		return
	}

	answer, err := mcpttAnswer(s.local.Addr(), offer)
	if err != nil {
		_ = r.Reject(488)
		return
	}

	if previous, ok := session.Value().(mcpttMedia); ok {
		answer = previous
	}

	_ = r.Respond(ua.Reply{Code: 200, Header: s.responseHeader(req), Bodies: []sip.BodyPart{ua.SDP(answer.sdp())}})
}

func (s *MCPTTServer) responseHeader(req *sip.Request) sip.Header {
	caps, _ := sip.FeatureCapsValue(mcpttICSIFeature())
	h := sip.Header{{Name: "Feature-Caps", Value: caps}}

	s.mu.Lock()
	priority := s.priority
	s.mu.Unlock()

	if priority == "" {
		priority = strings.Join(req.Header.Values("Resource-Priority"), ", ")
	}

	if priority != "" {
		h = append(h, sip.Field{Name: "Resource-Priority", Value: priority})
	}

	return h
}

func mcpttAnswer(addr netip.Addr, offer []byte) (mcpttMedia, error) {
	o, err := sdp.Parse(offer)
	if err != nil {
		return mcpttMedia{}, fmt.Errorf("SDP offer: %w", err)
	}

	if len(o.Media) != 2 {
		return mcpttMedia{}, fmt.Errorf("SDP offer with %d media, want MCPTT speech and the media plane control channel", len(o.Media))
	}

	audio, err := o.Media[0].Desc()
	if err != nil || audio.Type != sdp.Audio || len(audio.Formats) == 0 {
		return mcpttMedia{}, errors.New("SDP offer without MCPTT speech first (TS 24.379 §6.2.1)")
	}

	control, err := o.Media[1].Desc()
	if err != nil || !strings.EqualFold(control.Type, "application") || !strings.EqualFold(control.Proto, "udp") ||
		!slices.ContainsFunc(control.Formats, func(f string) bool { return strings.EqualFold(f, "MCPTT") }) {
		return mcpttMedia{}, errors.New("SDP offer without the media plane control channel second (TS 24.380 §4.3.3.1)")
	}

	m := mcpttMedia{
		addr: addr, id: sdpSessionID(string(offer)), version: 1,
		audio: mcpttServerAudioPort, control: mcpttServerControlPort, pt: audio.Formats[0], floorSSRC: mcpttServerFloorSSRC,
	}

	if maps, err := o.Media[0].RTPMaps(); err == nil {
		for _, rm := range maps {
			if strconv.Itoa(int(rm.Payload)) == m.pt {
				m.rtpmap = rm.Encoding + "/" + strconv.FormatUint(uint64(rm.ClockRate), 10)
				if rm.Params != "" {
					m.rtpmap += "/" + rm.Params
				}
			}
		}
	}

	if f, ok := o.Media[0].Fmtp(m.pt); ok {
		m.fmtp = f
	}

	return m, nil
}

const thirdPartyRegisterTimeout = 10 * time.Second

func RequireThirdPartyRegister(ctx context.Context, s *MCPTTServer, sub SubscriberSpec) error {
	ctx, cancel := context.WithTimeout(ctx, thirdPartyRegisterTimeout)
	defer cancel()

	for n := 1; ; n++ {
		req, err := s.AwaitRequest(ctx, "REGISTER", n)
		if err != nil {
			return fmt.Errorf("no third-party REGISTER of %s at the MCPTT application server (TS 24.229 §5.4.1.7): %w", sub.IMSI, err)
		}

		to, err := req.Header.To()
		if err != nil || !strings.Contains(to.URI.String(), strings.TrimPrefix(sub.MSISDN, "+")) && !strings.Contains(to.URI.String(), sub.IMSI) || req.Header.Get("Expires") == "0" {
			continue
		}

		if _, ok, _ := req.Content().Find("message/sip"); !ok {
			return fmt.Errorf("the third-party REGISTER of %s has no message/sip body, want the REGISTER and its 200 (initial filter criteria with the request and response included)", sub.IMSI)
		}

		return nil
	}
}

func RequireMCPTTInvite(ctx context.Context, as *MCPTTServer, n int, priority string) error {
	req, err := as.AwaitRequest(ctx, "INVITE", n)
	if err != nil {
		return err
	}

	what := "INVITE"
	if n > 1 {
		what = "re-INVITE"
	}

	switch {
	case !strings.EqualFold(strings.TrimSpace(req.Header.Get("P-Asserted-Service")), MCPTTICSI) && n == 1:
		return fmt.Errorf("the %s at the MCPTT server has P-Asserted-Service %q, want %q (TS 24.229 §5.4.3.2)", what, req.Header.Get("P-Asserted-Service"), MCPTTICSI)
	case n == 1 && req.URI.String() != MCPTTPSI("grp-1"):
		return fmt.Errorf("the %s at the MCPTT server has Request-URI %s, want the PSI %s", what, req.URI, MCPTTPSI("grp-1"))
	case req.Header.Get("Resource-Priority") != priority:
		return fmt.Errorf("the %s at the MCPTT server has Resource-Priority %q, want %q", what, req.Header.Get("Resource-Priority"), priority)
	}

	return nil
}

func RequireAuthorisedPriority(res *sip.Response, priority string) error {
	if res == nil {
		return errors.New("no response")
	}

	if got := res.Header.Get("Resource-Priority"); got != priority {
		return fmt.Errorf("Resource-Priority %q, want the MCPTT server's %q", got, priority)
	}

	caps, err := res.Header.FeatureCaps()
	if err != nil || !slices.ContainsFunc(caps, func(fs sip.FeatureSet) bool {
		return slices.Contains(fs.Tokens("g.3gpp.icsi-ref"), MCPTTICSI)
	}) {
		return fmt.Errorf("Feature-Caps %q, want the MCPTT ICSI (TS 29.214 Annex A.1 NOTE 3)", res.Header.Values("Feature-Caps"))
	}

	return nil
}
