// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func unrecoverable(seid uint64) error {
	return fmt.Errorf("modify session %d: %w", seid, models.ErrSessionUnrecoverable)
}

func (f *fakeAMF) releases() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.releaseCalls)
}

func (f *fakeUPF) deleted(seid uint64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Contains(f.deleteCalls, deletionCall{seid})
}

func TestUnrecoverableModifyReleasesThe5GSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	seid, an := smCtx.PFCPContext.SEID, smCtx.Tunnel.AN

	upf.modifyErrBySEID = map[uint64]error{seid: unrecoverable(seid)}

	err := s.HandleErrorIndicationReport(context.Background(), reportFor(seid, an))
	if !errors.Is(err, models.ErrSessionUnrecoverable) {
		t.Fatalf("HandleErrorIndicationReport = %v, want ErrSessionUnrecoverable", err)
	}

	waitFor(t, "the network-requested release", func() bool { return amfCb.releases() == 1 })
	waitFor(t, "the UPF session deletion", func() bool { return upf.deleted(seid) })

	time.Sleep(20 * time.Millisecond)

	if got := amfCb.releases(); got != 1 {
		t.Fatalf("release commands = %d, want 1", got)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("the SM context was removed before the UE acknowledged the release")
	}
}

func TestRecoverableModifyFailureKeepsTheSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	seid := smCtx.PFCPContext.SEID

	upf.modifyErrBySEID = map[uint64]error{seid: errors.New("pfcp modification failed")}

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(seid, smCtx.Tunnel.AN)); err == nil {
		t.Fatal("a failed modification was reported as success")
	}

	time.Sleep(20 * time.Millisecond)

	if amfCb.releases() != 0 || upf.deleted(seid) || s.GetSession(ref) == nil {
		t.Fatal("a recoverable modification failure released the session")
	}
}

func TestUnrecoverableModifyReleasesTheEPSSession(t *testing.T) {
	for _, unreachable := range []bool{false, true} {
		name := "reactivation"
		if unreachable {
			name = "unreachable UE"
		}

		t.Run(name, func(t *testing.T) {
			pcf, store, upf, amfCb := defaultFakes()
			s := newTestSMF(pcf, store, upf, amfCb)

			mme := &fakeMME{}
			if unreachable {
				mme.reactivateErr = smf.ErrUENotReachable
			}

			s.SetMME(mme)

			smCtx := establishEPSForArrival(t, s)
			smCtx.Tunnel.AN = smf.AnchorBinding{TEID: 7000, IPv4: net.ParseIP("10.0.0.200").To4()}
			smCtx.Tunnel.Downlink = smf.DownlinkForwarding

			seid, ebi, ref, imsi, an := smCtx.PFCPContext.SEID, smCtx.EBI, smCtx.Ref, smCtx.Supi.IMSI(), smCtx.Tunnel.AN

			upf.modifyErrBySEID = map[uint64]error{seid: unrecoverable(seid)}

			if err := s.HandleErrorIndicationReport(context.Background(), reportFor(seid, an)); !errors.Is(err, models.ErrSessionUnrecoverable) {
				t.Fatalf("HandleErrorIndicationReport = %v, want ErrSessionUnrecoverable", err)
			}

			waitFor(t, "the EPS bearer reactivation", func() bool {
				mme.mu.Lock()
				defer mme.mu.Unlock()

				return slices.Equal(mme.reactivated, []uint8{ebi}) &&
					slices.Equal(mme.reactivationReasons, []models.SessionReleaseReason{models.ReleaseNetworkFailure})
			})

			if !unreachable {
				return
			}

			waitFor(t, "the local teardown", func() bool {
				return upf.deleted(seid) && s.GetSession(ref) == nil &&
					slices.Contains(mme.dropped(), mmeTransferredCall{imsi, ebi, ref})
			})
		})
	}
}
