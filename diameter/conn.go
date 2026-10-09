// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errConnClosed  = errors.New("diameter: connection closed")
	errConnSuspect = errors.New("diameter: connection suspect")
)

type connState int32

const (
	stateWaitCER connState = iota
	stateWaitCEA
	stateParked
	stateOpen
	stateClosing
)

type watchdogStatus int32

const (
	watchdogOkay watchdogStatus = iota
	watchdogSuspect
	watchdogReopen
)

type Conn struct {
	n         *Node
	t         transport
	peer      *peer
	initiator bool

	ctx    context.Context
	cancel context.CancelFunc

	state     atomic.Int32
	watchdog  atomic.Int32
	available atomic.Bool
	reopen    bool
	dprCause  atomic.Int64
	dprHbH    atomic.Uint32
	hopByHop  atomic.Uint32
	cerHbH    uint32

	peerHost     string
	peerRealm    string
	localApps    []Application
	localVendors []uint32
	common       map[uint32]bool
	parkedCER    *Message
	lastError    atomic.Pointer[string]

	pendingSlot bool
	unordered   atomic.Bool
	requests    chan struct{}

	mu        sync.Mutex
	pending   map[uint32]chan *Message
	suspected chan struct{}
	suspect   bool

	activity    chan struct{}
	watchdogDWA chan struct{}
	opened      chan struct{}
	done        chan struct{}
	openOnce    sync.Once
	doneOnce    sync.Once

	timerMu sync.Mutex
	timer   *time.Timer
}

func newConn(n *Node, t transport, p *peer) *Conn {
	ctx, cancel := context.WithCancel(n.baseCtx) // #nosec G118 -- cancelled when the connection closes

	c := &Conn{
		n:            n,
		t:            t,
		peer:         p,
		initiator:    p != nil,
		ctx:          ctx,
		cancel:       cancel,
		localApps:    n.cfg.UnknownPeerApplications,
		localVendors: n.cfg.UnknownPeerSupportedVendors,
		requests:     make(chan struct{}, n.cfg.MaxConcurrentRequests),
		pending:      make(map[uint32]chan *Message),
		suspected:    make(chan struct{}),
		activity:     make(chan struct{}, 1),
		watchdogDWA:  make(chan struct{}, 1),
		opened:       make(chan struct{}),
		done:         make(chan struct{}),
	}

	if p != nil {
		c.localApps = p.cfg.Applications
		c.localVendors = p.cfg.SupportedVendors
		c.state.Store(int32(stateWaitCEA))
	}

	t.setWriteTimeout(2 * n.cfg.WatchdogInterval)
	c.dprCause.Store(-1)
	c.hopByHop.Store(randomUint32())

	return c
}

func randomJitter(spread time.Duration) time.Duration {
	if spread <= 0 {
		return 0
	}

	var b [8]byte

	_, _ = rand.Read(b[:])

	return time.Duration(binary.BigEndian.Uint64(b[:])%uint64(2*spread+1)) - spread
}

func randomUint32() uint32 {
	var b [4]byte

	_, _ = rand.Read(b[:])

	return binary.BigEndian.Uint32(b[:])
}

func (c *Conn) PeerHost() string { return c.peerHost }

func (c *Conn) PeerRealm() string { return c.peerRealm }

func (c *Conn) PeerID() string {
	if c.peer == nil {
		return ""
	}

	return c.peer.id
}

func (c *Conn) RemoteAddr() netip.Addr { return c.t.remoteAddr() }

func (c *Conn) Transport() Transport { return c.t.kind() }

func (c *Conn) LocalIdentity() Identity { return c.n.Identity() }

func (c *Conn) Answer(req *Message, resultCode uint32) *Message {
	return NewAnswer(req, c.n.cfg.Identity, resultCode)
}

func (c *Conn) supports(appID uint32) bool {
	return c.common[appID]
}

func (c *Conn) commonApplications() []Application {
	var apps []Application

	for _, app := range c.localApps {
		if c.common[app.ID] {
			apps = append(apps, app)
		}
	}

	return apps
}

func (c *Conn) peerState() PeerState {
	if connState(c.state.Load()) == stateClosing {
		return PeerClosing
	}

	switch watchdogStatus(c.watchdog.Load()) {
	case watchdogSuspect:
		return PeerSuspect
	case watchdogReopen:
		return PeerReopen
	default:
		return PeerOpen
	}
}

func (c *Conn) logger() *slog.Logger {
	return c.n.logger.With(slog.String("peer", c.name()), slog.String("remote", c.t.remoteAddr().String()))
}

func (c *Conn) name() string {
	switch {
	case c.peer != nil && c.peer.cfg != nil:
		return c.peer.name()
	case c.peerHost != "":
		return c.peerHost
	default:
		return "unidentified"
	}
}

func (c *Conn) start() {
	c.startTimer(c.n.cfg.HandshakeTimeout, func() {
		switch connState(c.state.Load()) {
		case stateWaitCER, stateWaitCEA, stateParked:
			c.fail("capabilities exchange timed out")
		}
	})

	go c.readLoop()

	if c.initiator {
		cer := c.capabilitiesRequest()
		c.cerHbH = cer.HopByHopID
		c.send(cer)
	}
}

func (c *Conn) readLoop() {
	defer c.n.goroutines.Done()
	defer c.finish()

	buf := make([]byte, maxMessageSize)

	for {
		size, err := c.t.readMessage(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				c.fail(err.Error())
			}

			return
		}

		m, err := Unmarshal(slices.Clone(buf[:size]))
		if err != nil {
			c.receiveMalformed(m, err)
			continue
		}

		c.receive(m)
	}
}

func (c *Conn) finish() {
	c.stopTimer()
	c.available.Store(false)
	_ = c.t.close()

	if c.pendingSlot {
		c.pendingSlot = false
		c.n.releasePending()
	}

	c.doneOnce.Do(func() {
		c.cancel()
		close(c.done)

		c.mu.Lock()
		c.pending = nil
		c.mu.Unlock()
	})

	c.n.connDown(c)
}

func (c *Conn) setError(reason string) {
	c.lastError.Store(&reason)
}

func (c *Conn) errorText() string {
	if p := c.lastError.Load(); p != nil {
		return *p
	}

	return ""
}

func (c *Conn) fail(reason string) {
	c.setError(reason)
	c.logger().Warn("aborting Diameter connection", slog.String("reason", reason))
	_ = c.t.abort()
}

func (c *Conn) abort(reason string) {
	c.logger().Debug("aborting Diameter connection", slog.String("reason", reason))
	_ = c.t.abort()
}

func (c *Conn) receive(m *Message) {
	switch connState(c.state.Load()) {
	case stateWaitCER:
		if m.IsRequest() && m.CommandCode == CommandCapabilitiesExchange {
			c.handleCER(m)
			return
		}

		c.fail(fmt.Sprintf("first message is command %d, not a CER", m.CommandCode))

		return
	case stateWaitCEA:
		if !m.IsRequest() && m.CommandCode == CommandCapabilitiesExchange && m.HopByHopID == c.cerHbH {
			c.handleCEA(m)
			return
		}

		c.fail(fmt.Sprintf("expected a CEA, got command %d", m.CommandCode))

		return
	case stateParked:
		c.fail(fmt.Sprintf("command %d received before our CEA", m.CommandCode))

		return
	}

	if !c.initiator && c.unordered.CompareAndSwap(false, true) {
		c.t.setUnordered()
	}

	if m.CommandCode != CommandDeviceWatchdog || m.IsRequest() {
		signal(c.activity)
	}

	if !m.IsRequest() {
		c.receiveAnswer(m)
		return
	}

	if m.Flags&FlagError != 0 {
		c.send(c.Answer(m, ResultInvalidHdrBits))
		return
	}

	switch m.CommandCode {
	case CommandDeviceWatchdog:
		c.send(c.baseAnswer(m, ResultSuccess))
	case CommandDisconnectPeer:
		c.receiveDPR(m)
	case CommandCapabilitiesExchange:
		c.fail("CER received on an open connection")
	default:
		c.receiveRequest(m)
	}
}

func (c *Conn) receiveRequest(m *Message) {
	code := uint32(0)

	switch {
	case connState(c.state.Load()) == stateClosing:
		code = ResultUnableToDeliver
	case m.ApplicationID == 0:
		code = ResultCommandUnsupported
	case !c.common[m.ApplicationID]:
		code = ResultApplicationUnsupported
	default:
		code = c.n.routingError(m)
	}

	if code != 0 {
		c.send(c.Answer(m, code))
		return
	}

	select {
	case c.requests <- struct{}{}:
	default:
		c.send(c.Answer(m, ResultTooBusy))
		return
	}

	if code := c.n.admit(m); code != 0 {
		<-c.requests
		c.send(c.Answer(m, code))

		return
	}

	go c.serve(m)
}

func (c *Conn) receiveMalformed(m *Message, err error) {
	state := connState(c.state.Load())
	if m == nil || !m.IsRequest() || state == stateWaitCER || state == stateWaitCEA || state == stateParked {
		c.fail(fmt.Sprintf("unparsable message: %v", err))
		return
	}

	switch {
	case errors.Is(err, ErrUnsupportedVersion):
		c.send(c.Answer(m, ResultUnsupportedVersion))
	case errors.Is(err, ErrInvalidMessageLength):
		c.send(c.Answer(m, ResultInvalidMessageLength))
	case errors.Is(err, ErrInvalidAVPLength):
		ans := c.Answer(m, ResultInvalidAVPLength)
		ans.AVPs = append(ans.AVPs, FailedAVP())
		c.send(ans)
	default:
		c.send(c.Answer(m, ResultUnableToComply))
	}
}

func (c *Conn) receiveAnswer(m *Message) {
	switch m.CommandCode {
	case CommandDeviceWatchdog:
		signal(c.watchdogDWA)
		return
	case CommandDisconnectPeer:
		if connState(c.state.Load()) == stateClosing && m.HopByHopID == c.dprHbH.Load() {
			_ = c.t.close()
		}

		return
	case CommandCapabilitiesExchange:
		return
	}

	c.mu.Lock()
	ch, ok := c.pending[m.HopByHopID]
	c.mu.Unlock()

	if !ok {
		c.logger().Debug("discarding Diameter answer with an unknown Hop-by-Hop Identifier", slog.Uint64("hop_by_hop", uint64(m.HopByHopID)))
		return
	}

	select {
	case ch <- m:
	default:
	}
}

func (c *Conn) receiveDPR(m *Message) {
	if cause, ok := m.Find(AVPDisconnectCause, 0); ok {
		if v, err := cause.Unsigned32(); err == nil {
			c.dprCause.Store(int64(v))
		}
	}

	c.enterClosing()
	c.send(c.baseAnswer(m, ResultSuccess))
	c.startTimer(c.n.cfg.HandshakeTimeout, func() { c.abort("peer did not close after DPA") })
}

func (c *Conn) enterClosing() bool {
	for {
		state := connState(c.state.Load())
		if state != stateOpen {
			return false
		}

		if c.state.CompareAndSwap(int32(stateOpen), int32(stateClosing)) {
			c.available.Store(false)
			c.n.connStateChanged(c)

			return true
		}
	}
}

func (c *Conn) disconnect(cause uint32) {
	if !c.enterClosing() {
		c.abort("disconnect before the connection opened")
		<-c.done

		return
	}

	dpr := &Message{
		Flags:       FlagRequest,
		CommandCode: CommandDisconnectPeer,
		HopByHopID:  c.hopByHop.Add(1),
		EndToEndID:  c.n.nextEndToEnd(),
		AVPs: []AVP{
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, c.n.cfg.Identity.OriginHost),
			UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, c.n.cfg.Identity.OriginRealm),
			Unsigned32(AVPDisconnectCause, AVPFlagMandatory, 0, cause),
		},
	}

	c.dprHbH.Store(dpr.HopByHopID)
	c.send(dpr)

	select {
	case <-c.done:
	case <-time.After(c.n.cfg.HandshakeTimeout):
		c.abort("peer did not answer DPR")
		<-c.done
	}
}

func (c *Conn) handleCER(req *Message) {
	c.stopTimer()

	host, hasHost := req.Find(AVPOriginHost, 0)
	realm, hasRealm := req.Find(AVPOriginRealm, 0)

	switch {
	case !hasHost:
		c.rejectCER(req, ResultMissingAVP, UTF8String(AVPOriginHost, AVPFlagMandatory, 0, ""))
		return
	case !hasRealm:
		c.rejectCER(req, ResultMissingAVP, UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, ""))
		return
	case host.UTF8String() == "":
		c.rejectCER(req, ResultInvalidAVPValue, host)
		return
	case realm.UTF8String() == "":
		c.rejectCER(req, ResultInvalidAVPValue, realm)
		return
	case strings.EqualFold(host.UTF8String(), c.n.cfg.Identity.OriginHost):
		c.rejectCER(req, ResultUnknownPeer)
		return
	case !offersNoInbandSecurity(req.AVPs):
		c.rejectCER(req, ResultNoCommonSecurity)
		return
	}

	c.peerHost = host.UTF8String()
	c.peerRealm = realm.UTF8String()

	d := c.n.acceptCER(c, c.peerHost, req)

	for _, other := range d.displaced {
		other.abort("superseded by a new connection from the same peer")
	}

	switch d.outcome {
	case cerReject:
		c.logger().Info("rejecting Diameter peer", slog.Uint64("result_code", uint64(d.result)))
		c.rejectCER(req, d.result)
	case cerDisconnect:
		c.abort("peer already connected")
	case cerParked:
		c.parkedCER = req
		c.state.Store(int32(stateParked))
		c.startTimer(c.n.cfg.HandshakeTimeout, func() {
			if connState(c.state.Load()) == stateParked {
				c.fail("election outcome timed out")
			}
		})
	case cerOpen:
		c.open(c.capabilitiesAnswer(req, ResultSuccess))
	}
}

func (c *Conn) completeParked() {
	if !c.state.CompareAndSwap(int32(stateParked), int32(stateWaitCER)) {
		return
	}

	c.stopTimer()
	c.open(c.capabilitiesAnswer(c.parkedCER, ResultSuccess))
	c.parkedCER = nil
}

func (c *Conn) rejectCER(req *Message, result uint32, failed ...AVP) {
	cea := c.capabilitiesAnswer(req, result)
	if len(failed) > 0 {
		cea.AVPs = append(cea.AVPs, FailedAVP(failed...))
	}

	c.setError(fmt.Sprintf("CER rejected with %d", result))
	c.send(cea)
	_ = c.t.close()
}

func (c *Conn) handleCEA(ans *Message) {
	c.stopTimer()

	result, _ := ans.Find(AVPResultCode, 0)
	code, _ := result.Unsigned32()

	if code != ResultSuccess {
		c.fail(fmt.Sprintf("peer refused the capabilities exchange with %d", code))
		return
	}

	host, hasHost := ans.Find(AVPOriginHost, 0)
	realm, hasRealm := ans.Find(AVPOriginRealm, 0)

	if !hasHost || !hasRealm || host.UTF8String() == "" || realm.UTF8String() == "" {
		c.fail("CEA without Origin-Host or Origin-Realm")
		return
	}

	common := commonApplications(ans.AVPs, c.localApps)
	if len(common) == 0 {
		c.fail("no common application")
		return
	}

	c.peerHost = host.UTF8String()
	c.peerRealm = realm.UTF8String()

	displaced, ok := c.n.acceptCEA(c, c.peerHost, common)

	for _, other := range displaced {
		other.abort("superseded by our connection to the same peer")
	}

	if !ok {
		c.abort("connection superseded or from an unexpected peer")
		return
	}

	c.unordered.Store(true)
	c.t.setUnordered()
	c.open(nil)
}

func (c *Conn) open(cea *Message) {
	if c.pendingSlot {
		c.pendingSlot = false
		c.n.releasePending()
	}

	if c.reopen {
		c.watchdog.Store(int32(watchdogReopen))
	} else {
		c.watchdog.Store(int32(watchdogOkay))
	}

	c.state.Store(int32(stateOpen))

	if cea != nil {
		c.send(cea)
	}

	if !c.reopen {
		c.available.Store(true)
	}

	c.logger().Debug("Diameter peer connected",
		slog.String("host", c.peerHost), slog.String("realm", c.peerRealm), slog.String("transport", c.t.kind().String()))

	c.n.connStateChanged(c)
	c.openOnce.Do(func() { close(c.opened) })

	c.n.goroutines.Add(1)

	go c.runWatchdog()
}

func (c *Conn) serve(req *Message) {
	defer c.n.goroutines.Done()
	defer c.n.inflight.Done()
	defer func() { <-c.requests }()

	key, entry, original := c.n.duplicates.begin(c.peer.key(), req)
	if !original {
		select {
		case <-entry.done:
			if b := entry.answerFor(req.HopByHopID); b != nil {
				_ = c.writeRaw(b)
			}
		case <-c.done:
		}

		return
	}

	after := &afterAnswer{}

	b, err := c.handle(req, after).Marshal()
	if err != nil {
		c.logger().Error("failed to encode Diameter answer", slog.Any("error", err))

		b, _ = c.Answer(req, ResultUnableToComply).Marshal()
	}

	c.n.duplicates.finish(key, entry, b)
	after.run(c, c.writeRaw(b))
}

func (c *Conn) handle(req *Message, after *afterAnswer) (ans *Message) {
	defer func() {
		if r := recover(); r != nil {
			c.logger().Error("panic handling Diameter request",
				slog.Any("panic", r), slog.Uint64("command_code", uint64(req.CommandCode)), slog.String("stack", string(debug.Stack())))

			ans = c.Answer(req, ResultUnableToComply)
		}
	}()

	ctx := context.WithValue(c.ctx, afterAnswerKey{}, after)

	if c.n.cfg.HandlerTimeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, c.n.cfg.HandlerTimeout)
		defer cancel()
	}

	ans = c.n.cfg.Handler.ServeDiameter(ctx, c, req)
	if ans == nil {
		ans = c.Answer(req, ResultUnableToComply)
	}

	return ans
}

func (c *Conn) exchange(ctx context.Context, req *Message, failover bool) (*Message, error) {
	m := *req
	m.HopByHopID = c.hopByHop.Add(1)

	ch := make(chan *Message, 1)

	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return nil, errConnClosed
	}

	c.pending[m.HopByHopID] = ch

	var suspected <-chan struct{}
	if failover {
		suspected = c.suspected
	}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.pending != nil {
			delete(c.pending, m.HopByHopID)
		}
		c.mu.Unlock()
	}()

	if err := c.write(&m); err != nil {
		return nil, errConnClosed
	}

	select {
	case ans := <-ch:
		return ans, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, errConnClosed
	case <-suspected:
		return nil, errConnSuspect
	}
}

func (c *Conn) setSuspect(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case on && !c.suspect:
		close(c.suspected)
	case !on && c.suspect:
		c.suspected = make(chan struct{})
	}

	c.suspect = on
}

func (c *Conn) write(m *Message) error {
	b, err := m.Marshal()
	if err != nil {
		return err
	}

	return c.t.writeMessage(b)
}

func (c *Conn) writeRaw(b []byte) error {
	err := c.t.writeMessage(b)
	if err != nil {
		c.logger().Debug("failed to send Diameter message", slog.Any("error", err))
	}

	return err
}

func (c *Conn) send(m *Message) {
	if err := c.write(m); err != nil {
		c.logger().Debug("failed to send Diameter message", slog.Uint64("command_code", uint64(m.CommandCode)), slog.Any("error", err))
	}
}

func (c *Conn) baseAnswer(req *Message, resultCode uint32) *Message {
	ans := c.Answer(req, resultCode)
	ans.AVPs = append(ans.AVPs, Unsigned32(AVPOriginStateID, AVPFlagMandatory, 0, c.n.cfg.OriginStateID))

	return ans
}

func (c *Conn) capabilityAVPs() []AVP {
	id := c.n.cfg.Identity

	avps := []AVP{
		UTF8String(AVPOriginHost, AVPFlagMandatory, 0, id.OriginHost),
		UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, id.OriginRealm),
	}

	addrs := c.t.localAddrs()
	if len(addrs) == 0 {
		addrs = id.HostIPAddresses
	}

	for _, addr := range addrs {
		avps = append(avps, Address(AVPHostIPAddress, AVPFlagMandatory, 0, addr))
	}

	avps = append(avps,
		Unsigned32(AVPVendorID, AVPFlagMandatory, 0, id.VendorID),
		UTF8String(AVPProductName, 0, 0, id.ProductName),
		Unsigned32(AVPOriginStateID, AVPFlagMandatory, 0, c.n.cfg.OriginStateID),
	)

	seenVendors := make(map[uint32]bool)

	vendors := make([]uint32, 0, len(c.localApps)+len(c.localVendors))
	for _, app := range c.localApps {
		vendors = append(vendors, app.VendorID)
	}

	for _, vendor := range append(vendors, c.localVendors...) {
		if vendor != 0 && !seenVendors[vendor] {
			seenVendors[vendor] = true
			avps = append(avps, Unsigned32(AVPSupportedVendorID, AVPFlagMandatory, 0, vendor))
		}
	}

	for _, app := range c.localApps {
		if app.VendorID == 0 {
			avps = append(avps, Unsigned32(AVPAuthApplicationID, AVPFlagMandatory, 0, app.ID))
			continue
		}

		avps = append(avps, Grouped(AVPVendorSpecificApplicationID, AVPFlagMandatory, 0,
			Unsigned32(AVPVendorID, AVPFlagMandatory, 0, app.VendorID),
			Unsigned32(AVPAuthApplicationID, AVPFlagMandatory, 0, app.ID),
		))
	}

	return avps
}

func (c *Conn) capabilitiesAnswer(req *Message, resultCode uint32) *Message {
	flags := uint8(0)
	if resultCode >= 3000 && resultCode < 4000 {
		flags = FlagError
	}

	return &Message{
		Flags:       flags,
		CommandCode: CommandCapabilitiesExchange,
		HopByHopID:  req.HopByHopID,
		EndToEndID:  req.EndToEndID,
		AVPs:        append([]AVP{Unsigned32(AVPResultCode, AVPFlagMandatory, 0, resultCode)}, c.capabilityAVPs()...),
	}
}

func (c *Conn) capabilitiesRequest() *Message {
	return &Message{
		Flags:       FlagRequest,
		CommandCode: CommandCapabilitiesExchange,
		HopByHopID:  c.hopByHop.Add(1),
		EndToEndID:  c.n.nextEndToEnd(),
		AVPs:        c.capabilityAVPs(),
	}
}

func offersNoInbandSecurity(avps []AVP) bool {
	offers := FindAll(avps, AVPInbandSecurityID, 0)
	if len(offers) == 0 {
		return true
	}

	for _, a := range offers {
		if v, err := a.Unsigned32(); err == nil && v == InbandSecurityNone {
			return true
		}
	}

	return false
}

func (c *Conn) startTimer(d time.Duration, fn func()) {
	c.timerMu.Lock()
	defer c.timerMu.Unlock()

	if c.timer != nil {
		c.timer.Stop()
	}

	c.timer = time.AfterFunc(d, fn)
}

func (c *Conn) stopTimer() {
	c.timerMu.Lock()
	defer c.timerMu.Unlock()

	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (c *Conn) runWatchdog() {
	defer c.n.goroutines.Done()

	tw := c.n.cfg.WatchdogInterval
	pending := false
	numDWA := 0

	timer := time.NewTimer(c.jitter(tw))
	defer timer.Stop()

	setStatus := func(s watchdogStatus) {
		c.watchdog.Store(int32(s))
		c.setSuspect(s == watchdogSuspect)
		c.available.Store(s == watchdogOkay && connState(c.state.Load()) == stateOpen)
		c.n.connStateChanged(c)
	}

	sendDWR := func() {
		c.send(&Message{
			Flags:       FlagRequest,
			CommandCode: CommandDeviceWatchdog,
			HopByHopID:  c.hopByHop.Add(1),
			EndToEndID:  c.n.nextEndToEnd(),
			AVPs: []AVP{
				UTF8String(AVPOriginHost, AVPFlagMandatory, 0, c.n.cfg.Identity.OriginHost),
				UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, c.n.cfg.Identity.OriginRealm),
				Unsigned32(AVPOriginStateID, AVPFlagMandatory, 0, c.n.cfg.OriginStateID),
			},
		})

		pending = true
	}

	status := watchdogStatus(c.watchdog.Load())
	if status == watchdogReopen {
		sendDWR()
	}

	for {
		select {
		case <-c.done:
			return
		case <-c.watchdogDWA:
			pending = false

			switch status {
			case watchdogReopen:
				numDWA++
				if numDWA >= 3 {
					status = watchdogOkay
					setStatus(status)
				}
			case watchdogSuspect:
				status = watchdogOkay
				setStatus(status)
			}
		case <-c.activity:
			if status == watchdogSuspect {
				status = watchdogOkay
				setStatus(status)
			}

			if status == watchdogOkay {
				timer.Reset(c.jitter(tw))
			}
		case <-timer.C:
			switch status {
			case watchdogOkay:
				if pending {
					status = watchdogSuspect
					setStatus(status)
				} else {
					sendDWR()
				}
			case watchdogSuspect:
				c.fail("watchdog expired twice without an answer")
				return
			case watchdogReopen:
				switch {
				case !pending:
					sendDWR()
				case numDWA < 0:
					c.fail("reopened connection did not answer the watchdog")
					return
				default:
					numDWA = -1
				}
			}

			timer.Reset(c.jitter(tw))
		}
	}
}

func (c *Conn) jitter(tw time.Duration) time.Duration {
	spread := c.n.cfg.watchdogJitter
	if spread <= 0 {
		return tw
	}

	return tw + randomJitter(spread)
}
