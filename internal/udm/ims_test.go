// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/udm"
)

type fakeIMSStore struct {
	err error
}

func (f *fakeIMSStore) AdvanceIMSSequenceNumber(context.Context, string, string, string) (*udm.AdvancedCredentials, error) {
	return nil, f.err
}

func TestGenerateIMSVectorPropagatesUnknownSubscriber(t *testing.T) {
	store := &fakeIMSStore{err: udm.ErrSubscriberUnknown}

	_, err := udm.NewIMSCredentials(store).GenerateIMSVector(context.Background(), "001010000000099", nil)
	if !errors.Is(err, udm.ErrSubscriberUnknown) {
		t.Fatalf("err = %v, want ErrSubscriberUnknown", err)
	}
}
