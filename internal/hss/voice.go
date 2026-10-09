// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"
)

func (h *HSS) VoiceSupported(ctx context.Context, imsi string) (bool, error) {
	configured, err := h.store.PCSCFConfigured(ctx)
	if err != nil || !configured {
		return false, err
	}

	sub, err := h.store.Subscriber(ctx, imsi)
	if errors.Is(err, ErrSubscriberUnknown) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	s := subject{Subscriber: *sub}

	return s.entitled(), nil
}
