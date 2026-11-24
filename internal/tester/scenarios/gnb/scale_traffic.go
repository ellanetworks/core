// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"go.uber.org/zap"
)

const (
	scalePingGrace     = 3 * time.Second
	scaleSilentUEsLogs = 5
)

type scalePinger struct {
	ue       *scaleUE
	id       uint16
	src, upf netip.Addr
	sent     atomic.Uint32
	received atomic.Uint32
}

type lossTimeline struct {
	sent     []atomic.Int64
	received []atomic.Int64
}

func newLossTimeline(d time.Duration) *lossTimeline {
	n := int(d/time.Second) + 2

	return &lossTimeline{sent: make([]atomic.Int64, n), received: make([]atomic.Int64, n)}
}

func (l *lossTimeline) bucket(offset time.Duration) int {
	return min(max(int(offset/time.Second), 0), len(l.sent)-1)
}

type rttRecorder struct {
	mu   sync.Mutex
	rtts []time.Duration
}

func (r *rttRecorder) add(d time.Duration) {
	r.mu.Lock()
	r.rtts = append(r.rtts, d)
	r.mu.Unlock()
}

func randUint64() uint64 {
	var b [8]byte

	_, _ = crand.Read(b[:])

	return binary.LittleEndian.Uint64(b[:])
}

func randDuration(d time.Duration) time.Duration {
	return time.Duration(randUint64() % uint64(d)) //nolint:gosec
}

func pingJitter(interval time.Duration) time.Duration {
	return interval/2 + randDuration(interval)
}

func runScalePingTraffic(ctx context.Context, ues []*scaleUE, p *scaleParams) error {
	dst, err := netip.ParseAddr(p.PingAddress)
	if err != nil {
		return fmt.Errorf("--ping-address: %w", err)
	}

	pingers := make([]*scalePinger, len(ues))
	byGNB := make(map[*gnb.GnodeB]map[uint32]*scalePinger)

	for i, s := range ues {
		src, err := netip.ParseAddr(s.session.UEIPv4)
		if err != nil {
			return fmt.Errorf("UE %s has no IPv4 address: %w", s.ue.UeSecurity.Supi, err)
		}

		upf, err := netip.ParseAddr(s.session.UpfAddress)
		if err != nil {
			return fmt.Errorf("UE %s has no UPF address: %w", s.ue.UeSecurity.Supi, err)
		}

		pingers[i] = &scalePinger{ue: s, id: uint16(randUint64()), src: src, upf: upf} //nolint:gosec

		if byGNB[s.gnb] == nil {
			byGNB[s.gnb] = make(map[uint32]*scalePinger)
		}

		byGNB[s.gnb][s.session.DLTEID] = pingers[i]
		s.gnb.WatchTEID(s.session.DLTEID)
	}

	start := time.Now()
	rtts := &rttRecorder{}
	timeline := newLossTimeline(p.Hold)

	for g, byTEID := range byGNB {
		g.SetDownlinkHook(func(teid uint32, packet []byte) {
			pg := byTEID[teid]
			if pg == nil {
				return
			}

			_, _, payload, ok := scenarios.ParseICMPEchoReply(packet)
			if !ok || len(payload) < 8 {
				return
			}

			sentAt := time.Duration(binary.BigEndian.Uint64(payload)) //nolint:gosec

			pg.received.Add(1)
			timeline.received[timeline.bucket(sentAt)].Add(1)
			rtts.add(time.Since(start) - sentAt)
		})
	}

	defer func() {
		for g := range byGNB {
			g.SetDownlinkHook(nil)
		}
	}()

	logger.Logger.Info("sending user plane traffic", zap.Int("ues", len(ues)), zap.Duration("interval", p.PingInterval), zap.Duration("duration", p.Hold))

	deadline := start.Add(p.Hold)

	var (
		wg         sync.WaitGroup
		sendErrors atomic.Int64
	)

	for _, pg := range pingers {
		wg.Add(1)

		var fire func()

		fire = func() {
			if ctx.Err() != nil || time.Now().After(deadline) {
				wg.Done()

				return
			}

			seq := uint16(pg.sent.Add(1)) //nolint:gosec
			sentAt := time.Since(start)
			payload := binary.BigEndian.AppendUint64(nil, uint64(sentAt)) //nolint:gosec
			timeline.sent[timeline.bucket(sentAt)].Add(1)

			packet := scenarios.ICMPEchoRequest(pg.src, dst, pg.id, seq, payload)

			if err := pg.ue.gnb.SendGPDUWithQFI(pg.ue.session.ULTEID, pg.upf, pg.ue.session.QFI, packet); err != nil {
				sendErrors.Add(1)
			}

			time.AfterFunc(pingJitter(p.PingInterval), fire)
		}

		time.AfterFunc(randDuration(p.PingInterval), fire)
	}

	wg.Wait()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(scalePingGrace):
	}

	logScaleLossTimeline(timeline)

	return summarizeScalePingTraffic(pingers, rtts, sendErrors.Load())
}

func logScaleLossTimeline(l *lossTimeline) {
	var seconds []string

	for i := range l.sent {
		sent, received := l.sent[i].Load(), l.received[i].Load()
		if sent == received {
			continue
		}

		seconds = append(seconds, fmt.Sprintf("%ds:%d/%d", i, sent-received, sent))
	}

	logger.Logger.Info("user plane loss timeline", zap.Int("seconds", len(l.sent)), zap.Int("lossy_seconds", len(seconds)), zap.Strings("lost_per_second", seconds))
}

func summarizeScalePingTraffic(pingers []*scalePinger, rtts *rttRecorder, sendErrors int64) error {
	var (
		sent, received        int64
		lossy, excess, dupIPs int
		silent                []*scalePinger
	)

	ips := make(map[netip.Addr]int, len(pingers))

	for _, pg := range pingers {
		s, r := pg.sent.Load(), pg.received.Load()
		sent += int64(s)
		received += int64(r)

		switch {
		case s > 0 && r == 0:
			silent = append(silent, pg)
		case r < s:
			lossy++
		case r > s:
			excess++
		}

		ips[pg.src]++
		if ips[pg.src] == 2 {
			dupIPs++
		}
	}

	loss := 0.0
	if sent > 0 {
		loss = 100 * float64(sent-received) / float64(sent)
	}

	fields := []zap.Field{
		zap.Int("ues", len(pingers)), zap.Int64("sent", sent), zap.Int64("received", received),
		zap.Float64("loss_percent", loss), zap.Int("silent_ues", len(silent)), zap.Int("lossy_ues", lossy),
		zap.Int("excess_ues", excess), zap.Int("duplicate_ue_ips", dupIPs), zap.Int64("send_errors", sendErrors),
	}

	rtts.mu.Lock()
	if len(rtts.rtts) > 0 {
		fields = append(fields, scenarios.LatencyFields(rtts.rtts)...)
	}
	rtts.mu.Unlock()

	logger.Logger.Info("user plane traffic complete", fields...)

	for _, pg := range silent[:min(len(silent), scaleSilentUEsLogs)] {
		logger.Logger.Warn("UE got no echo reply",
			zap.String("supi", pg.ue.ue.UeSecurity.Supi), zap.Stringer("ue_ip", pg.src), zap.Stringer("upf", pg.upf),
			zap.Uint32("ul_teid", pg.ue.session.ULTEID), zap.Uint32("dl_teid", pg.ue.session.DLTEID),
			zap.Int64("ran_ue_id", pg.ue.ranID), zap.String("gnb", pg.ue.gnb.Name), zap.Uint32("sent", pg.sent.Load()))
	}

	if len(silent) > 0 {
		return fmt.Errorf("%d UEs got no echo reply", len(silent))
	}

	if sendErrors > 0 {
		return fmt.Errorf("%d echo requests could not be sent", sendErrors)
	}

	return nil
}
