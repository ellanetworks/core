// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

type deregisterTestSmf struct {
	releaseCalls []string
	onRelease    func(context.Context, string) error
}

func (s *deregisterTestSmf) GetSession(string) *smf.SMContext { return nil }

func (s *deregisterTestSmf) SessionsByDNN(string) []*smf.SMContext { return nil }

func (s *deregisterTestSmf) SessionCount() int { return 0 }

func (s *deregisterTestSmf) CreateSmContext(context.Context, etsi.SUPI, uint8, string, *models.Snssai, []byte) (string, []byte, error) {
	return "", nil, nil
}

func (s *deregisterTestSmf) ActivateSmContext(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) DeactivateSmContext(context.Context, string) error { return nil }

func (s *deregisterTestSmf) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	s.releaseCalls = append(s.releaseCalls, smContextRef)
	if s.onRelease != nil {
		return s.onRelease(ctx, smContextRef)
	}

	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN1Msg(context.Context, string, []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResSetupRsp(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResSetupFail(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResRelRsp(context.Context, string) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextCauseDuplicatePDUSessionID(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverPreparing(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverPrepared(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextXnHandoverPathSwitchReq(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextHandoverFailed(context.Context, string, []byte) error {
	return nil
}

func TestDeregister_DoesNotHoldLockDuringSmfRelease(t *testing.T) {
	ue := NewAmfUe()
	ue.Log = zap.NewNop()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}

	fakeSmf := &deregisterTestSmf{}
	relockCount := 0
	fakeSmf.onRelease = func(_ context.Context, _ string) error {
		ue.Mutex.Lock()
		_ = ue.state
		ue.Mutex.Unlock()

		relockCount++

		return nil
	}
	ue.smf = fakeSmf

	done := make(chan struct{})

	go func() {
		ue.Deregister(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Deregister appears blocked; likely held ue.Mutex while calling SMF")
	}

	if ue.state != Deregistered {
		t.Fatalf("expected state %q, got %q", Deregistered, ue.state)
	}

	if len(ue.SmContextList) != 0 {
		t.Fatalf("expected SmContextList to be cleared, got %d entries", len(ue.SmContextList))
	}

	if len(fakeSmf.releaseCalls) != 2 {
		t.Fatalf("expected 2 SM context releases, got %d", len(fakeSmf.releaseCalls))
	}

	if relockCount != 2 {
		t.Fatalf("expected to re-lock UE mutex during each release call, got %d", relockCount)
	}
}
