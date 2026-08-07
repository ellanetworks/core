// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// movedFrom5GSToEPS establishes a PDU session, transfers it to EPS with request
// type "handover", and binds the eNB so the move commits. It returns the ref
// both accesses still hold.
func movedFrom5GSToEPS(t *testing.T, s *smf.SMF) string {
	t.Helper()

	ctx := context.Background()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil || rejectN1 != nil {
		t.Fatalf("CreateSmContext: %v (reject %d bytes)", err, len(rejectN1))
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	return ref
}

// Every 5GS entry point is reached with a live Ref naming a session EPS now
// serves. Acting on one would point the downlink back at a gNB the session left,
// or tear down a session the MME owns.
func TestFiveGSEntryPointsRefuseASessionMovedToEPS(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*smf.SMF, context.Context, string) error
		// The reconciler signals 5GSM, so a session on EPS is left to the MME's own
		// reconciler and is not an error.
		refusalIsAnError bool
	}{
		{"ActivateSmContext", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.ActivateSmContext(ctx, ref)
			return err
		}, true},
		{"UpdateSmContextN1Msg", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(transferTestPDUSessionID, 7))
			return err
		}, true},
		{"UpdateSmContextN2InfoPduResRelRsp", func(s *smf.SMF, ctx context.Context, ref string) error {
			return s.UpdateSmContextN2InfoPduResRelRsp(ctx, ref)
		}, true},
		{"UpdateSmContextCauseDuplicatePDUSessionID", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.UpdateSmContextCauseDuplicatePDUSessionID(ctx, ref)
			return err
		}, true},
		{"UpdateSmContextN2HandoverPreparing", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.UpdateSmContextN2HandoverPreparing(ctx, ref, nil)
			return err
		}, true},
		{"UpdateSmContextN2HandoverPrepared", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, nil)
			return err
		}, true},
		{"UpdateSmContextN2HandoverComplete", func(s *smf.SMF, ctx context.Context, ref string) error {
			return s.UpdateSmContextN2HandoverComplete(ctx, ref)
		}, true},
		{"UpdateSmContextXnHandoverPathSwitchReq", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.UpdateSmContextXnHandoverPathSwitchReq(ctx, ref, nil)
			return err
		}, true},
		{"UpdateSmContextN2ModifyIndication", func(s *smf.SMF, ctx context.Context, ref string) error {
			_, err := s.UpdateSmContextN2ModifyIndication(ctx, ref, nil)
			return err
		}, true},
		{"ReconcileSmContext", func(s *smf.SMF, ctx context.Context, ref string) error {
			return s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{SmContextRef: ref})
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb := defaultFakes()
			s := newTestSMF(pcf, store, upf, amfCb)
			ctx := context.Background()

			ref := movedFrom5GSToEPS(t, s)

			upf.mu.Lock()
			modifiesBefore := len(upf.modifyCalls)
			deletesBefore := len(upf.deleteCalls)
			upf.mu.Unlock()

			err := tc.call(s, ctx, ref)
			if tc.refusalIsAnError && err == nil {
				t.Error("entry point accepted a session on EPS, want it refused")
			}

			if s.GetSession(ref) == nil {
				t.Fatal("the EPS session was torn down by a 5GS entry point")
			}

			sc := s.GetSession(ref)

			unlock := sc.LockForTest()
			onEPS := sc.IsEPS()

			unlock()

			if !onEPS {
				t.Error("the session was moved off EPS by a 5GS entry point")
			}

			upf.mu.Lock()
			modifies := len(upf.modifyCalls) - modifiesBefore
			deletes := len(upf.deleteCalls) - deletesBefore
			upf.mu.Unlock()

			if modifies != 0 || deletes != 0 {
				t.Errorf("UPF modifications = %d and deletions = %d, want 0 and 0", modifies, deletes)
			}
		})
	}
}
