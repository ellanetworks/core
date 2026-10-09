// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/ims/sip"
	"github.com/ellanetworks/ims/sip/sdp"
	"github.com/ellanetworks/ims/testue"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	IMSDNN                = "ims"
	IMSUEIPv4Pool         = "10.60.0.0/16"
	IMSUEIPv6Pool         = "fd60::/48"
	IMSProfileName        = "imsprofile"
	IMSPolicyName         = "imspolicy"
	IMSPolicy5QI          = 5
	IMSInternetPolicyName = "imsinternetpolicy"
)

var PCSCFAddresses = []string{"2001:db8::5", "192.0.2.5", "192.0.2.6"}

func IMSDataNetwork(env Env) DataNetworkSpec {
	dn := DataNetworkSpec{Name: IMSDNN, IPv4Pool: IMSUEIPv4Pool, DNS: DefaultDNS, MTU: DefaultMTU}
	if env.HasIPv6() {
		dn.IPv6Pool = IMSUEIPv6Pool
	}

	return dn
}

func IMSPolicy(profile string) PolicySpec {
	return PolicySpec{
		Name:                IMSPolicyName,
		ProfileName:         profile,
		SliceName:           DefaultSliceName,
		DataNetworkName:     IMSDNN,
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              IMSPolicy5QI,
		Arp:                 15,
	}
}

func ExpectedPCSCFAddresses(env Env) []netip.Addr {
	var out []netip.Addr

	for _, s := range PCSCFAddresses {
		addr := netip.MustParseAddr(s)
		if (addr.Is4() && env.HasIPv4()) || (addr.Is6() && env.HasIPv6()) {
			out = append(out, addr)
		}
	}

	return out
}

func CheckPCSCFAddresses(got, want []netip.Addr) error {
	if !slices.Equal(got, want) {
		return fmt.Errorf("P-CSCF addresses = %v, want %v", got, want)
	}

	return nil
}

const (
	IMSHomeDomain    = "ims.mnc001.mcc001.3gppnetwork.org"
	imsTestUEIMEI    = "35693803564380"
	imsTestUETimeout = 90 * time.Second

	imsTerminationTimeout = 15 * time.Second
	imsCallAttemptTimeout = 3 * time.Second
	imsMediaLossGuard     = 6 * time.Second

	imsTestUEAheadSQN = 0x00000100001e
)

var IMSServerAddresses = []string{"fd00:6::5", "10.6.0.5"}

var IMSTransports = []sip.Transport{sip.UDP, sip.TCP}

func IMSSubscriber(imsi, msisdn string) SubscriberSpec {
	s := DefaultSubscriberWith(imsi, IMSProfileName)
	s.MSISDN = msisdn

	return s
}

func NonIMSSubscriber(imsi, msisdn string) SubscriberSpec {
	s := DefaultSubscriberWith(imsi, "")
	s.MSISDN = msisdn

	return s
}

func IMSFixture(env Env, subs ...SubscriberSpec) FixtureSpec {
	return FixtureSpec{
		Profiles: []ProfileSpec{{
			Name:           IMSProfileName,
			UeAmbrUplink:   DefaultProfileUeAmbrUplink,
			UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
		}},
		DataNetworks:   []DataNetworkSpec{IMSDataNetwork(env)},
		Policies:       []PolicySpec{IMSPolicy(IMSProfileName)},
		Subscribers:    subs,
		PCSCFAddresses: IMSServerAddresses,
	}
}

func IMSRoute(pcscf netip.Addr) string {
	return netip.PrefixFrom(pcscf, pcscf.BitLen()).String()
}

func IMSServerAddress(ipv6 bool) netip.Addr {
	for _, s := range IMSServerAddresses {
		if a := netip.MustParseAddr(s); a.Is6() == ipv6 {
			return a
		}
	}

	return netip.Addr{}
}

func SelectPCSCF(addrs []netip.Addr, ipv6 bool) (netip.Addr, error) {
	for _, a := range addrs {
		if a.Is6() == ipv6 {
			return a, nil
		}
	}

	return netip.Addr{}, fmt.Errorf("no P-CSCF address of the UE's family in %v", addrs)
}

func UEAddress(iface string, pool netip.Prefix) (netip.Addr, error) {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return netip.Addr{}, err
	}

	addrs, err := link.Addrs()
	if err != nil {
		return netip.Addr{}, err
	}

	for _, a := range addrs {
		p, err := netip.ParsePrefix(a.String())
		if err == nil && pool.Contains(p.Addr()) {
			return p.Addr(), nil
		}
	}

	return netip.Addr{}, fmt.Errorf("%s has no address in %s", iface, pool)
}

const IMSPCSCFPort = 5060

func RegisterIMS(ctx context.Context, sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport) error {
	_, err := registerIMS(ctx, sub, pcscf, local, transport, 0)

	return err
}

func RequireIMSTerminationOnDeletion(ctx context.Context, env Env, sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport) error {
	u, err := newIMSUE(sub, pcscf, local, transport, 0)
	if err != nil {
		return err
	}

	defer u.close()

	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	if err := u.register(ctx, sub, transport); err != nil {
		return err
	}

	cl, err := coreClient(env)
	if err != nil {
		return err
	}

	if err := cl.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: sub.IMSI}); err != nil {
		return fmt.Errorf("delete subscriber %s: %w", sub.IMSI, err)
	}

	deadline := time.Now().Add(imsTerminationTimeout)

	for u.State().Registered {
		if time.Now().After(deadline) {
			return fmt.Errorf("deleted subscriber %s is still registered in IMS", sub.IMSI)
		}

		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func coreClient(env Env) (*client.Client, error) {
	cl, err := client.New(&client.Config{BaseURL: env.APIAddress})
	if err != nil {
		return nil, fmt.Errorf("create core client: %w", err)
	}

	cl.SetToken(env.APIToken)

	return cl, nil
}

func RequireIMSSignallingLoss(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, release func() error) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	if err := release(); err != nil {
		return fmt.Errorf("release the ims session of %s: %w", callee.Subscriber.IMSI, err)
	}

	target := "tel:" + callee.Subscriber.MSISDN
	deadline := time.Now().Add(imsTerminationTimeout)

	for {
		code, err := callOutcome(ctx, a, target)
		if err != nil {
			return err
		}

		if code == 500 {
			break
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("calls to %s after its ims session was released end with %d, want 500 Server Internal Error", target, code)
		}

		time.Sleep(200 * time.Millisecond)
	}

	return a.deregister(ctx, transport)
}

func callOutcome(ctx context.Context, u *imsUE, target string) (int, error) {
	c, err := u.Invite(target, testue.CallOptions{})
	if err != nil {
		return 0, fmt.Errorf("invite %s: %w", target, err)
	}

	attempt, cancel := context.WithTimeout(ctx, imsCallAttemptTimeout)
	defer cancel()

	_, err = c.Wait(attempt)

	var rejected *testue.ResponseError

	switch {
	case err == nil:
		_ = c.Bye(ctx)
		return 200, nil
	case errors.As(err, &rejected):
		return rejected.Response.StatusCode, nil
	case attempt.Err() != nil && ctx.Err() == nil:
		_ = c.Cancel(ctx)
		return 0, nil
	default:
		return 0, fmt.Errorf("call to %s: %w", target, err)
	}
}

func RegisterIMSOutOfSync(ctx context.Context, sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport) error {
	got, err := registerIMS(ctx, sub, pcscf, local, transport, imsTestUEAheadSQN)
	if err != nil {
		return err
	}

	want, err := sqn.NextIMS(fmt.Sprintf("%012x", imsTestUEAheadSQN))
	if err != nil {
		return err
	}

	if s := fmt.Sprintf("%012x", got); s != want {
		return fmt.Errorf("SQN after resynchronisation = %s, want %s", s, want)
	}

	return nil
}

func RequireIMSRegistrationForbidden(ctx context.Context, sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport) error {
	_, err := registerIMS(ctx, sub, pcscf, local, transport, 0)

	var rejected *testue.ResponseError

	switch {
	case err == nil:
		return fmt.Errorf("IMS registration over %s succeeded, want 403 Forbidden", transport)
	case !errors.As(err, &rejected) || rejected.Response.StatusCode != 403:
		return fmt.Errorf("IMS registration over %s: %w, want 403 Forbidden", transport, err)
	}

	return nil
}

func registerIMS(ctx context.Context, sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport, sqnMS uint64) (uint64, error) {
	u, err := newIMSUE(sub, pcscf, local, transport, sqnMS)
	if err != nil {
		return 0, err
	}

	defer u.close()

	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	if err := u.register(ctx, sub, transport); err != nil {
		return 0, err
	}

	if err := u.deregister(ctx, transport); err != nil {
		return 0, err
	}

	return u.SQN(), nil
}

type imsUE struct {
	*testue.UE
	xfrm testue.XFRM
}

func newIMSUE(sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport, sqnMS uint64) (*imsUE, error) {
	return newIMSUEWith(sub, pcscf, local, transport, sqnMS, false)
}

func newIMSUEWith(sub SubscriberSpec, pcscf, local netip.Addr, transport sip.Transport, sqnMS uint64, video bool) (*imsUE, error) {
	k, err := hex.DecodeString(sub.Key)
	if err != nil {
		return nil, fmt.Errorf("decode K: %w", err)
	}

	opc, err := hex.DecodeString(sub.OPc)
	if err != nil {
		return nil, fmt.Errorf("decode OPc: %w", err)
	}

	xfrm, err := testue.OpenXFRM()
	if err != nil {
		return nil, err
	}

	u, err := testue.New(testue.Config{
		IMSI:        sub.IMSI,
		IMEI:        imsTestUEIMEI,
		K:           k,
		OPc:         opc,
		SQN:         sqnMS,
		PCSCF:       netip.AddrPortFrom(pcscf, IMSPCSCFPort),
		Local:       local,
		Transport:   transport,
		AcceptCalls: true,
		Video:       video,
		Kernel:      xfrm,
		Logger:      slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		_ = xfrm.Close()
		return nil, err
	}

	return &imsUE{UE: u, xfrm: xfrm}, nil
}

func (u *imsUE) close() {
	_ = u.Close()
	_ = u.xfrm.Close()
}

func (u *imsUE) register(ctx context.Context, sub SubscriberSpec, transport sip.Transport) error {
	if err := u.Register(ctx); err != nil {
		return fmt.Errorf("IMS registration of %s over %s: %w", sub.IMSI, transport, err)
	}

	return checkIMSRegistration(u.UE, sub)
}

func (u *imsUE) deregister(ctx context.Context, transport sip.Transport) error {
	if err := u.Deregister(ctx); err != nil {
		return fmt.Errorf("IMS deregistration over %s: %w", transport, err)
	}

	if u.State().Registered {
		return fmt.Errorf("the UE is still registered after deregistration over %s", transport)
	}

	return nil
}

type IMSEndpoint struct {
	Subscriber SubscriberSpec
	PCSCF      netip.Addr
	Local      netip.Addr
}

type IMSMedia struct {
	Caller sdp.Endpoint
	Callee sdp.Endpoint
}

func RequireIMSCall(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport) error {
	return RequireIMSCallWithMedia(ctx, caller, callee, transport, nil)
}

func RequireIMSCallWithMedia(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, media func(context.Context, IMSMedia) error) error {
	return requireIMSCall(ctx, caller, callee, transport, media, 0)
}

func RequireIMSCallKept(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, media func(context.Context, IMSMedia) error) error {
	return requireIMSCall(ctx, caller, callee, transport, media, imsMediaLossGuard)
}

func requireIMSCall(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, media func(context.Context, IMSMedia) error, keep time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	target := "tel:" + callee.Subscriber.MSISDN

	if err := call(ctx, a, b, target, media, keep); err != nil {
		return fmt.Errorf("call from %s to %s over %s: %w", caller.Subscriber.IMSI, target, transport, err)
	}

	if err := a.deregister(ctx, transport); err != nil {
		return err
	}

	return b.deregister(ctx, transport)
}

func RequireIMSCallEndedByNetwork(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	target := "tel:" + callee.Subscriber.MSISDN

	ac, err := a.Invite(target, testue.CallOptions{})
	if err != nil {
		return fmt.Errorf("invite: %w", err)
	}

	var bc *testue.Call

	select {
	case bc = <-b.Calls():
	case <-ac.Done():
		return networkEnded(ac)
	case <-ctx.Done():
		return fmt.Errorf("the callee received no INVITE: %w", ctx.Err())
	}

	if bc.Ring(ctx) == nil {
		_ = bc.Answer(ctx)
	}

	select {
	case <-ac.Done():
		return networkEnded(ac)
	case <-bc.Done():
		return networkEnded(bc)
	case <-ctx.Done():
		return fmt.Errorf("the call from %s to %s is still %s although its voice bearer failed: %w", caller.Subscriber.IMSI, target, ac.State(), ctx.Err())
	}
}

func RequireIMSCallLost(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, lose func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	ac, bc, err := establishCall(ctx, a, b, "tel:"+callee.Subscriber.MSISDN)
	if err != nil {
		return err
	}

	if err := lose(ctx); err != nil {
		return fmt.Errorf("lose the voice bearer: %w", err)
	}

	select {
	case <-ac.Done():
		return networkEnded(ac)
	case <-bc.Done():
		return networkEnded(bc)
	case <-ctx.Done():
		return fmt.Errorf("the call is still %s although its voice bearer was lost (IR.92 §2.4.2.3): %w", ac.State(), ctx.Err())
	}
}

func networkEnded(c *testue.Call) error {
	if end := c.End(); end == testue.LocalBye || end == testue.Cancelled {
		return fmt.Errorf("the call ended by %s, want the network to end it", end)
	}

	return nil
}

func callMedia(c *testue.Call) (IMSMedia, error) {
	local, remote := c.LocalSDP(), c.RemoteSDP()
	if local == nil || remote == nil {
		return IMSMedia{}, fmt.Errorf("the call has no negotiated SDP")
	}

	caller, err := local.RTPEndpoint(0)
	if err != nil {
		return IMSMedia{}, fmt.Errorf("caller media: %w", err)
	}

	callee, err := remote.RTPEndpoint(0)
	if err != nil {
		return IMSMedia{}, fmt.Errorf("callee media: %w", err)
	}

	return IMSMedia{Caller: caller, Callee: callee}, nil
}

func call(ctx context.Context, a, b *imsUE, target string, media func(context.Context, IMSMedia) error, keep time.Duration) error {
	ac, bc, err := establishCall(ctx, a, b, target)
	if err != nil {
		return err
	}

	if media != nil {
		m, err := callMedia(ac)
		if err != nil {
			return err
		}

		if err := media(ctx, m); err != nil {
			return fmt.Errorf("media: %w", err)
		}
	}

	if keep > 0 {
		select {
		case <-ac.Done():
			return fmt.Errorf("the call ended by %s within %s of the last media check, want it kept with no bearer loss reported", ac.End(), keep)
		case <-bc.Done():
			return fmt.Errorf("the call ended by %s within %s of the last media check, want it kept with no bearer loss reported", bc.End(), keep)
		case <-time.After(keep):
		}
	}

	return hangUp(ctx, ac, bc)
}

func establishCall(ctx context.Context, a, b *imsUE, target string) (*testue.Call, *testue.Call, error) {
	return establishCallWith(ctx, a, b, target, testue.CallOptions{})
}

func establishCallWith(ctx context.Context, a, b *imsUE, target string, opts testue.CallOptions) (*testue.Call, *testue.Call, error) {
	ac, err := a.Invite(target, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("invite: %w", err)
	}

	var bc *testue.Call

	select {
	case bc = <-b.Calls():
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("the callee received no INVITE: %w", ctx.Err())
	}

	if err := bc.Ring(ctx); err != nil {
		return nil, nil, fmt.Errorf("ring: %w", err)
	}

	answered := make(chan error, 1)

	go func() { answered <- bc.Answer(ctx) }()

	res, err := ac.Wait(ctx)

	switch {
	case err != nil:
		return nil, nil, fmt.Errorf("await the answer: %w", err)
	case res.StatusCode != 200:
		return nil, nil, fmt.Errorf("the INVITE was answered %d, want 200", res.StatusCode)
	}

	if err := <-answered; err != nil {
		return nil, nil, fmt.Errorf("answer: %w", err)
	}

	return ac, bc, nil
}

func hangUp(ctx context.Context, ac, bc *testue.Call) error {
	if err := ac.Bye(ctx); err != nil {
		return fmt.Errorf("bye: %w", err)
	}

	for _, c := range []*testue.Call{ac, bc} {
		select {
		case <-c.Done():
		case <-ctx.Done():
			return fmt.Errorf("the call is still %s after BYE: %w", c.State(), ctx.Err())
		}
	}

	return nil
}

type IMSTwoCallSteps struct {
	BothUp     func(ctx context.Context, first, second IMSMedia) error
	FirstEnded func(ctx context.Context, second IMSMedia) error
}

type IMSRejectedCallSteps struct {
	FirstUp        func(ctx context.Context, first IMSMedia) error
	AfterRejection func(ctx context.Context, first IMSMedia) error
}

func RequireIMSSecondCallRejected(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, steps IMSRejectedCallSteps) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	target := "tel:" + callee.Subscriber.MSISDN

	ac1, bc1, err := establishCall(ctx, a, b, target)
	if err != nil {
		return fmt.Errorf("first call: %w", err)
	}

	first, err := callMedia(ac1)
	if err != nil {
		return err
	}

	if err := steps.FirstUp(ctx, first); err != nil {
		return fmt.Errorf("first call up: %w", err)
	}

	ac2, err := a.Invite(target, testue.CallOptions{})
	if err != nil {
		return fmt.Errorf("second call: invite: %w", err)
	}

	select {
	case bc2 := <-b.Calls():
		if bc2.Ring(ctx) == nil {
			_ = bc2.Answer(ctx)
		}

		select {
		case <-ac2.Done():
		case <-bc2.Done():
		case <-ctx.Done():
			return fmt.Errorf("the second call is still %s although its bearer modification was rejected: %w", ac2.State(), ctx.Err())
		}
	case <-ac2.Done():
	case <-ctx.Done():
		return fmt.Errorf("the callee received no second INVITE: %w", ctx.Err())
	}

	if err := networkEnded(ac2); err != nil {
		return fmt.Errorf("second call: %w", err)
	}

	if ac1.State() == testue.CallTerminated {
		return fmt.Errorf("the first call ended with the second call's rejected modification (TS 29.212 §4.5.12)")
	}

	if err := steps.AfterRejection(ctx, first); err != nil {
		return fmt.Errorf("first call after the rejection: %w", err)
	}

	if err := hangUp(ctx, ac1, bc1); err != nil {
		return fmt.Errorf("first call: %w", err)
	}

	if err := a.deregister(ctx, transport); err != nil {
		return err
	}

	return b.deregister(ctx, transport)
}

func RequireIMSTwoCalls(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, steps IMSTwoCallSteps) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	target := "tel:" + callee.Subscriber.MSISDN

	ac1, bc1, err := establishCall(ctx, a, b, target)
	if err != nil {
		return fmt.Errorf("first call: %w", err)
	}

	ac2, bc2, err := establishCall(ctx, a, b, target)
	if err != nil {
		return fmt.Errorf("second call: %w", err)
	}

	first, err := callMedia(ac1)
	if err != nil {
		return err
	}

	second, err := callMedia(ac2)
	if err != nil {
		return err
	}

	if err := steps.BothUp(ctx, first, second); err != nil {
		return fmt.Errorf("both calls up: %w", err)
	}

	if err := hangUp(ctx, ac1, bc1); err != nil {
		return fmt.Errorf("first call: %w", err)
	}

	if err := steps.FirstEnded(ctx, second); err != nil {
		return fmt.Errorf("after the first call: %w", err)
	}

	if err := hangUp(ctx, ac2, bc2); err != nil {
		return fmt.Errorf("second call: %w", err)
	}

	if err := a.deregister(ctx, transport); err != nil {
		return err
	}

	return b.deregister(ctx, transport)
}

type IMSHoldSteps struct {
	Up      func(ctx context.Context, m IMSMedia) error
	Held    func(ctx context.Context, m IMSMedia) error
	Resumed func(ctx context.Context, m IMSMedia) error
}

func RequireIMSCallHeld(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, steps IMSHoldSteps) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	ac, bc, err := establishCall(ctx, a, b, "tel:"+callee.Subscriber.MSISDN)
	if err != nil {
		return err
	}

	m, err := callMedia(ac)
	if err != nil {
		return err
	}

	if err := steps.Up(ctx, m); err != nil {
		return fmt.Errorf("call up: %w", err)
	}

	if err := ac.Hold(ctx); err != nil {
		return fmt.Errorf("hold: %w", err)
	}

	if err := steps.Held(ctx, m); err != nil {
		return fmt.Errorf("on hold: %w", err)
	}

	if err := ac.Resume(ctx); err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	if err := steps.Resumed(ctx, m); err != nil {
		return fmt.Errorf("resumed: %w", err)
	}

	if err := hangUp(ctx, ac, bc); err != nil {
		return err
	}

	if err := a.deregister(ctx, transport); err != nil {
		return err
	}

	return b.deregister(ctx, transport)
}

func RTCPEndpoint(rtp sdp.Endpoint) sdp.Endpoint {
	rtp.Port++
	return rtp
}

func RerouteToIMS(iface string, local, dst netip.Addr, table int) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return err
	}

	if local.Is6() {
		addr := &netlink.Addr{IPNet: &net.IPNet{IP: local.AsSlice(), Mask: net.CIDRMask(64, 128)}, Flags: unix.IFA_F_NODAD}
		if err := netlink.AddrAdd(link, addr); err != nil && !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("keep %s on %s: %w", local, iface, err)
		}
	}

	route := &netlink.Route{LinkIndex: link.Attrs().Index, Scope: netlink.SCOPE_UNIVERSE, Dst: prefixNet(dst), Table: table}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("route %s via %s in table %d: %w", dst, iface, table, err)
	}

	return nil
}

func RouteFromUE(iface string, local, dst netip.Addr, table int) (func(), error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return nil, err
	}

	route := &netlink.Route{LinkIndex: link.Attrs().Index, Scope: netlink.SCOPE_UNIVERSE, Dst: prefixNet(dst), Table: table}
	if err := netlink.RouteReplace(route); err != nil {
		return nil, fmt.Errorf("route %s via %s in table %d: %w", dst, iface, table, err)
	}

	rule := netlink.NewRule()
	rule.Src = prefixNet(local)
	rule.Table = table

	if local.Is6() {
		rule.Family = netlink.FAMILY_V6
	}

	if err := netlink.RuleAdd(rule); err != nil {
		return nil, fmt.Errorf("rule from %s to table %d: %w", local, table, err)
	}

	return func() { _ = netlink.RuleDel(rule) }, nil
}

func prefixNet(a netip.Addr) *net.IPNet {
	return &net.IPNet{IP: a.AsSlice(), Mask: net.CIDRMask(a.BitLen(), a.BitLen())}
}

func checkIMSRegistration(u *testue.UE, sub SubscriberSpec) error {
	impi := sub.IMSI + "@" + IMSHomeDomain
	impu := "tel:" + sub.MSISDN
	s := u.State()

	switch {
	case u.IMPI() != impi:
		return fmt.Errorf("IMPI = %q, want %q", u.IMPI(), impi)
	case s.DefaultIMPU != impu:
		return fmt.Errorf("default IMPU = %q, want %q", s.DefaultIMPU, impu)
	case !slices.Contains(s.AssociatedURIs, impu):
		return fmt.Errorf("associated URIs %v do not include %q", s.AssociatedURIs, impu)
	}

	return nil
}

type IMSCall struct {
	Caller *testue.Call
	Callee *testue.Call
}

func (c IMSCall) Media(kind string) (IMSMedia, error) {
	s, ok := c.Caller.Stream(kind)

	switch {
	case !ok:
		return IMSMedia{}, fmt.Errorf("the call has no active %s stream", kind)
	case !s.Remote.Addr.IsValid():
		return IMSMedia{}, fmt.Errorf("the callee sent no endpoint for the %s stream", kind)
	}

	return IMSMedia{Caller: s.Local, Callee: s.Remote}, nil
}

func RequireIMSVideoCall(ctx context.Context, caller, callee IMSEndpoint, transport sip.Transport, video bool, script func(context.Context, IMSCall) error) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUEWith(caller.Subscriber, caller.PCSCF, caller.Local, transport, 0, true)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUEWith(callee.Subscriber, callee.PCSCF, callee.Local, transport, 0, true)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, caller.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, callee.Subscriber, transport); err != nil {
		return err
	}

	target := "tel:" + callee.Subscriber.MSISDN

	ac, bc, err := establishCallWith(ctx, a, b, target, testue.CallOptions{Video: video})
	if err != nil {
		return fmt.Errorf("video call from %s to %s: %w", caller.Subscriber.IMSI, target, err)
	}

	if err := script(ctx, IMSCall{Caller: ac, Callee: bc}); err != nil {
		return fmt.Errorf("video call from %s to %s: %w", caller.Subscriber.IMSI, target, err)
	}

	select {
	case <-ac.Done():
		return fmt.Errorf("the call ended by %s within %s of the last check, want it kept", ac.End(), imsMediaLossGuard)
	case <-bc.Done():
		return fmt.Errorf("the call ended by %s within %s of the last check, want it kept", bc.End(), imsMediaLossGuard)
	case <-time.After(imsMediaLossGuard):
	}

	if err := hangUp(ctx, ac, bc); err != nil {
		return err
	}

	if err := a.deregister(ctx, transport); err != nil {
		return err
	}

	return b.deregister(ctx, transport)
}
