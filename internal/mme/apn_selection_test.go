// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"
)

// TS 23.401 §5.3.2.1
func TestSubscribedAPNDefaultWhenNoAPN(t *testing.T) {
	m := newTestMME(t)

	apn, err := SubscribedAPN(context.Background(), m, testSubscriber.IMSI, "")
	if err != nil {
		t.Fatalf("SubscribedAPN: %v", err)
	}

	if apn != "internet" {
		t.Errorf("APN = %q, want the default %q", apn, "internet")
	}
}

func TestSubscribedAPNRejectsUnknownAPN(t *testing.T) {
	m := newTestMME(t)

	if _, err := SubscribedAPN(context.Background(), m, testSubscriber.IMSI, "nonexistent"); !errors.Is(err, ErrUnknownAPN) {
		t.Fatalf("SubscribedAPN error = %v, want ErrUnknownAPN", err)
	}
}
